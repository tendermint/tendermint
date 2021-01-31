package p2p

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/orderedcode"
	dbm "github.com/tendermint/tm-db"

	p2pproto "github.com/tendermint/tendermint/proto/tendermint/p2p"
)

// PeerStatus specifies peer statuses.
type PeerStatus string

const (
	PeerStatusNew     = PeerStatus("new")     // New peer which we haven't tried to contact yet.
	PeerStatusUp      = PeerStatus("up")      // Peer which we have an active connection to.
	PeerStatusDown    = PeerStatus("down")    // Peer which we're temporarily disconnected from.
	PeerStatusRemoved = PeerStatus("removed") // Peer which has been removed.
	PeerStatusBanned  = PeerStatus("banned")  // Peer which is banned for misbehavior.
)

// PeerError is a peer error reported by a reactor via the Error channel. The
// severity may cause the peer to be disconnected or banned depending on policy.
type PeerError struct {
	PeerID   NodeID
	Err      error
	Severity PeerErrorSeverity
}

// PeerErrorSeverity determines the severity of a peer error.
type PeerErrorSeverity string

const (
	PeerErrorSeverityLow      PeerErrorSeverity = "low"      // Mostly ignored.
	PeerErrorSeverityHigh     PeerErrorSeverity = "high"     // May disconnect.
	PeerErrorSeverityCritical PeerErrorSeverity = "critical" // Ban.
)

// PeerUpdatesCh defines a wrapper around a PeerUpdate go channel that allows
// a reactor to listen for peer updates and safely close it when stopping.
type PeerUpdatesCh struct {
	closeOnce sync.Once

	// updatesCh defines the go channel in which the router sends peer updates to
	// reactors. Each reactor will have its own PeerUpdatesCh to listen for updates
	// from.
	updatesCh chan PeerUpdate

	// doneCh is used to signal that a PeerUpdatesCh is closed. It is the
	// reactor's responsibility to invoke Close.
	doneCh chan struct{}
}

// NewPeerUpdates returns a reference to a new PeerUpdatesCh.
func NewPeerUpdates(updatesCh chan PeerUpdate) *PeerUpdatesCh {
	return &PeerUpdatesCh{
		updatesCh: updatesCh,
		doneCh:    make(chan struct{}),
	}
}

// Updates returns a read-only go channel where a consuming reactor can listen
// for peer updates sent from the router.
func (puc *PeerUpdatesCh) Updates() <-chan PeerUpdate {
	return puc.updatesCh
}

// Close closes the PeerUpdatesCh channel. It should only be closed by the respective
// reactor when stopping and ensure nothing is listening for updates.
//
// NOTE: After a PeerUpdatesCh is closed, the router may safely assume it can no
// longer send on the internal updatesCh, however it should NEVER explicitly close
// it as that could result in panics by sending on a closed channel.
func (puc *PeerUpdatesCh) Close() {
	puc.closeOnce.Do(func() {
		close(puc.doneCh)
	})
}

// Done returns a read-only version of the PeerUpdatesCh's internal doneCh go
// channel that should be used by a router to signal when it is safe to explicitly
// not send any peer updates.
func (puc *PeerUpdatesCh) Done() <-chan struct{} {
	return puc.doneCh
}

// PeerUpdate is a peer status update for reactors.
type PeerUpdate struct {
	PeerID NodeID
	Status PeerStatus
}

// PeerScore is a numeric score assigned to a peer (higher is better).
type PeerScore uint16

const (
	// PeerScorePersistent is added for persistent peers.
	PeerScorePersistent PeerScore = 100
)

