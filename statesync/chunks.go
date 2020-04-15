package statesync

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// bufferChunks is the number of queued chunks to buffer in memory, before spooling to disk.
const bufferChunks = 4

// errDone is returned by chunkQueue.Next when all chunks have been returned.
var errDone = errors.New("chunk queue is done")

// chunk contains data for a chunk.
type chunk struct {
	Height uint64
	Format uint32
	Index  uint32
	Body   []byte
}

// Hash generates a hash for the chunk body, used for verification.
func (c *chunk) Hash() []byte {
	if c == nil || c.Body == nil {
		return []byte{}
	}
	hash := sha256.Sum256(c.Body)
	return hash[:]
}

// chunkQueue manages chunks for a state sync process, ordering them as necessary. It acts as an
// iterator over an ordered sequence of chunks. Callers can add chunks out-of-order and they will
// be spooled in memory or on disk as appropriate.
type chunkQueue struct {
	sync.Mutex
	snapshot   *snapshot                // if this is nil, the queue has been closed
	dir        string                   // temp dir for on-disk chunk storage
	nextChunk  uint32                   // next chunk that Next() is waiting for
	memChunks  map[uint32]*chunk        // in-memory chunk storage (see bufferChunks)
	diskChunks map[uint32]string        // on-disk chunk storage (path to temp file)
	chReady    chan uint32              // signals Next() about next blocks being ready
	waiters    map[uint32][]chan<- bool // signals WaitFor() waiters about block arrival
}

// newChunkQueue creates a new chunk queue for a snapshot, using the OS temp dir for storage.
// Callers must call Close() when done.
func newChunkQueue(snapshot *snapshot) (*chunkQueue, error) {
	dir, err := ioutil.TempDir("", "tm-statesync")
	if err != nil {
		return nil, fmt.Errorf("unable to create temp dir for snapshot chunks: %w", err)
	}
	if len(snapshot.ChunkHashes) == 0 {
		return nil, errors.New("snapshot has no chunks")
	}
	return &chunkQueue{
		snapshot:   snapshot,
		dir:        dir,
		memChunks:  make(map[uint32]*chunk),
		diskChunks: make(map[uint32]string),
		chReady:    make(chan uint32, len(snapshot.ChunkHashes)),
		waiters:    make(map[uint32][]chan<- bool),
	}, nil
}

// Add adds a chunk to the queue, verifying its hash. It ignores chunks that already exist,
// returning false.
func (q *chunkQueue) Add(chunk *chunk) (bool, error) {
	if chunk == nil {
		return false, errors.New("cannot add nil chunk")
	}
	if chunk.Body == nil {
		return false, errors.New("cannot add chunk with nil body")
	}
	q.Lock()
	defer q.Unlock()
	if q.snapshot == nil {
		return false, errors.New("chunk queue is closed")
	}
	if chunk.Height != q.snapshot.Height {
		return false, fmt.Errorf("invalid chunk height %v, expected %v", chunk.Height, q.snapshot.Height)
	}
	if chunk.Format != q.snapshot.Format {
		return false, fmt.Errorf("invalid chunk format %v, expected %v", chunk.Format, q.snapshot.Format)
	}
	if chunk.Index >= uint32(len(q.snapshot.ChunkHashes)) {
		return false, fmt.Errorf("received unexpected chunk %v", chunk.Index)
	}
	if chunk.Index < q.nextChunk || q.memChunks[chunk.Index] != nil || q.diskChunks[chunk.Index] != "" {
		return false, nil
	}
	if !bytes.Equal(chunk.Hash(), q.snapshot.ChunkHashes[chunk.Index]) {
		return false, fmt.Errorf("chunk %v hash mismatch, expected %x got %x", chunk.Index,
			q.snapshot.ChunkHashes[chunk.Index], chunk.Hash())
	}

	// Save the chunk in memory if it is an upcoming one, otherwise spool to disk.
	if len(q.memChunks) < bufferChunks && chunk.Index < q.nextChunk+bufferChunks {
		q.memChunks[chunk.Index] = chunk
	} else {
		path := filepath.Join(q.dir, strconv.FormatUint(uint64(chunk.Index), 10))
		err := ioutil.WriteFile(path, chunk.Body, 0644)
		if err != nil {
			return false, fmt.Errorf("failed to save chunk %v to file %v: %w", chunk.Index, path, err)
		}
		q.diskChunks[chunk.Index] = path
	}

	// If this is the next expected chunk, pass any ready chunks to Next() via channel, and close
	// the channel when all chunks have been passed.
	//
	// We don't pass chunk contents, since they may be spooled to disk, and loading all of them
	// here first would block other callers and use a ton of memory.
	if chunk.Index == q.nextChunk {
		for i := chunk.Index; q.memChunks[i] != nil || q.diskChunks[i] != ""; i++ {
			q.chReady <- i
			q.nextChunk++
		}
		if q.nextChunk >= uint32(len(q.snapshot.ChunkHashes)) {
			close(q.chReady)
		}
	}

	// Signal any waiters that the chunk has arrived.
	for _, waiter := range q.waiters[chunk.Index] {
		waiter <- true
		close(waiter)
	}
	delete(q.waiters, chunk.Index)

	return true, nil
}

// Close closes the chunk queue, cleaning up all temporary files.
func (q *chunkQueue) Close() error {
	q.Lock()
	defer q.Unlock()
	if q.snapshot == nil {
		return nil
	}
	if q.nextChunk < uint32(len(q.snapshot.ChunkHashes)) {
		close(q.chReady)
	}
	for _, waiters := range q.waiters {
		for _, waiter := range waiters {
			waiter <- false
			close(waiter)
		}
	}
	q.waiters = nil
	q.snapshot = nil
	err := os.RemoveAll(q.dir)
	if err != nil {
		return fmt.Errorf("failed to clean up state sync tempdir %v: %w", q.dir, err)
	}
	return nil
}

// Next returns the next chunk from the queue, or a Done error if no more chunks are available.
// It blocks until the chunk is available.
func (q *chunkQueue) Next() (*chunk, error) {
	index, ok := <-q.chReady
	if !ok {
		return nil, errDone
	}
	q.Lock()
	defer q.Unlock()

	if chunk, ok := q.memChunks[index]; ok {
		delete(q.memChunks, index)
		return chunk, nil
	}
	if path, ok := q.diskChunks[index]; ok {
		body, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load chunk %v from file %v: %w", index, path, err)
		}
		err = os.Remove(path)
		if err != nil {
			return nil, fmt.Errorf("failed to remove chunk %v: %w", index, err)
		}
		delete(q.diskChunks, index)
		return &chunk{
			Height: q.snapshot.Height,
			Format: q.snapshot.Format,
			Index:  index,
			Body:   body,
		}, nil
	}
	return nil, fmt.Errorf("chunk %v not found", index)
}

// WaitFor returns a channel that receives true when the chunk arrives in the pool, or false when
// the pool is closed.
func (q *chunkQueue) WaitFor(index uint32) <-chan bool {
	q.Lock()
	defer q.Unlock()
	ch := make(chan bool, 1)
	switch {
	case q.snapshot == nil:
		ch <- false
		close(ch)
	case q.nextChunk > index:
		ch <- true
		close(ch)
	default:
		if q.waiters[index] == nil {
			q.waiters[index] = make([]chan<- bool, 0)
		}
		q.waiters[index] = append(q.waiters[index], ch)
	}
	return ch
}
