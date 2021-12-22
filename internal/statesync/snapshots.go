package statesync

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"

	"github.com/tendermint/tendermint/types"
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
	hasher.Write([]byte(fmt.Sprintf("%v:%v:%v", s.Height, s.Format, s.Chunks)))
	hasher.Write(s.Hash)
	hasher.Write(s.Metadata)
	var key snapshotKey
	copy(key[:], hasher.Sum(nil))
	return key
}

// snapshotPool discovers and aggregates snapshots across peers.
type snapshotPool struct {
	sync.Mutex
	snapshots     map[snapshotKey]*snapshot
	snapshotPeers map[snapshotKey]map[types.NodeID]types.NodeID

	// indexes for fast searches
	formatIndex map[uint32]map[snapshotKey]bool
	heightIndex map[uint64]map[snapshotKey]bool
	peerIndex   map[types.NodeID]map[snapshotKey]bool

	// blacklists for rejected items
	formatBlacklist   map[uint32]bool
	peerBlacklist     map[types.NodeID]bool
	snapshotBlacklist map[snapshotKey]bool
}

// newSnapshotPool creates a new empty snapshot pool.
func newSnapshotPool() *snapshotPool {
	return &snapshotPool{
		snapshots:         make(map[snapshotKey]*snapshot),
		snapshotPeers:     make(map[snapshotKey]map[types.NodeID]types.NodeID),
		formatIndex:       make(map[uint32]map[snapshotKey]bool),
		heightIndex:       make(map[uint64]map[snapshotKey]bool),
		peerIndex:         make(map[types.NodeID]map[snapshotKey]bool),
		formatBlacklist:   make(map[uint32]bool),
		peerBlacklist:     make(map[types.NodeID]bool),
		snapshotBlacklist: make(map[snapshotKey]bool),
	}
}

// Add adds a snapshot to the pool, unless the peer has already sent recentSnapshots
// snapshots. It returns true if this was a new, non-blacklisted snapshot. The
// snapshot height is verified using the light client, and the expected app hash
// is set for the snapshot.
func (p *snapshotPool) Add(peerID types.NodeID, snapshot *snapshot) (bool, error) {
	key := snapshot.Key()

	p.Lock()
	defer p.Unlock()

	switch {
	case p.formatBlacklist[snapshot.Format]:
		return false, nil
	case p.peerBlacklist[peerID]:
		return false, nil
	case p.snapshotBlacklist[key]:
		return false, nil
	case len(p.peerIndex[peerID]) >= recentSnapshots:
		return false, nil
	}

	if p.snapshotPeers[key] == nil {
		p.snapshotPeers[key] = make(map[types.NodeID]types.NodeID)
	}
	p.snapshotPeers[key][peerID] = peerID

	if p.peerIndex[peerID] == nil {
		p.peerIndex[peerID] = make(map[snapshotKey]bool)
	}
	p.peerIndex[peerID][key] = true

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
func (p *snapshotPool) GetPeer(snapshot *snapshot) types.NodeID {
	peers := p.GetPeers(snapshot)
	if len(peers) == 0 {
		return ""
	}
	return peers[rand.Intn(len(peers))] // nolint:gosec // G404: Use of weak random number generator
}

// GetPeers returns the peers for a snapshot.
func (p *snapshotPool) GetPeers(snapshot *snapshot) []types.NodeID {
	key := snapshot.Key()

	p.Lock()
	defer p.Unlock()

	peers := make([]types.NodeID, 0, len(p.snapshotPeers[key]))
	for _, peer := range p.snapshotPeers[key] {
		peers = append(peers, peer)
	}

	// sort results, for testability (otherwise order is random, so tests randomly fail)
	sort.Slice(peers, func(a int, b int) bool {
		return strings.Compare(string(peers[a]), string(peers[b])) < 0
	})

	return peers
}

// Ranked returns a list of snapshots ranked by preference. The current heuristic is very naïve,
// preferring the snapshot with the greatest height, then greatest format, then greatest number of
// peers. This can be improved quite a lot.
func (p *snapshotPool) Ranked() []*snapshot {
	p.Lock()
	defer p.Unlock()

	if len(p.snapshots) == 0 {
		return []*snapshot{}
	}

	numPeers := make([]int, 0, len(p.snapshots))
	for key := range p.snapshots {
		numPeers = append(numPeers, len(p.snapshotPeers[key]))
	}
	sort.Ints(numPeers)
	median := len(numPeers) / 2
	if len(numPeers)%2 == 0 {
		median = (numPeers[median-1] + numPeers[median]) / 2
	} else {
		median = numPeers[median]
	}

	commonCandidates := make([]*snapshot, 0, len(p.snapshots)/2)
	uncommonCandidates := make([]*snapshot, 0, len(p.snapshots)/2)
	for key := range p.snapshots {
		if len(p.snapshotPeers[key]) > median {
			commonCandidates = append(commonCandidates, p.snapshots[key])
			continue
		}

		uncommonCandidates = append(uncommonCandidates, p.snapshots[key])
	}

	sort.Slice(commonCandidates, p.sorterFactory(commonCandidates))
	sort.Slice(uncommonCandidates, p.sorterFactory(uncommonCandidates))

	return append(commonCandidates, uncommonCandidates...)
}

func (p *snapshotPool) sorterFactory(candidates []*snapshot) func(int, int) bool {
	return func(i, j int) bool {
		a := candidates[i]
		b := candidates[j]

		switch {
		case a.Height > b.Height:
			return true
		case a.Height < b.Height:
			return false
		case len(p.snapshotPeers[a.Key()]) > len(p.snapshotPeers[b.Key()]):
			return true
		case a.Format > b.Format:
			return true
		case a.Format < b.Format:
			return false
		default:
			return false
		}
	}
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
func (p *snapshotPool) RejectPeer(peerID types.NodeID) {
	if len(peerID) == 0 {
		return
	}

	p.Lock()
	defer p.Unlock()

	p.removePeer(peerID)
	p.peerBlacklist[peerID] = true
}

// RemovePeer removes a peer from the pool, and any snapshots that no longer have peers.
func (p *snapshotPool) RemovePeer(peerID types.NodeID) {
	p.Lock()
	defer p.Unlock()
	p.removePeer(peerID)
}

// removePeer removes a peer. The caller must hold the mutex lock.
func (p *snapshotPool) removePeer(peerID types.NodeID) {
	for key := range p.peerIndex[peerID] {
		delete(p.snapshotPeers[key], peerID)
		if len(p.snapshotPeers[key]) == 0 {
			p.removeSnapshot(key)
		}
	}

	delete(p.peerIndex, peerID)
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