// PeerManager manages peer lifecycle information, using a peerStore for
// underlying storage. Its primary purpose is to determine which peers to
// connect to next, make sure a peer only has a single active connection (either
// inbound or outbound), and evict peers to make room for higher-scored peers.
// It does not manage actual connections (this is handled by the Router),
// only the peer lifecycle state.
//
// We track dialing and connected states independently. This allows us to accept
// an inbound connection from a peer while the router is also dialing an
// outbound connection to that same peer, which will cause the dialer to
// eventually error when attempting to mark the peer as connected. This also
// avoids race conditions where multiple goroutines may end up dialing a peer if
// an incoming connection was briefly accepted and disconnected while we were
// also dialing.
//
// For an outbound connection, the flow is as follows:
// - DialNext: returns a peer address to dial, marking the peer as dialing.
// - DialFailed: reports a dial failure, unmarking the peer as dialing.
// - Dialed: successfully dialed, unmarking as dialing and marking as connected
//   (or erroring if already connected).
// - Ready: routing is up, broadcasts a PeerStatusUp peer update to subscribers.
// - Disconnected: peer disconnects, unmarking as connected and broadcasts a
//   PeerStatusDown peer update.
//
// For an inbound connection, the flow is as follows:
// - Accepted: successfully accepted connection, marking as connected (or erroring
//   if already connected).
// - Ready: routing is up, broadcasts a PeerStatusUp peer update to subscribers.
// - Disconnected: peer disconnects, unmarking as connected and broadcasts a
//   PeerStatusDown peer update.
//
// When evicting peers, either because peers are explicitly scheduled for
// eviction or we are connected to too many peers, the flow is as follows:
// - EvictNext: if marked evict and connected, unmark evict and mark evicting.
//   If beyond MaxConnected, pick lowest-scored peer and mark evicting.
// - Disconnected: unmark connected, evicting, evict, and broadcast a
//   PeerStatusDown peer update.
//
// If all connection slots are full (at MaxConnections), we can use up to
// MaxConnectionsUpgrade additional connections to probe any higher-scored
// unconnected peers, and if we reach them (or they reach us) we allow the
// connection and evict a lower-scored peer. We mark the lower-scored peer as
// upgrading[from]=to to make sure no other higher-scored peers can claim the
// same one for an upgrade. The flow is as follows:
// - Accepted: if upgrade is possible, mark connected and add lower-scored to evict.
// - DialNext: if upgrade is possible, mark upgrading[from]=to and dialing.
// - DialFailed: unmark upgrading[from]=to and dialing.
// - Dialed: unmark upgrading[from]=to and dialing, mark as connected, add
//   lower-scored to evict.
// - EvictNext: pick peer from evict, mark as evicting.
// - Disconnected: unmark connected, upgrading[from]=to, evict, evicting.
//
// FIXME: The old stack supports ABCI-based peer ID filtering via
// /p2p/filter/id/<ID> queries, we should implement this here as well by taking
// a peer ID filtering callback in PeerManagerOptions and configuring it during
// Node setup.
type PeerManager struct {
	options     PeerManagerOptions
	wakeDialCh  chan struct{} // wakes up DialNext() on relevant peer changes
	wakeEvictCh chan struct{} // wakes up EvictNext() on relevant peer changes
	closeCh     chan struct{} // signal channel for Close()
	closeOnce   sync.Once

	mtx           sync.Mutex
	store         *peerStore
	dialing       map[NodeID]bool                   // peers being dialed (DialNext -> Dialed/DialFail)
	upgrading     map[NodeID]NodeID                 // peers claimed for upgrade (DialNext -> Dialed/DialFail)
	connected     map[NodeID]bool                   // connected peers (Dialed/Accepted -> Disconnected)
	evict         map[NodeID]bool                   // peers scheduled for eviction (Connected -> EvictNext)
	evicting      map[NodeID]bool                   // peers being evicted (EvictNext -> Disconnected)
	subscriptions map[*PeerUpdatesCh]*PeerUpdatesCh // keyed by struct identity (address)
}

// PeerManagerOptions specifies options for a PeerManager.
type PeerManagerOptions struct {
	// PersistentPeers are peers that we want to maintain persistent connections
	// to. These will be scored higher than other peers, and if
	// MaxConnectedUpgrade is non-zero any lower-scored peers will be evicted if
	// necessary to make room for these.
	PersistentPeers []NodeID

	// MaxPeers is the maximum number of peers to track information about, i.e.
	// store in the peer store. When exceeded, the lowest-scored unconnected peers
	// will be deleted. 0 means no limit.
	MaxPeers uint16

	// MaxConnected is the maximum number of connected peers (inbound and
	// outbound). 0 means no limit.
	MaxConnected uint16

	// MaxConnectedUpgrade is the maximum number of additional connections to
	// use for probing any better-scored peers to upgrade to when all connection
	// slots are full. 0 disables peer upgrading.
	//
	// For example, if we are already connected to MaxConnected peers, but we
	// know or learn about better-scored peers (e.g. configured persistent
	// peers) that we are not connected too, then we can probe these peers by
	// using up to MaxConnectedUpgrade connections, and once connected evict the
	// lowest-scored connected peers. This also works for inbound connections,
	// i.e. if a higher-scored peer attempts to connect to us, we can accept
	// the connection and evict a lower-scored peer.
	MaxConnectedUpgrade uint16

	// MinRetryTime is the minimum time to wait between retries. Retry times
	// double for each retry, up to MaxRetryTime. 0 disables retries.
	MinRetryTime time.Duration

	// MaxRetryTime is the maximum time to wait between retries. 0 means
	// no maximum, in which case the retry time will keep doubling.
	MaxRetryTime time.Duration

	// MaxRetryTimePersistent is the maximum time to wait between retries for
	// peers listed in PersistentPeers. 0 uses MaxRetryTime instead.
	MaxRetryTimePersistent time.Duration

	// RetryTimeJitter is the upper bound of a random interval added to
	// retry times, to avoid thundering herds. 0 disables jutter.
	RetryTimeJitter time.Duration
}

