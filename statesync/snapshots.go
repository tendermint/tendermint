package statesync

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	lite "github.com/tendermint/tendermint/lite2"
	"github.com/tendermint/tendermint/p2p"
)

const (
	// recentSnapshots is the number of recent snapshots to send and receive per peer.
	recentSnapshots = 10
)

// snapshotHash is a snapshot hash, used for lookups.
type snapshotHash [sha256.Size]byte

// snapshot contains data about a snapshot.
type snapshot struct {
	Height      uint64
	Format      uint32
	ChunkHashes [][]byte
	Metadata    []byte

	trustedAppHash []byte // populated by light client
}

// Hash generates a snapshot hash, used for lookups. It takes into account not only the height
// and format, but also the chunk hashes and metadata in case peers have generated snapshots in a
// non-deterministic manner such that chunks from different peers wouldn't fit together.
func (s *snapshot) Hash() snapshotHash {
	// Hash.Write() never returns an error.
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%v:%v", s.Height, s.Format)))
	if s.Metadata != nil {
		hasher.Write(s.Metadata)
	}
	for _, chunkHash := range s.ChunkHashes {
		hasher.Write(chunkHash)
	}
	var hash snapshotHash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

// snapshotPool discovers and aggregates snapshots across peers.
type snapshotPool struct {
	lc *lite.Client

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
func newSnapshotPool(lc *lite.Client) *snapshotPool {
	return &snapshotPool{
		lc:                lc,
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
// returns true if this was a new, non-blacklisted snapshot. The snapshot height is verified using
// the light client, and the expected app hash is set for the snapshot.
func (p *snapshotPool) Add(peer p2p.Peer, snapshot *snapshot) (bool, error) {
	// FIXME Check if the light client deduplicates concurrent requests for the same height.
	// Otherwise we'll have to manage this ourself.
	appHash, err := p.fetchTrustedAppHash(snapshot.Height)
	if err != nil {
		return false, err
	}
	snapshot.trustedAppHash = appHash
	hash := snapshot.Hash()

	p.Lock()
	defer p.Unlock()

	switch {
	case p.formatBlacklist[snapshot.Format]:
		return false, nil
	case p.heightBlacklist[snapshot.Height]:
		return false, nil
	case p.snapshotBlacklist[hash]:
		return false, nil
	case p.snapshots[hash] != nil:
		return false, nil
	case len(p.peerIndex[peer.ID()]) >= recentSnapshots:
		return false, nil
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

	return true, nil
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

// GetPeer returns a random peer for a snapshot, if any.
func (p *snapshotPool) GetPeer(snapshot *snapshot) p2p.Peer {
	hash := snapshot.Hash()
	p.Lock()
	defer p.Unlock()

	if len(p.snapshotPeers[hash]) == 0 {
		return nil
	}

	peers := make([]p2p.Peer, 0, len(p.snapshotPeers[hash]))
	for _, peer := range p.snapshotPeers[hash] {
		peers = append(peers, peer)
	}
	return peers[rand.Intn(len(peers))]
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

// fetchTrustedAppHash fetches the app hash for a given height using the light client.
func (p *snapshotPool) fetchTrustedAppHash(height uint64) ([]byte, error) {
	// we have to fetch the next height, which contains the app hash for the previous height.
	header, err := p.lc.VerifyHeaderAtHeight(int64(height+1), time.Now())
	if err != nil {
		return nil, err
	}
	return header.AppHash, nil
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
