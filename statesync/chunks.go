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

const (
	// bufferChunks is the number of queued chunks to buffer in memory, before spooling to disk.
	bufferChunks = 4
)

var (
	// Done is returned by chunkQueue.Next when all chunks have been returned.
	Done = errors.New("iterator is done")
)

// chunk contains data about a chunk.
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
// iterator over an ordered sequence of chunks, but callers can add chunks out-of-order and it will
// be spooled in memory or on disk as appropriate.
type chunkQueue struct {
	sync.Mutex
	snapshot   *snapshot
	dir        string
	nextChunk  uint32
	memChunks  map[uint32]*chunk
	diskChunks map[uint32]string
	ch         chan uint32
}

// newChunkQueue creates a new chunk queue for a snapshot, using the OS temp dir for storage.
// Callers must call Close() when done.
func newChunkQueue(snapshot *snapshot) (*chunkQueue, error) {
	dir, err := ioutil.TempDir("", "tm-statesync")
	if err != nil {
		return nil, fmt.Errorf("unable to create temp dir for snapshot chunks: %w", err)
	}
	return &chunkQueue{
		snapshot:   snapshot,
		dir:        dir,
		memChunks:  make(map[uint32]*chunk),
		diskChunks: make(map[uint32]string),
		ch:         make(chan uint32, len(snapshot.ChunkHashes)),
	}, nil
}

// Add adds a chunk to the queue, verifying its hash. It ignores chunks that already exists,
// returning false.
func (p *chunkQueue) Add(chunk *chunk) (bool, error) {
	if chunk.Body == nil {
		return false, errors.New("cannot add chunk with nil body")
	}
	p.Lock()
	defer p.Unlock()
	if p.ch == nil {
		return false, errors.New("chunk queue is closed")
	}
	if chunk.Index >= uint32(len(p.snapshot.ChunkHashes)) {
		return false, fmt.Errorf("received unexpected snapshot chunk %v", chunk.Index)
	}
	if p.memChunks[chunk.Index] != nil || p.diskChunks[chunk.Index] != "" {
		return false, nil
	}
	if !bytes.Equal(chunk.Hash(), p.snapshot.ChunkHashes[chunk.Index]) {
		return false, fmt.Errorf("chunk %v hash mismatch, expected %x got %x", chunk.Index,
			p.snapshot.ChunkHashes[chunk.Index], chunk.Hash())
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

	// If this is the next chunk we've been waiting for, pass the next sequence of available chunks
	// through the channel to signal readiness. We can't pass the contents, since they may be
	// spooled to disk, and we don't want to block here while loading them.
	if chunk.Index == p.nextChunk {
		for i := chunk.Index; p.memChunks[i] != nil || p.diskChunks[i] != ""; i++ {
			p.ch <- i
			p.nextChunk++
		}
		if p.nextChunk >= uint32(len(p.snapshot.ChunkHashes)) {
			close(p.ch)
		}
	}
	return true, nil
}

// Close closes the chunk queue, cleaning up all temporary files.
func (p *chunkQueue) Close() error {
	p.Lock()
	defer p.Unlock()
	if p.snapshot == nil {
		return nil
	}
	err := os.RemoveAll(p.dir)
	if err != nil {
		return fmt.Errorf("failed to clean up state sync tempdir %v: %w", p.dir, err)
	}
	if p.nextChunk < uint32(len(p.snapshot.ChunkHashes)) {
		close(p.ch)
	}
	p.snapshot = nil
	return nil
}

// Has returns true if the chunk queue contains (or contained) a given chunk.
func (p *chunkQueue) Has(index uint32) bool {
	p.Lock()
	defer p.Unlock()
	return p.nextChunk > index || p.memChunks[index] != nil || p.diskChunks[index] != ""
}

// Next returns the next chunk from the queue, or a Done error if no more chunks are available.
// It blocks until the chunk is available.
func (p *chunkQueue) Next() (*chunk, error) {
	index, ok := <-p.ch
	if !ok {
		return nil, Done
	}
	p.Lock()
	defer p.Unlock()

	if c, ok := p.memChunks[index]; ok {
		delete(p.memChunks, index)
		return c, nil

	} else if path, ok := p.diskChunks[index]; ok {
		body, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load chunk %v from file %v: %w", index, path, err)
		}
		err = os.Remove(path)
		if err != nil {
			return nil, err
		}
		delete(p.diskChunks, index)
		return &chunk{
			Index: index,
			Body:  body,
		}, nil
	}
	return nil, fmt.Errorf("chunk %v not found", index)
}