// NewPeerManager creates a new peer manager.
func NewPeerManager(peerDB dbm.DB, options PeerManagerOptions) (*PeerManager, error) {
	store, err := newPeerStore(peerDB)
	if err != nil {
		return nil, err
	}
	peerManager := &PeerManager{
		options: options,
		closeCh: make(chan struct{}),

		// We use a buffer of size 1 for these trigger channels, with
		// non-blocking sends. This ensures that if e.g. wakeDial() is called
		// multiple times before the initial trigger is picked up we only
		// process the trigger once.
		//
		// FIXME: This should maybe be a libs/sync type.
		wakeDialCh:  make(chan struct{}, 1),
		wakeEvictCh: make(chan struct{}, 1),

		store:         store,
		dialing:       map[NodeID]bool{},
		upgrading:     map[NodeID]NodeID{},
		connected:     map[NodeID]bool{},
		evict:         map[NodeID]bool{},
		evicting:      map[NodeID]bool{},
		subscriptions: map[*PeerUpdatesCh]*PeerUpdatesCh{},
	}
	if err = peerManager.configurePeers(); err != nil {
		return nil, err
	}
	if err = peerManager.prunePeers(); err != nil {
		return nil, err
	}
	return peerManager, nil
}

// configurePeers configures peers in the peer store with ephemeral runtime
// configuration, e.g. setting peerInfo.Persistent based on
// PeerManagerOptions.PersistentPeers. The caller must hold the mutex lock.
func (m *PeerManager) configurePeers() error {
	for _, peerID := range m.options.PersistentPeers {
		if peer, ok := m.store.Get(peerID); ok {
			peer.Persistent = true
			if err := m.store.Set(peer); err != nil {
				return err
			}
		}
	}
	return nil
}

// prunePeers removes peers from the peer store if it contains more than
// MaxPeers peers. The lowest-scored non-connected peers are removed.
// The caller must hold the mutex lock.
func (m *PeerManager) prunePeers() error {
	if m.options.MaxPeers == 0 || m.store.Size() <= int(m.options.MaxPeers) {
		return nil
	}
	m.mtx.Lock()
	defer m.mtx.Unlock()

	ranked := m.store.Ranked()
	for i := len(ranked) - 1; i >= 0; i-- {
		peerID := ranked[i].ID
		switch {
		case m.store.Size() <= int(m.options.MaxPeers):
			break
		case m.dialing[peerID]:
		case m.connected[peerID]:
		case m.evicting[peerID]:
		default:
			if err := m.store.Delete(peerID); err != nil {
				return err
			}
		}
	}
	return nil
}

// Close closes the peer manager, releasing resources allocated with it
// (specifically any running goroutines).
func (m *PeerManager) Close() {
	m.closeOnce.Do(func() {
		close(m.closeCh)
	})
}

// Add adds a peer to the manager, given as an address. If the peer already
// exists, the address is added to it.
func (m *PeerManager) Add(address PeerAddress) error {
	if err := address.Validate(); err != nil {
		return err
	}
	m.mtx.Lock()
	defer m.mtx.Unlock()

	peer, ok := m.store.Get(address.NodeID)
	if !ok {
		peer = m.makePeerInfo(address.NodeID)
	}
	if _, ok := peer.AddressInfo[address.String()]; !ok {
		peer.AddressInfo[address.String()] = &peerAddressInfo{Address: address}
	}
	if err := m.store.Set(peer); err != nil {
		return err
	}
	if err := m.prunePeers(); err != nil {
		return err
	}
	m.wakeDial()
	return nil
}

// Advertise returns a list of peer addresses to advertise to a peer.
//
// FIXME: This is fairly naïve and only returns the addresses of the
// highest-ranked peers.
func (m *PeerManager) Advertise(peerID NodeID, limit uint16) []PeerAddress {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	addresses := make([]PeerAddress, 0, limit)
	for _, peer := range m.store.Ranked() {
		if peer.ID == peerID {
			continue
		}
		for _, addressInfo := range peer.AddressInfo {
			if len(addresses) >= int(limit) {
				return addresses
			}
			addresses = append(addresses, addressInfo.Address)
		}
	}
	return addresses
}

// makePeerInfo creates a peerInfo for a new peer.
func (m *PeerManager) makePeerInfo(id NodeID) peerInfo {
	isPersistent := false
	for _, p := range m.options.PersistentPeers {
		if id == p {
			isPersistent = true
			break
		}
	}
	return peerInfo{
		ID:          id,
		Persistent:  isPersistent,
		AddressInfo: map[string]*peerAddressInfo{},
	}
}

// Subscribe subscribes to peer updates. The caller must consume the peer
// updates in a timely fashion and close the subscription when done, since
// delivery is guaranteed and will block peer connection/disconnection
// otherwise.
func (m *PeerManager) Subscribe() *PeerUpdatesCh {
	// FIXME: We may want to use a size 1 buffer here. When the router
	// broadcasts a peer update it has to loop over all of the
	// subscriptions, and we want to avoid blocking and waiting for a
	// context switch before continuing to the next subscription. This also
	// prevents tail latencies from compounding across updates. We also want
	// to make sure the subscribers are reasonably in sync, so it should be
	// kept at 1. However, this should be benchmarked first.
	peerUpdates := NewPeerUpdates(make(chan PeerUpdate))
	m.mtx.Lock()
	m.subscriptions[peerUpdates] = peerUpdates
	m.mtx.Unlock()

	go func() {
		<-peerUpdates.Done()
		m.mtx.Lock()
		delete(m.subscriptions, peerUpdates)
		m.mtx.Unlock()
	}()
	return peerUpdates
}

