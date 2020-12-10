package statesync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"sort"
	"time"

	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
)

// snapshotKey is a snapshot key used for lookups.
type snapshotKey [sha256.Size]byte

// snapshot contains data about a snapshot.
type snapshot struct {
	Height   uint64
	Format   uint32
	Chunks   uint32
	Hash     []byte
	Metadata []byte

	trustedAppHash []byte // populated by light client
}

// Key generates a snapshot key, used for lookups. It takes into account not only the height and
// format, but also the chunks, hash, and metadata in case peers have generated snapshots in a
// non-deterministic manner. All fields must be equal for the snapshot to be considered the same.
func (s *snapshot) Key() snapshotKey {
	// Hash.Write() never returns an error.
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%v:%v:%v", s.Height, s.Format, s.Chunks))) //nolint:errcheck // ignore error
	hasher.Write(s.Hash)                                                        //nolint:errcheck // ignore error
	hasher.Write(s.Metadata)                                                    //nolint:errcheck // ignore error
	var key snapshotKey
	copy(key[:], hasher.Sum(nil))
	return key
}

// snapshotPool discovers and aggregates snapshots across peers.
type snapshotPool struct {
	stateProvider StateProvider

	tmsync.Mutex
	snapshots     map[snapshotKey]*snapshot
	snapshotPeers map[snapshotKey]map[string]p2p.PeerID

	// indexes for fast searches
	formatIndex map[uint32]map[snapshotKey]bool
	heightIndex map[uint64]map[snapshotKey]bool
	peerIndex   map[string]map[snapshotKey]bool

	// blacklists for rejected items
	formatBlacklist   map[uint32]bool
	peerBlacklist     map[string]bool
	snapshotBlacklist map[snapshotKey]bool
}

// newSnapshotPool creates a new snapshot pool. The state source is used for
func newSnapshotPool(stateProvider StateProvider) *snapshotPool {
	return &snapshotPool{
		stateProvider:     stateProvider,
		snapshots:         make(map[snapshotKey]*snapshot),
		snapshotPeers:     make(map[snapshotKey]map[string]p2p.PeerID),
		formatIndex:       make(map[uint32]map[snapshotKey]bool),
		heightIndex:       make(map[uint64]map[snapshotKey]bool),
		peerIndex:         make(map[string]map[snapshotKey]bool),
		formatBlacklist:   make(map[uint32]bool),
		peerBlacklist:     make(map[string]bool),
		snapshotBlacklist: make(map[snapshotKey]bool),
	}
}

// Add adds a snapshot to the pool, unless the peer has already sent recentSnapshots
// snapshots. It returns true if this was a new, non-blacklisted snapshot. The
// snapshot height is verified using the light client, and the expected app hash
// is set for the snapshot.
func (p *snapshotPool) Add(peer p2p.PeerID, snapshot *snapshot) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	appHash, err := p.stateProvider.AppHash(ctx, snapshot.Height)
	if err != nil {
		return false, err
	}
	snapshot.trustedAppHash = appHash
	key := snapshot.Key()

	p.Lock()
	defer p.Unlock()

	switch {
	case p.formatBlacklist[snapshot.Format]:
		return false, nil
	case p.peerBlacklist[peer.String()]:
		return false, nil
	case p.snapshotBlacklist[key]:
		return false, nil
	case len(p.peerIndex[peer.String()]) >= recentSnapshots:
		return false, nil
	}

	if p.snapshotPeers[key] == nil {
		p.snapshotPeers[key] = make(map[string]p2p.PeerID)
	}
	p.snapshotPeers[key][peer.String()] = peer

	if p.peerIndex[peer.String()] == nil {
		p.peerIndex[peer.String()] = make(map[snapshotKey]bool)
	}
	p.peerIndex[peer.String()][key] = true

	if p.snapshots[key] != nil {
		return false, nil
	}
	p.snapshots[key] = snapshot

	if p.formatIndex[snapshot.Format] == nil {
		p.formatIndex[snapshot.Format] = make(map[snapshotKey]bool)
	}
	p.formatIndex[snapshot.Format][key] = true

	if p.heightIndex[snapshot.Height] == nil {
		p.heightIndex[snapshot.Height] = make(map[snapshotKey]bool)
	}
	p.heightIndex[snapshot.Height][key] = true

	return true, nil
}

