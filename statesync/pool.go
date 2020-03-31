package statesync

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"sync"

	"github.com/tendermint/tendermint/p2p"
)

const (
	// recentSnapshots is the number of recent snapshots to send and receive
	recentSnapshots = 10

	// number of chunks to buffer in memory, before spooling to disk
	bufferChunks = 4
)

var (
	Done = errors.New("iterator is done")
)

// snapshotPool discovers and aggregates snapshots across peers.
type snapshotPool struct {
	sync.Mutex
	snapshots     map[snapshotHash]*snapshot
	snapshotPeers map[snapshotHash]map[p2p.ID]p2p.Peer

	// indexes for fast searches
	formatIndex map[uint32]map[snapshotHash]bool
	heightIndex map[uint64]map[snapshotHash]bool
	peerIndex   map[p2p.ID]map[snapshotHash]bool

	// blacklists for rejected items
	formatBlacklist   map[uint32]bool
	heightBlacklist   map[uint64]bool
	snapshotBlacklist map[snapshotHash]bool
}

// newSnapshotPool creates a new snapshot pool.
func newSnapshotPool() *snapshotPool {
	return &snapshotPool{
		snapshots:         make(map[snapshotHash]*snapshot),
		snapshotPeers:     make(map[snapshotHash]map[p2p.ID]p2p.Peer),
		formatIndex:       make(map[uint32]map[snapshotHash]bool),
		heightIndex:       make(map[uint64]map[snapshotHash]bool),
		peerIndex:         make(map[p2p.ID]map[snapshotHash]bool),
		formatBlacklist:   make(map[uint32]bool),
		heightBlacklist:   make(map[uint64]bool),
		snapshotBlacklist: make(map[snapshotHash]bool),
	}
}

// Add adds a snapshot to the pool, unless the peer has already sent recentSnapshots snapshots. It
// returns true if this was a new, non-blacklisted snapshot.
func (p *snapshotPool) Add(peer p2p.Peer, snapshot *snapshot) bool {
	hash := snapshot.Hash()
	p.Lock()
	defer p.Unlock()

	switch {
	case p.formatBlacklist[snapshot.Format]:
		return false
	case p.heightBlacklist[snapshot.Height]:
		return false
	case p.snapshotBlacklist[hash]:
		return false
	case p.snapshots[hash] != nil:
		return false
	case len(p.peerIndex[peer.ID()]) >= recentSnapshots:
		return false
	}

	p.snapshots[hash] = snapshot

	if p.snapshotPeers[hash] == nil {
		p.snapshotPeers[hash] = make(map[p2p.ID]p2p.Peer)
	}
	p.snapshotPeers[hash][peer.ID()] = peer

	if p.peerIndex[peer.ID()] == nil {
		p.peerIndex[peer.ID()] = make(map[snapshotHash]bool)
	}
	p.peerIndex[peer.ID()][hash] = true

	if p.formatIndex[snapshot.Format] == nil {
		p.formatIndex[snapshot.Format] = make(map[snapshotHash]bool)
	}
	p.formatIndex[snapshot.Format][hash] = true

	if p.heightIndex[snapshot.Height] == nil {
		p.heightIndex[snapshot.Height] = make(map[snapshotHash]bool)
	}
	p.heightIndex[snapshot.Height][hash] = true

	return true
}

// Best returns the "best" currently known snapshot, if any. The current heuristic is very naïve,
// preferring the snapshot with the greatest height, then greatest format, then greatest number
// of peers. This can be improved quite a lot.
func (p *snapshotPool) Best() *snapshot {
	p.Lock()
	defer p.Unlock()

	candidates := make([]*snapshot, 0, len(p.snapshots))
	for _, snapshot := range p.snapshots {
		candidates = append(candidates, snapshot)
	}
	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		a := candidates[i]
		b := candidates[j]

		switch {
		case a.Height > b.Height:
			return true
		case a.Height < b.Height:
			return false
		case a.Format > b.Format:
			return true
		case a.Format < b.Format:
			return false
		case len(p.snapshotPeers[a.Hash()]) > len(p.snapshotPeers[b.Hash()]):
			return true
		default:
			return false
		}
	})

	return candidates[0]
}

// Reject rejects a snapshot. Rejected snapshots will never be used again.
func (p *snapshotPool) Reject(snapshot *snapshot) {
	hash := snapshot.Hash()
	p.Lock()
	defer p.Unlock()

	p.snapshotBlacklist[hash] = true
	p.removeSnapshot(hash)
}