// broadcast broadcasts a peer update to all subscriptions. The caller must
// already hold the mutex lock. This means the mutex is held for the duration
// of the broadcast, which we want to make sure all subscriptions receive all
// updates in the same order.
//
// FIXME: Consider using more fine-grained mutexes here, and/or a channel to
// enforce ordering of updates.
func (m *PeerManager) broadcast(peerUpdate PeerUpdate) {
	for _, sub := range m.subscriptions {
		select {
		case sub.updatesCh <- peerUpdate:
		case <-sub.doneCh:
		}
	}
}

// DialNext finds an appropriate peer address to dial, and marks it as dialing.
// If no peer is found, or all connection slots are full, it blocks until one
// becomes available. The caller must call Dialed() or DialFailed() for the
// returned peer. The context can be used to cancel the call.
func (m *PeerManager) DialNext(ctx context.Context) (NodeID, PeerAddress, error) {
	for {
		id, address, err := m.TryDialNext()
		if err != nil || id != "" {
			return id, address, err
		}
		select {
		case <-m.wakeDialCh:
		case <-ctx.Done():
			return "", PeerAddress{}, ctx.Err()
		}
	}
}

// TryDialNext is equivalent to DialNext(), but immediately returns an empty
// peer ID if no peers or connection slots are available.
func (m *PeerManager) TryDialNext() (NodeID, PeerAddress, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// We allow dialing MaxConnected+MaxConnectedUpgrade peers. Including
	// MaxConnectedUpgrade allows us to probe additional peers that have a
	// higher score than any other peers, and if successful evict it.
	if m.options.MaxConnected > 0 &&
		len(m.connected)+len(m.dialing) >= int(m.options.MaxConnected)+int(m.options.MaxConnectedUpgrade) {
		return "", PeerAddress{}, nil
	}

	for _, peer := range m.store.Ranked() {
		if m.dialing[peer.ID] || m.connected[peer.ID] {
			continue
		}

		for _, addressInfo := range peer.AddressInfo {
			if time.Since(addressInfo.LastDialFailure) < m.retryDelay(addressInfo.DialFailures, peer.Persistent) {
				continue
			}

			// We now have an eligible address to dial. If we're full but have
			// upgrade capacity (as checked above), we find a lower-scored peer
			// we can replace and mark it as upgrading so noone else claims it.
			//
			// If we don't find one, there is no point in trying additional
			// peers, since they will all have the same or lower score than this
			// peer (since they're ordered by score via peerStore.Ranked).
			if m.options.MaxConnected > 0 && len(m.connected) >= int(m.options.MaxConnected) {
				upgradeFromPeer := m.findUpgradeCandidate(peer.ID, peer.Score())
				if upgradeFromPeer == "" {
					return "", PeerAddress{}, nil
				}
				m.upgrading[upgradeFromPeer] = peer.ID
			}

			m.dialing[peer.ID] = true
			return peer.ID, addressInfo.Address, nil
		}
	}
	return "", PeerAddress{}, nil
}

// wakeDial is used to notify DialNext about changes that *may* cause new
// peers to become eligible for dialing, such as peer disconnections and
// retry timeouts.
func (m *PeerManager) wakeDial() {
	// The channel has a 1-size buffer. A non-blocking send ensures
	// we only queue up at most 1 trigger between each DialNext().
	select {
	case m.wakeDialCh <- struct{}{}:
	default:
	}
}

// wakeEvict is used to notify EvictNext about changes that *may* cause
// peers to become eligible for eviction, such as peer upgrades.
func (m *PeerManager) wakeEvict() {
	// The channel has a 1-size buffer. A non-blocking send ensures
	// we only queue up at most 1 trigger between each EvictNext().
	select {
	case m.wakeEvictCh <- struct{}{}:
	default:
	}
}

// retryDelay calculates a dial retry delay using exponential backoff, based on
// retry settings in PeerManagerOptions. If MinRetryTime is 0, this returns
// MaxInt64 (i.e. an infinite retry delay, effectively disabling retries).
func (m *PeerManager) retryDelay(failures uint32, persistent bool) time.Duration {
	if failures == 0 {
		return 0
	}
	if m.options.MinRetryTime == 0 {
		return time.Duration(math.MaxInt64)
	}
	maxDelay := m.options.MaxRetryTime
	if persistent && m.options.MaxRetryTimePersistent > 0 {
		maxDelay = m.options.MaxRetryTimePersistent
	}

	delay := m.options.MinRetryTime * time.Duration(math.Pow(2, float64(failures)))
	if maxDelay > 0 && delay > maxDelay {
		delay = maxDelay
	}
	// FIXME: This should use a PeerManager-scoped RNG.
	delay += time.Duration(rand.Int63n(int64(m.options.RetryTimeJitter))) // nolint:gosec
	return delay
}