// Best returns the "best" currently known snapshot, if any.
func (p *snapshotPool) Best() *snapshot {
	ranked := p.Ranked()
	if len(ranked) == 0 {
		return nil
	}
	return ranked[0]
}

// GetPeer returns a random peer for a snapshot, if any.
func (p *snapshotPool) GetPeer(snapshot *snapshot) p2p.PeerID {
	peers := p.GetPeers(snapshot)
	if len(peers) == 0 {
		return nil
	}
	return peers[rand.Intn(len(peers))] // nolint:gosec // G404: Use of weak random number generator
}

// GetPeers returns the peers for a snapshot.
func (p *snapshotPool) GetPeers(snapshot *snapshot) []p2p.PeerID {
	key := snapshot.Key()

	p.Lock()
	defer p.Unlock()

	peers := make([]p2p.PeerID, 0, len(p.snapshotPeers[key]))
	for _, peer := range p.snapshotPeers[key] {
		peers = append(peers, peer)
	}

	// sort results, for testability (otherwise order is random, so tests randomly fail)
	sort.Slice(peers, func(a int, b int) bool {
		return bytes.Compare(peers[a], peers[b]) < 0
	})

	return peers
}

// Ranked returns a list of snapshots ranked by preference. The current heuristic is very naïve,
// preferring the snapshot with the greatest height, then greatest format, then greatest number of
// peers. This can be improved quite a lot.
func (p *snapshotPool) Ranked() []*snapshot {
	p.Lock()
	defer p.Unlock()

	candidates := make([]*snapshot, 0, len(p.snapshots))
	for _, snapshot := range p.snapshots {
		candidates = append(candidates, snapshot)
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
		case len(p.snapshotPeers[a.Key()]) > len(p.snapshotPeers[b.Key()]):
			return true
		default:
			return false
		}
	})

	return candidates
}

// Reject rejects a snapshot. Rejected snapshots will never be used again.
func (p *snapshotPool) Reject(snapshot *snapshot) {
	key := snapshot.Key()
	p.Lock()
	defer p.Unlock()

	p.snapshotBlacklist[key] = true
	p.removeSnapshot(key)
}

// RejectFormat rejects a snapshot format. It will never be used again.
func (p *snapshotPool) RejectFormat(format uint32) {
	p.Lock()
	defer p.Unlock()

	p.formatBlacklist[format] = true
	for key := range p.formatIndex[format] {
		p.removeSnapshot(key)
	}
}

// RejectPeer rejects a peer. It will never be used again.
func (p *snapshotPool) RejectPeer(peerID p2p.PeerID) {
	if len(peerID) == 0 {
		return
	}

	p.Lock()
	defer p.Unlock()

	p.removePeer(peerID)
	p.peerBlacklist[peerID.String()] = true
}

// RemovePeer removes a peer from the pool, and any snapshots that no longer have peers.
func (p *snapshotPool) RemovePeer(peerID p2p.PeerID) {
	p.Lock()
	defer p.Unlock()
	p.removePeer(peerID)
}

// removePeer removes a peer. The caller must hold the mutex lock.
func (p *snapshotPool) removePeer(peerID p2p.PeerID) {
	for key := range p.peerIndex[peerID.String()] {
		delete(p.snapshotPeers[key], peerID.String())
		if len(p.snapshotPeers[key]) == 0 {
			p.removeSnapshot(key)
		}
	}

	delete(p.peerIndex, peerID.String())
}

// removeSnapshot removes a snapshot. The caller must hold the mutex lock.
func (p *snapshotPool) removeSnapshot(key snapshotKey) {
	snapshot := p.snapshots[key]
	if snapshot == nil {
		return
	}

	delete(p.snapshots, key)
	delete(p.formatIndex[snapshot.Format], key)
	delete(p.heightIndex[snapshot.Height], key)
	for peerID := range p.snapshotPeers[key] {
		delete(p.peerIndex[peerID], key)
	}
	delete(p.snapshotPeers, key)
}
