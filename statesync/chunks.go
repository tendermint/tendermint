package statesync

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"sync"
)

// bufferChunks is the number of queued chunks to buffer in memory, before spooling to disk.
const bufferChunks = 4

// Done is returned by chunkQueue.Next when all chunks have been returned.
var Done = errors.New("iterator is done")

// chunk contains data for a chunk.
type chunk struct {
	Index uint32
	Body  []byte
}

// Hash generates a hash for the chunk body, used for verification.
func (c *chunk) Hash() []byte {
	if c.Body == nil {
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
	hashes     [][]byte // if this is nil, the queue has been closed
	dir        string
	nextChunk  uint32
	memChunks  map[uint32]*chunk
	diskChunks map[uint32]string
	chReady    chan uint32
	waiters    map[uint32][]chan<- bool
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
		hashes:     snapshot.ChunkHashes,
		dir:        dir,
		memChunks:  make(map[uint32]*chunk),
		diskChunks: make(map[uint32]string),
		chReady:    make(chan uint32, len(snapshot.ChunkHashes)),
		waiters:    make(map[uint32][]chan<- bool),
	}, nil
}

// Add adds a chunk to the queue, verifying its hash. It ignores chunks that already exist,
// returning false.
func (p *chunkQueue) Add(chunk *chunk) (bool, error) {
	if chunk.Body == nil {
		return false, errors.New("cannot add chunk with nil body")
	}
	p.Lock()
	defer p.Unlock()
	if p.hashes == nil {
		return false, errors.New("chunk queue is closed")
	}
	if chunk.Index >= uint32(len(p.hashes)) {
		return false, fmt.Errorf("received unexpected chunk %v", chunk.Index)
	}
	if p.memChunks[chunk.Index] != nil || p.diskChunks[chunk.Index] != "" {
		return false, nil
	}
	if !bytes.Equal(chunk.Hash(), p.hashes[chunk.Index]) {
		return false, fmt.Errorf("chunk %v hash mismatch, expected %x got %x", chunk.Index,
			p.hashes[chunk.Index], chunk.Hash())
	}

	// Save the chunk in memory if it is an upcoming one, otherwise spool to disk.
	if chunk.Index < p.nextChunk+bufferChunks {
		p.memChunks[chunk.Index] = chunk
	} else {
		path := path.Join(p.dir, strconv.FormatUint(uint64(chunk.Index), 10))
		err := ioutil.WriteFile(path, chunk.Body, 0644)
		if err != nil {
			return false, fmt.Errorf("failed to save chunk %v to file %v: %w", chunk.Index, path, err)
		}
		p.diskChunks[chunk.Index] = path
	}

	// If this is the next expected chunk, pass any ready chunks to Next() via channel, and close
	// the channel when all chunks have been passed.
	//
	// We don't pass chunk contents, since they may be spooled to disk, and loading all of them
	// here first would block other callers and use a ton of memory.
	if chunk.Index == p.nextChunk {
		for i := chunk.Index; p.memChunks[i] != nil || p.diskChunks[i] != ""; i++ {
			p.chReady <- i
			p.nextChunk++
		}
		if p.nextChunk >= uint32(len(p.hashes)) {
			close(p.chReady)
		}
	}

	// Signal any waiters that the chunk has arrived.
	for _, waiter := range p.waiters[chunk.Index] {
		waiter <- true
	}
	delete(p.waiters, chunk.Index)

	return true, nil
}

// Close closes the chunk queue, cleaning up all temporary files.
func (p *chunkQueue) Close() error {
	p.Lock()
	defer p.Unlock()
	if p.hashes == nil {
		return nil
	}
	if p.nextChunk < uint32(len(p.hashes)) {
		close(p.chReady)
	}
	for _, waiters := range p.waiters {
		for _, waiter := range waiters {
			waiter <- false
		}
	}
	p.waiters = nil
	p.hashes = nil
	err := os.RemoveAll(p.dir)
	if err != nil {
		return fmt.Errorf("failed to clean up state sync tempdir %v: %w", p.dir, err)
	}
	return nil
}

// Next returns the next chunk from the queue, or a Done error if no more chunks are available.
// It blocks until the chunk is available.
func (p *chunkQueue) Next() (*chunk, error) {
	index, ok := <-p.chReady
	if !ok {
		return nil, Done
	}
	p.Lock()
	defer p.Unlock()

	if chunk, ok := p.memChunks[index]; ok {
		delete(p.memChunks, index)
		return chunk, nil
	}
	if path, ok := p.diskChunks[index]; ok {
		body, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load chunk %v from file %v: %w", index, path, err)
		}
		err = os.Remove(path)
		if err != nil {
			return nil, fmt.Errorf("failed to remove chunk %v: %w", index, err)
		}
		delete(p.diskChunks, index)
		return &chunk{
			Index: index,
			Body:  body,
		}, nil
	}
	return nil, fmt.Errorf("chunk %v not found", index)
}

// WaitFor returns a channel that receives true when the chunk arrives in the pool, or false when
// the pool is closed.
func (p *chunkQueue) WaitFor(index uint32) <-chan bool {
	p.Lock()
	defer p.Unlock()
	ch := make(chan bool, 1)
	switch {
	case p.hashes == nil:
		ch <- false
	case p.nextChunk > index:
		ch <- true
	default:
		if p.waiters[index] == nil {
			p.waiters[index] = make([]chan<- bool, 0)
		}
		p.waiters[index] = append(p.waiters[index], ch)
	}
	return ch
}