// DialFailed reports a failed dial attempt. This will make the peer available
// for dialing again when appropriate.
//
// FIXME: This should probably delete or mark bad addresses/peers after some time.
func (m *PeerManager) DialFailed(peerID NodeID, address PeerAddress) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	delete(m.dialing, peerID)
	for from, to := range m.upgrading {
		if to == peerID {
			delete(m.upgrading, from) // Unmark failed upgrade attempt.
		}
	}

	peer, ok := m.store.Get(peerID)
	if !ok { // Peer may have been removed while dialing, ignore.
		return nil
	}
	addressInfo, ok := peer.AddressInfo[address.String()]
	if !ok {
		return nil // Assume the address has been removed, ignore.
	}
	addressInfo.LastDialFailure = time.Now().UTC()
	addressInfo.DialFailures++
	if err := m.store.Set(peer); err != nil {
		return err
	}

	// We spawn a goroutine that notifies DialNext() again when the retry
	// timeout has elapsed, so that we can consider dialing it again.
	go func() {
		retryDelay := m.retryDelay(addressInfo.DialFailures, peer.Persistent)
		if retryDelay == time.Duration(math.MaxInt64) {
			return
		}
		// Use an explicit timer with deferred cleanup instead of
		// time.After(), to avoid leaking goroutines on PeerManager.Close().
		timer := time.NewTimer(retryDelay)
		defer timer.Stop()
		select {
		case <-timer.C:
			m.wakeDial()
		case <-m.closeCh:
		}
	}()

	m.wakeDial()
	return nil
}

// Dialed marks a peer as successfully dialed. Any further incoming connections
// will be rejected, and once disconnected the peer may be dialed again.
func (m *PeerManager) Dialed(peerID NodeID, address PeerAddress) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	delete(m.dialing, peerID)

	var upgradeFromPeer NodeID
	for from, to := range m.upgrading {
		if to == peerID {
			delete(m.upgrading, from)
			upgradeFromPeer = from
			// Don't break, just in case this peer was marked as upgrading for
			// multiple lower-scored peers (shouldn't really happen).
		}
	}

	if m.connected[peerID] {
		return fmt.Errorf("peer %v is already connected", peerID)
	}
	if m.options.MaxConnected > 0 &&
		len(m.connected) >= int(m.options.MaxConnected)+int(m.options.MaxConnectedUpgrade) {
		return fmt.Errorf("already connected to maximum number of peers")
	}

	peer, ok := m.store.Get(peerID)
	if !ok {
		return fmt.Errorf("peer %q was removed while dialing", peerID)
	}
	now := time.Now().UTC()
	peer.LastConnected = now
	if addressInfo, ok := peer.AddressInfo[address.String()]; ok {
		addressInfo.DialFailures = 0
		addressInfo.LastDialSuccess = now
		// If not found, assume address has been removed.
	}
	if err := m.store.Set(peer); err != nil {
		return err
	}

	if upgradeFromPeer != "" && m.options.MaxConnected > 0 &&
		len(m.connected) >= int(m.options.MaxConnected) {
		// Look for an even lower-scored peer that may have appeared
		// since we started the upgrade.
		if p, ok := m.store.Get(upgradeFromPeer); ok {
			if u := m.findUpgradeCandidate(p.ID, p.Score()); u != "" {
				upgradeFromPeer = u
			}
		}
		m.evict[upgradeFromPeer] = true
	}
	m.connected[peerID] = true
	m.wakeEvict()

	return nil
}

// Accepted marks an incoming peer connection successfully accepted. If the peer
// is already connected or we don't allow additional connections then this will
// return an error.
//
// If full but MaxConnectedUpgrade is non-zero and the incoming peer is
// better-scored than any existing peers, then we accept it and evict a
// lower-scored peer.
//
// NOTE: We can't take an address here, since e.g. TCP uses a different port
// number for outbound traffic than inbound traffic, so the peer's endpoint
// wouldn't necessarily be an appropriate address to dial.
//
// FIXME: When we accept a connection from a peer, we should register that
// peer's address in the peer store so that we can dial it later. In order to do
// that, we'll need to get the remote address after all, but as noted above that
// can't be the remote endpoint since that will usually have the wrong port
// number.
func (m *PeerManager) Accepted(peerID NodeID) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.connected[peerID] {
		return fmt.Errorf("peer %q is already connected", peerID)
	}
	if m.options.MaxConnected > 0 &&
		len(m.connected) >= int(m.options.MaxConnected)+int(m.options.MaxConnectedUpgrade) {
		return fmt.Errorf("already connected to maximum number of peers")
	}

	peer, ok := m.store.Get(peerID)
	if !ok {
		peer = m.makePeerInfo(peerID)
	}

	// If all connections slots are full, but we allow upgrades (and we checked
	// above that we have upgrade capacity), then we can look for a lower-scored
	// peer to replace and if found accept the connection anyway and evict it.
	var upgradeFromPeer NodeID
	if m.options.MaxConnected > 0 && len(m.connected) >= int(m.options.MaxConnected) {
		upgradeFromPeer = m.findUpgradeCandidate(peer.ID, peer.Score())
		if upgradeFromPeer == "" {
			return fmt.Errorf("already connected to maximum number of peers")
		}
	}

	peer.LastConnected = time.Now().UTC()
	if err := m.store.Set(peer); err != nil {
		return err
	}

	m.connected[peerID] = true
	if upgradeFromPeer != "" {
		m.evict[upgradeFromPeer] = true
	}
	m.wakeEvict()
	return nil
}