// RejectFormat rejects a snapshot format. It will never be used again.
func (p *snapshotPool) RejectFormat(format uint32) {
	p.Lock()
	defer p.Unlock()

	p.formatBlacklist[format] = true
	for hash := range p.formatIndex[format] {
		p.removeSnapshot(hash)
	}
}

// RejectHeight rejects a snapshot height. It will never be used again.
func (p *snapshotPool) RejectHeight(height uint64) {
	p.Lock()
	defer p.Unlock()

	p.heightBlacklist[height] = true
	for hash := range p.heightIndex[height] {
		p.removeSnapshot(hash)
	}
}

// RemovePeer removes a peer from the pool, and any snapshots that no longer have peers.
func (p *snapshotPool) RemovePeer(peer p2p.Peer) {
	p.Lock()
	defer p.Unlock()

	for hash := range p.peerIndex[peer.ID()] {
		delete(p.snapshotPeers[hash], peer.ID())
		if len(p.snapshotPeers[hash]) == 0 {
			p.removeSnapshot(hash)
		}
	}
	delete(p.peerIndex, peer.ID())
}

// removeSnapshot removes a snapshot. The caller must hold the mutex lock.
func (p *snapshotPool) removeSnapshot(hash snapshotHash) {
	snapshot := p.snapshots[hash]
	if snapshot == nil {
		return
	}

	delete(p.snapshots, hash)
	delete(p.formatIndex[snapshot.Format], hash)
	delete(p.heightIndex[snapshot.Height], hash)
	for peerID := range p.snapshotPeers[hash] {
		delete(p.peerIndex[peerID], hash)
	}
	delete(p.snapshotPeers, hash)
}

// chunkPool manages chunks for a state sync process, ordering them as necessary. It acts as an
// iterator over an ordered sequence of chunks, but callers can add chunks out-of-order and it will
// be spooled in memory or on disk as appropriate.
type chunkPool struct {
	sync.Mutex
	snapshot   *snapshot
	dir        string
	nextChunk  uint32
	memChunks  map[uint32]*chunk
	diskChunks map[uint32]string
	ch         chan uint32
}

// newChunkPool creates a new chunk pool for a snapshot, using the OS temp dir for storage.
// Callers must call Close() when done.
func newChunkPool(snapshot *snapshot) (*chunkPool, error) {
	dir, err := ioutil.TempDir("", "tm-statesync")
	if err != nil {
		return nil, fmt.Errorf("unable to create temp dir for snapshot chunks: %w", err)
	}
	return &chunkPool{
		snapshot:   snapshot,
		dir:        dir,
		memChunks:  make(map[uint32]*chunk),
		diskChunks: make(map[uint32]string),
		ch:         make(chan uint32, len(snapshot.ChunkHashes)),
	}, nil
}

// Add adds a chunk to the pool, verifying its checksum. It ignores chunks that already exists.
func (p *chunkPool) Add(chunk *chunk) error {
	if chunk.Body == nil {
		return errors.New("cannot add chunk with nil body")
	}
	p.Lock()
	defer p.Unlock()
	if p.ch == nil {
		return errors.New("pool is closed")
	}
	if chunk.Index >= uint32(len(p.snapshot.ChunkHashes)) {
		return fmt.Errorf("received unexpected snapshot chunk %v", chunk.Index)
	}
	if p.memChunks[chunk.Index] != nil || p.diskChunks[chunk.Index] != "" {
		return nil
	}
	if !bytes.Equal(chunk.Hash(), p.snapshot.ChunkHashes[chunk.Index]) {
		return fmt.Errorf("chunk %v hash mismatch, expected %x got %x", chunk.Index,
			p.snapshot.ChunkHashes[chunk.Index], chunk.Hash())
	}

	// Save the chunk in memory if it is an upcoming one, otherwise spool to disk.
	if chunk.Index < p.nextChunk+bufferChunks {
		p.memChunks[chunk.Index] = chunk
	} else {
		path := path.Join(p.dir, strconv.FormatUint(uint64(chunk.Index), 10))
		err := ioutil.WriteFile(path, chunk.Body, 0644)
		if err != nil {
			return fmt.Errorf("failed to save chunk %v to file %v: %w", chunk.Index, path, err)
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
	return nil
}

// Close closes the chunk pool, cleaning up all temporary files.
func (p *chunkPool) Close() error {
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

// Next returns the next chunk from the pool, or a Done error if no more chunks are available.
// It blocks until the chunk is available.
func (p *chunkPool) Next() (*chunk, error) {
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
		return &chunk{
			Index: index,
			Body:  body,
		}, nil
	}
	return nil, fmt.Errorf("chunk %v not found", index)
}