// Ready marks a peer as ready, broadcasting status updates to subscribers. The
// peer must already be marked as connected. This is separate from Dialed() and
// Accepted() to allow the router to set up its internal queues before reactors
// start sending messages.
func (m *PeerManager) Ready(peerID NodeID) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.connected[peerID] {
		m.broadcast(PeerUpdate{
			PeerID: peerID,
			Status: PeerStatusUp,
		})
	}
}

// Disconnected unmarks a peer as connected, allowing new connections to be
// established.
func (m *PeerManager) Disconnected(peerID NodeID) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	delete(m.connected, peerID)
	delete(m.upgrading, peerID)
	delete(m.evict, peerID)
	delete(m.evicting, peerID)
	m.broadcast(PeerUpdate{
		PeerID: peerID,
		Status: PeerStatusDown,
	})
	m.wakeDial()
	return nil
}

// EvictNext returns the next peer to evict (i.e. disconnect). If no evictable
// peers are found, the call will block until one becomes available or the
// context is cancelled.
func (m *PeerManager) EvictNext(ctx context.Context) (NodeID, error) {
	for {
		id, err := m.TryEvictNext()
		if err != nil || id != "" {
			return id, err
		}
		select {
		case <-m.wakeEvictCh:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// TryEvictNext is equivalent to EvictNext, but immediately returns an empty
// node ID if no evictable peers are found.
func (m *PeerManager) TryEvictNext() (NodeID, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// If any connected peers are explicitly scheduled for eviction, we return a
	// random one.
	for peerID := range m.evict {
		delete(m.evict, peerID)
		if m.connected[peerID] && !m.evicting[peerID] {
			m.evicting[peerID] = true
			return peerID, nil
		}
	}

	// If we're below capacity, we don't need to evict anything.
	if m.options.MaxConnected == 0 ||
		len(m.connected)-len(m.evicting) <= int(m.options.MaxConnected) {
		return "", nil
	}

	// If we're above capacity, just pick the lowest-ranked peer to evict.
	ranked := m.store.Ranked()
	for i := len(ranked) - 1; i >= 0; i-- {
		peer := ranked[i]
		if m.connected[peer.ID] && !m.evicting[peer.ID] {
			m.evicting[peer.ID] = true
			return peer.ID, nil
		}
	}

	return "", nil
}

// findUpgradeCandidate looks for a lower-scored peer that we could evict
// to make room for the given peer. Returns an empty ID if none is found.
// The caller must hold the mutex lock.
func (m *PeerManager) findUpgradeCandidate(id NodeID, score PeerScore) NodeID {
	ranked := m.store.Ranked()
	for i := len(ranked) - 1; i >= 0; i-- {
		candidate := ranked[i]
		switch {
		case candidate.Score() >= score:
			return "" // no further peers can be scored lower, due to sorting
		case !m.connected[candidate.ID]:
		case m.evict[candidate.ID]:
		case m.evicting[candidate.ID]:
		case m.upgrading[candidate.ID] != "":
		default:
			return candidate.ID
		}
	}
	return ""
}

// GetHeight returns a peer's height, as reported via SetHeight. If the peer
// or height is unknown, this returns 0.
//
// FIXME: This is a temporary workaround for the peer state stored via the
// legacy Peer.Set() and Peer.Get() APIs, used to share height state between the
// consensus and mempool reactors. These dependencies should be removed from the
// reactors, and instead query this information independently via new P2P
// protocol additions.
func (m *PeerManager) GetHeight(peerID NodeID) int64 {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	peer, _ := m.store.Get(peerID)
	return peer.Height
}

// SetHeight stores a peer's height, making it available via GetHeight. If the
// peer is unknown, it is created.
//
// FIXME: This is a temporary workaround for the peer state stored via the
// legacy Peer.Set() and Peer.Get() APIs, used to share height state between the
// consensus and mempool reactors. These dependencies should be removed from the
// reactors, and instead query this information independently via new P2P
// protocol additions.
func (m *PeerManager) SetHeight(peerID NodeID, height int64) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	peer, ok := m.store.Get(peerID)
	if !ok {
		peer = m.makePeerInfo(peerID)
	}
	peer.Height = height
	return m.store.Set(peer)
}

// peerStore stores information about peers. It is not thread-safe, assuming
// it is used only by PeerManager which handles concurrency control, allowing
// it to execute multiple operations atomically via its own mutex.
//
// The entire set of peers is kept in memory, for performance. It is loaded
// from disk on initialization, and any changes are written back to disk
// (without fsync, since we can afford to lose recent writes).
type peerStore struct {
	db     dbm.DB
	peers  map[NodeID]*peerInfo
	ranked []*peerInfo // cache for Ranked(), nil invalidates cache
}

// newPeerStore creates a new peer store, loading all persisted peers from the
// database into memory.
func newPeerStore(db dbm.DB) (*peerStore, error) {
	store := &peerStore{
		db: db,
	}
	if err := store.loadPeers(); err != nil {
		return nil, err
	}
	return store, nil
}

// loadPeers loads all peers from the database into memory.
func (s *peerStore) loadPeers() error {
	peers := make(map[NodeID]*peerInfo)

	start, end := keyPeerInfoRange()
	iter, err := s.db.Iterator(start, end)
	if err != nil {
		return err
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		// FIXME: We may want to tolerate failures here, by simply logging
		// the errors and ignoring the faulty peer entries.
		msg := new(p2pproto.PeerInfo)
		if err := proto.Unmarshal(iter.Value(), msg); err != nil {
			return fmt.Errorf("invalid peer Protobuf data: %w", err)
		}
		peer, err := peerInfoFromProto(msg)
		if err != nil {
			return fmt.Errorf("invalid peer data: %w", err)
		}
		peers[peer.ID] = peer
	}
	if iter.Error() != nil {
		return iter.Error()
	}
	s.peers = peers
	s.ranked = nil // invalidate cache if populated
	return nil
}

// Get fetches a peer. The boolean indicates whether the peer existed or not.
// The returned peer info is a copy, and can be mutated at will.
func (s *peerStore) Get(id NodeID) (peerInfo, bool) {
	peer, ok := s.peers[id]
	return peer.Copy(), ok
}

// Set stores peer data. The input data will be copied, and can safely be reused
// by the caller.
func (s *peerStore) Set(peer peerInfo) error {
	if err := peer.Validate(); err != nil {
		return err
	}
	peer = peer.Copy()

	// FIXME: We may want to optimize this by avoiding saving to the database
	// if there haven't been any changes to persisted fields.
	bz, err := peer.ToProto().Marshal()
	if err != nil {
		return err
	}
	if err = s.db.Set(keyPeerInfo(peer.ID), bz); err != nil {
		return err
	}

	if current, ok := s.peers[peer.ID]; !ok || current.Score() != peer.Score() {
		// If the peer is new, or its score changes, we invalidate the Ranked() cache.
		s.peers[peer.ID] = &peer
		s.ranked = nil
	} else {
		// Otherwise, since s.ranked contains pointers to the old data and we
		// want those pointers to remain valid with the new data, we have to
		// update the existing pointer address.
		*current = peer
	}

	return nil
}

// Delete deletes a peer, or does nothing if it does not exist.
func (s *peerStore) Delete(id NodeID) error {
	if _, ok := s.peers[id]; !ok {
		return nil
	}
	if err := s.db.Delete(keyPeerInfo(id)); err != nil {
		return err
	}
	delete(s.peers, id)
	s.ranked = nil
	return nil
}

// List retrieves all peers in an arbitrary order. The returned data is a copy,
// and can be mutated at will.
func (s *peerStore) List() []peerInfo {
	peers := make([]peerInfo, 0, len(s.peers))
	for _, peer := range s.peers {
		peers = append(peers, peer.Copy())
	}
	return peers
}

// Ranked returns a list of peers ordered by score (better peers first). Peers
// with equal scores are returned in an arbitrary order. The returned list must
// not be mutated or accessed concurrently by the caller, since it returns
// pointers to internal peerStore data for performance.
//
// Ranked is used to determine both which peers to dial, which ones to evict,
// and which ones to delete completely.
//
// FIXME: For now, we simply maintain a cache in s.ranked which is invalidated
// by setting it to nil, but if necessary we should use a better data structure
// for this (e.g. a heap or ordered map).
//
// FIXME: The scoring logic is currently very naïve, see peerInfo.Score().
func (s *peerStore) Ranked() []*peerInfo {
	if s.ranked != nil {
		return s.ranked
	}
	s.ranked = make([]*peerInfo, 0, len(s.peers))
	for _, peer := range s.peers {
		s.ranked = append(s.ranked, peer)
	}
	sort.Slice(s.ranked, func(i, j int) bool {
		// FIXME: If necessary, consider precomputing scores before sorting,
		// to reduce the number of Score() calls.
		return s.ranked[i].Score() > s.ranked[j].Score()
	})
	return s.ranked
}

// Size returns the number of peers in the peer store.
func (s *peerStore) Size() int {
	return len(s.peers)
}

// peerInfo contains peer information stored in a peerStore.
type peerInfo struct {
	ID            NodeID
	AddressInfo   map[string]*peerAddressInfo
	LastConnected time.Time

	// These fields are ephemeral, i.e. not persisted to the database.
	Persistent bool
	Height     int64
}

// peerInfoFromProto converts a Protobuf PeerInfo message to a peerInfo,
// erroring if the data is invalid.
func peerInfoFromProto(msg *p2pproto.PeerInfo) (*peerInfo, error) {
	p := &peerInfo{
		ID:          NodeID(msg.ID),
		AddressInfo: map[string]*peerAddressInfo{},
	}
	if msg.LastConnected != nil {
		p.LastConnected = *msg.LastConnected
	}
	for _, addr := range msg.AddressInfo {
		addressInfo, err := peerAddressInfoFromProto(addr)
		if err != nil {
			return nil, err
		}
		p.AddressInfo[addressInfo.Address.String()] = addressInfo
	}
	return p, p.Validate()
}

// ToProto converts the peerInfo to p2pproto.PeerInfo for database storage. The
// Protobuf type only contains persisted fields, while ephemeral fields are
// discarded. The returned message may contain pointers to original data, since
// it is expected to be serialized immediately.
func (p *peerInfo) ToProto() *p2pproto.PeerInfo {
	msg := &p2pproto.PeerInfo{
		ID:            string(p.ID),
		LastConnected: &p.LastConnected,
	}
	for _, addressInfo := range p.AddressInfo {
		msg.AddressInfo = append(msg.AddressInfo, addressInfo.ToProto())
	}
	if msg.LastConnected.IsZero() {
		msg.LastConnected = nil
	}
	return msg
}

// Copy returns a deep copy of the peer info.
func (p *peerInfo) Copy() peerInfo {
	if p == nil {
		return peerInfo{}
	}
	c := *p
	for i, addressInfo := range c.AddressInfo {
		addressInfoCopy := addressInfo.Copy()
		c.AddressInfo[i] = &addressInfoCopy
	}
	return c
}

// Score calculates a score for the peer. Higher-scored peers will be
// preferred over lower scores.
func (p *peerInfo) Score() PeerScore {
	var score PeerScore
	if p.Persistent {
		score += PeerScorePersistent
	}
	return score
}

// Validate validates the peer info.
func (p *peerInfo) Validate() error {
	if p.ID == "" {
		return errors.New("no peer ID")
	}
	return nil
}

// peerAddressInfo contains information and statistics about a peer address.
type peerAddressInfo struct {
	Address         PeerAddress
	LastDialSuccess time.Time
	LastDialFailure time.Time
	DialFailures    uint32 // since last successful dial
}

// peerAddressInfoFromProto converts a Protobuf PeerAddressInfo message
// to a peerAddressInfo.
func peerAddressInfoFromProto(msg *p2pproto.PeerAddressInfo) (*peerAddressInfo, error) {
	address, err := ParsePeerAddress(msg.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", address, err)
	}
	addressInfo := &peerAddressInfo{
		Address:      address,
		DialFailures: msg.DialFailures,
	}
	if msg.LastDialSuccess != nil {
		addressInfo.LastDialSuccess = *msg.LastDialSuccess
	}
	if msg.LastDialFailure != nil {
		addressInfo.LastDialFailure = *msg.LastDialFailure
	}
	return addressInfo, addressInfo.Validate()
}

// ToProto converts the address into to a Protobuf message for serialization.
func (a *peerAddressInfo) ToProto() *p2pproto.PeerAddressInfo {
	msg := &p2pproto.PeerAddressInfo{
		Address:         a.Address.String(),
		LastDialSuccess: &a.LastDialSuccess,
		LastDialFailure: &a.LastDialFailure,
		DialFailures:    a.DialFailures,
	}
	if msg.LastDialSuccess.IsZero() {
		msg.LastDialSuccess = nil
	}
	if msg.LastDialFailure.IsZero() {
		msg.LastDialFailure = nil
	}
	return msg
}

// Copy returns a copy of the address info.
func (a *peerAddressInfo) Copy() peerAddressInfo {
	return *a
}

// Validate validates the address info.
func (a *peerAddressInfo) Validate() error {
	return a.Address.Validate()
}

// These are database key prefixes.
const (
	prefixPeerInfo int64 = 1
)

// keyPeerInfo generates a peerInfo database key.
func keyPeerInfo(id NodeID) []byte {
	key, err := orderedcode.Append(nil, prefixPeerInfo, string(id))
	if err != nil {
		panic(err)
	}
	return key
}

// keyPeerInfoPrefix generates start/end keys for the entire peerInfo key range.
func keyPeerInfoRange() ([]byte, []byte) {
	start, err := orderedcode.Append(nil, prefixPeerInfo, "")
	if err != nil {
		panic(err)
	}
	end, err := orderedcode.Append(nil, prefixPeerInfo, orderedcode.Infinity)
	if err != nil {
		panic(err)
	}
	return start, end
}
