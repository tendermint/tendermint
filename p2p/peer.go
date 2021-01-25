package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/url"
	"runtime/debug"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/tendermint/tendermint/libs/cmap"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmconn "github.com/tendermint/tendermint/p2p/conn"
)

// PeerAddress is a peer address URL.
type PeerAddress struct {
	*url.URL
}

// ParsePeerAddress parses a peer address URL into a PeerAddress.
func ParsePeerAddress(address string) (PeerAddress, error) {
	u, err := url.Parse(address)
	if err != nil || u == nil {
		return PeerAddress{}, fmt.Errorf("unable to parse peer address %q: %w", address, err)
	}
	if u.Scheme == "" {
		u.Scheme = string(defaultProtocol)
	}
	pa := PeerAddress{URL: u}
	if err = pa.Validate(); err != nil {
		return PeerAddress{}, err
	}
	return pa, nil
}

// NodeID returns the address node ID.
func (a PeerAddress) NodeID() NodeID {
	return NodeID(a.User.Username())
}

// Resolve resolves a PeerAddress into a set of Endpoints, by expanding
// out a DNS name in Host to its IP addresses. Field mapping:
//
//   Scheme → Endpoint.Protocol
//   Host   → Endpoint.IP
//   User   → Endpoint.PeerID
//   Port   → Endpoint.Port
//   Path+Query+Fragment,Opaque → Endpoint.Path
//
func (a PeerAddress) Resolve(ctx context.Context) ([]Endpoint, error) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", a.Host)
	if err != nil {
		return nil, err
	}
	port, err := a.parsePort()
	if err != nil {
		return nil, err
	}

	path := a.Path
	if a.RawPath != "" {
		path = a.RawPath
	}
	if a.Opaque != "" { // used for e.g. "about:blank" style URLs
		path = a.Opaque
	}
	if a.RawQuery != "" {
		path += "?" + a.RawQuery
	}
	if a.RawFragment != "" {
		path += "#" + a.RawFragment
	}

	endpoints := make([]Endpoint, len(ips))
	for i, ip := range ips {
		endpoints[i] = Endpoint{
			PeerID:   a.NodeID(),
			Protocol: Protocol(a.Scheme),
			IP:       ip,
			Port:     port,
			Path:     path,
		}
	}
	return endpoints, nil
}

// Validates validates a PeerAddress.
func (a PeerAddress) Validate() error {
	if a.Scheme == "" {
		return errors.New("no protocol")
	}
	if id := a.User.Username(); id == "" {
		return errors.New("no peer ID")
	} else if err := NodeID(id).Validate(); err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}
	if a.Hostname() == "" && len(a.Query()) == 0 && a.Opaque == "" {
		return errors.New("no host or path given")
	}
	if port, err := a.parsePort(); err != nil {
		return err
	} else if port > 0 && a.Hostname() == "" {
		return errors.New("cannot specify port without host")
	}
	return nil
}

// parsePort returns the port number as a uint16.
func (a PeerAddress) parsePort() (uint16, error) {
	if portString := a.Port(); portString != "" {
		port64, err := strconv.ParseUint(portString, 10, 16)
		if err != nil {
			return 0, fmt.Errorf("invalid port %q: %w", portString, err)
		}
		return uint16(port64), nil
	}
	return 0, nil
}

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
// If we are connected to too many peers (more than MaxConnections), typically
// because we have upgraded to higher-scored peers and need to shed lower-scored
// ones, the flow is as follows:
// - EvictNext: returns a peer ID to evict, marking peer as evicting.
// - Disconnected: peer was disconnected, unmarking as connected and evicting,
//   and broadcasts a PeerStatusDown peer update.
//
// If all connection slots are full (at MaxConnections), we can use up to
// MaxConnectionsUpgrade additional connections to probe any higher-scored
// unconnected peers, and if we reach them (or they reach us) we allow the
// connection and evict lower-scored peers. We mark the lower-scored peer as
// upgrading[from]=to to make sure no other higher-scored peers can claim the
// same one for an upgrade. The flow is as follows:
// - Accepted: if upgrade is possible, mark upgrading[from]=to and connected.
// - DialNext: if upgrade is possible, mark upgrading[from]=to and dialing.
// - DialFailed: unmark upgrading[from]=to and dialing.
// - Dialed: unmark dialing, mark as connected.
// - EvictNext: unmark upgrading[from]=to, then if over MaxConnections
//   either the upgraded peer or an even lower-scored one (if found)
//   is marked as evicting and returned.
// - Disconnected: unmark connected and evicting, also upgrading[from]=to
//   both from and to (in case either disconnected before eviction).
type PeerManager struct {
	options     PeerManagerOptions
	wakeDialCh  chan struct{} // wakes up DialNext() on relevant peer changes
	wakeEvictCh chan struct{} // wakes up EvictNext() on relevant peer changes
	closeCh     chan struct{} // signal channel for Close()
	closeOnce   sync.Once

	mtx           sync.Mutex
	store         *peerStore
	dialing       map[NodeID]bool                   // peers being dialed (DialNext -> Dialed/DialFail)
	connected     map[NodeID]bool                   // connected peers (Dialed/Accepted -> Disconnected)
	upgrading     map[NodeID]NodeID                 // peers claimed for upgrading (key is lower-scored peer)
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

// isPersistent is a convenience function that checks if the given peer ID
// is contained in PersistentPeers. It just uses a linear search, since
// PersistentPeers is expected to be small.
func (o PeerManagerOptions) isPersistent(id NodeID) bool {
	for _, p := range o.PersistentPeers {
		if id == p {
			return true
		}
	}
	return false
}

// NewPeerManager creates a new peer manager.
func NewPeerManager(options PeerManagerOptions) *PeerManager {
	return &PeerManager{
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

		// FIXME: Once the store persists data, we need to update existing
		// peers in the store with any new information, e.g. changes to
		// PersistentPeers configuration.
		store:         newPeerStore(),
		dialing:       map[NodeID]bool{},
		connected:     map[NodeID]bool{},
		upgrading:     map[NodeID]NodeID{},
		evicting:      map[NodeID]bool{},
		subscriptions: map[*PeerUpdatesCh]*PeerUpdatesCh{},
	}
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

	peer, err := m.store.Get(address.NodeID())
	if err != nil {
		return err
	}
	if peer == nil {
		peer = &peerInfo{
			ID:         address.NodeID(),
			Persistent: m.options.isPersistent(address.NodeID()),
		}
	}
	peer.AddAddress(address)
	err = m.store.Set(peer)
	if err != nil {
		return err
	}
	m.wakeDial()
	return nil
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
	// higher score than a connected peer, and if successful evict the
	// lower-scored peer via EvictNext().
	if m.options.MaxConnected > 0 &&
		len(m.connected)+len(m.dialing) >= int(m.options.MaxConnected)+int(m.options.MaxConnectedUpgrade) {
		return "", PeerAddress{}, nil
	}

	ranked, err := m.store.Ranked()
	if err != nil {
		return "", PeerAddress{}, err
	}
	for _, peer := range ranked {
		if m.dialing[peer.ID] || m.connected[peer.ID] {
			continue
		}

		for _, addressInfo := range peer.AddressInfo {
			if time.Since(addressInfo.LastDialFailure) < m.retryDelay(peer, addressInfo.DialFailures) {
				continue
			}

			// At this point we have an eligible address to dial. If we're full
			// but have peer upgrade capacity (as checked above), we need to
			// make sure there exists an evictable peer of a lower score that we
			// can replace. If so, we mark the lower-scored peer as upgrading so
			// noone else can claim it, and EvictNext() will evict it later.
			//
			// If we don't find one, there is no point in trying additional
			// peers, since they will all have the same or lower score than this
			// peer (since they're ordered by score via peerStore.Ranked).
			if m.options.MaxConnected > 0 && len(m.connected) >= int(m.options.MaxConnected) {
				upgradePeer := m.findUpgradeCandidate(peer, ranked)
				if upgradePeer == "" {
					return "", PeerAddress{}, nil
				}
				m.upgrading[upgradePeer] = peer.ID
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
func (m *PeerManager) retryDelay(peer *peerInfo, failures uint32) time.Duration {
	if failures == 0 {
		return 0
	}
	if m.options.MinRetryTime == 0 {
		return time.Duration(math.MaxInt64)
	}
	maxDelay := m.options.MaxRetryTime
	if peer.Persistent && m.options.MaxRetryTimePersistent > 0 {
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
			// Unmark failed upgrade attempt.
			delete(m.upgrading, from)
		}
	}

	peer, err := m.store.Get(peerID)
	if err != nil || peer == nil { // Peer may have been removed while dialing, ignore.
		return err
	}
	addressInfo := peer.LookupAddressInfo(address)
	if addressInfo == nil {
		return nil // Assume the address has been removed, ignore.
	}
	addressInfo.LastDialFailure = time.Now().UTC()
	addressInfo.DialFailures++
	if err = m.store.Set(peer); err != nil {
		return err
	}

	// We spawn a goroutine that notifies DialNext() again when the retry
	// timeout has elapsed, so that we can consider dialing it again.
	//
	// FIXME: We need to calculate the retry delay outside of the goroutine,
	// since the arguments are currently pointers to structs shared in the
	// peerStore. The peerStore should probably return struct copies instead,
	// to avoid these sorts of issues.
	if retryDelay := m.retryDelay(peer, addressInfo.DialFailures); retryDelay != time.Duration(math.MaxInt64) {
		go func() {
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
	}

	m.wakeDial()
	return nil
}

// Dialed marks a peer as successfully dialed. Any further incoming connections
// will be rejected, and once disconnected the peer may be dialed again.
func (m *PeerManager) Dialed(peerID NodeID, address PeerAddress) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	delete(m.dialing, peerID)

	if m.connected[peerID] {
		return fmt.Errorf("peer %v is already connected", peerID)
	}
	if m.options.MaxConnected > 0 &&
		len(m.connected) >= int(m.options.MaxConnected)+int(m.options.MaxConnectedUpgrade) {
		return fmt.Errorf("already connected to maximum number of peers")
	}

	peer, err := m.store.Get(peerID)
	if err != nil {
		return err
	} else if peer == nil {
		return fmt.Errorf("peer %q was removed while dialing", peerID)
	}

	now := time.Now().UTC()
	peer.LastConnected = now
	if addressInfo := peer.LookupAddressInfo(address); addressInfo != nil {
		addressInfo.DialFailures = 0
		addressInfo.LastDialSuccess = now
	}
	if err = m.store.Set(peer); err != nil {
		return err
	}

	m.connected[peerID] = true
	m.wakeEvict()

	return nil
}

// Accepted marks an incoming peer connection successfully accepted. If the peer
// is already connected or we don't allow additional connections then this will
// return an error.
//
// If MaxConnectedUpgrade is non-zero, the accepted peer is better-scored than any
// other connected peer, and the number of connections does not exceed
// MaxConnected + MaxConnectedUpgrade then we accept the connection and rely on
// EvictNext() to evict lower-scored peers.
//
// NOTE: We can't take an address here, since e.g. TCP uses a different port
// number for outbound traffic than inbound traffic, so the peer's endpoint
// wouldn't necessarily be an appropriate address to dial.
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

	peer, err := m.store.Get(peerID)
	if err != nil {
		return err
	}
	if peer == nil {
		peer = &peerInfo{
			ID:         peerID,
			Persistent: m.options.isPersistent(peerID),
		}
	}

	// If we're already full (i.e. at MaxConnected), but we allow upgrades (and
	// we know from the check above that we have upgrade capacity), then we can
	// look for any lower-scored evictable peer, and if found we can accept this
	// connection anyway and let EvictNext() evict a lower-scored peer for us.
	if m.options.MaxConnected > 0 && len(m.connected) >= int(m.options.MaxConnected) {
		ranked, err := m.store.Ranked()
		if err != nil {
			return err
		}
		upgradePeer := m.findUpgradeCandidate(peer, ranked)
		if upgradePeer == "" {
			return fmt.Errorf("already connected to maximum number of peers")
		}
		m.upgrading[upgradePeer] = peerID
	}

	peer.LastConnected = time.Now().UTC()
	if err = m.store.Set(peer); err != nil {
		return err
	}

	m.connected[peerID] = true
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

	// After upgrading to a peer, it's possible for that peer to disconnect
	// before EvictNext() gets around to evicting the lower-scored peer. To
	// avoid stale upgrade markers, we remove it here.
	for from, to := range m.upgrading {
		if to == peerID {
			delete(m.upgrading, from)
		}
	}

	delete(m.connected, peerID)
	delete(m.upgrading, peerID)
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

	// We first prune the upgrade list. All connection slots were full when the
	// upgrades began, but we may have disconnected other peers in the meanwhile
	// and thus don't have to evict the upgraded peers after all.
	for from, to := range m.upgrading {
		// Stop pruning when the upgrade slots are only for connections
		// exceeding MaxConnected.
		if m.options.MaxConnected == 0 ||
			len(m.upgrading) <= len(m.connected)-len(m.evicting)-int(m.options.MaxConnected) {
			break
		}
		if m.connected[to] {
			delete(m.upgrading, from)
		}
	}

	// If we're below capacity, we don't need to evict anything.
	if m.options.MaxConnected == 0 ||
		len(m.connected)-len(m.evicting) <= int(m.options.MaxConnected) {
		return "", nil
	}

	ranked, err := m.store.Ranked()
	if err != nil {
		return "", err
	}

	// Look for any upgraded peers that we can evict.
	for from, to := range m.upgrading {
		if m.connected[to] {
			delete(m.upgrading, from)
			// We may have connected to even lower-scored peers that we can
			// evict since we started upgrading this one, in which case we can
			// evict one of those.
			fromPeer, err := m.store.Get(from)
			if err != nil {
				return "", err
			} else if fromPeer == nil {
				continue
			} else if evictPeer := m.findUpgradeCandidate(fromPeer, ranked); evictPeer != "" {
				m.evicting[evictPeer] = true
				return evictPeer, nil
			} else {
				m.evicting[from] = true
				return from, nil
			}
		}
	}

	// If we didn't find any upgraded peers to evict, we just pick a low-ranked one.
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
func (m *PeerManager) findUpgradeCandidate(peer *peerInfo, ranked []*peerInfo) NodeID {
	// Check for any existing upgrade claims to this peer. It is important that
	// we return this, since we can get an inbound connection from a peer that
	// we're concurrently trying to dial for an upgrade, and we want the inbound
	// connection to be accepted in this case.
	for from, to := range m.upgrading {
		if to == peer.ID {
			return from
		}
	}
	for i := len(ranked) - 1; i >= 0; i-- {
		candidate := ranked[i]
		switch {
		case candidate.Score() >= peer.Score():
			return "" // no further peers can be scored lower, due to sorting
		case !m.connected[candidate.ID]:
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
func (m *PeerManager) GetHeight(peerID NodeID) (int64, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	peer, err := m.store.Get(peerID)
	if err != nil || peer == nil {
		return 0, err
	}
	return peer.Height, nil
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

	peer, err := m.store.Get(peerID)
	if err != nil {
		return err
	}
	if peer == nil {
		peer = &peerInfo{
			ID:         peerID,
			Persistent: m.options.isPersistent(peerID),
		}
	}
	peer.Height = height
	return m.store.Set(peer)
}

// peerStore stores information about peers. It is currently a bare-bones
// in-memory store, and will be fleshed out later.
//
// peerStore is not thread-safe, since it assumes it is only used by PeerManager
// which handles concurrency control. This allows the manager to execute multiple
// operations atomically while it holds the mutex.
type peerStore struct {
	peers map[NodeID]peerInfo
}

// newPeerStore creates a new peer store.
func newPeerStore() *peerStore {
	return &peerStore{
		peers: map[NodeID]peerInfo{},
	}
}

// Get fetches a peer, returning nil if not found.
func (s *peerStore) Get(id NodeID) (*peerInfo, error) {
	peer, ok := s.peers[id]
	if !ok {
		return nil, nil
	}
	return &peer, nil
}

// Set stores peer data.
func (s *peerStore) Set(peer *peerInfo) error {
	if peer == nil {
		return errors.New("peer cannot be nil")
	}
	s.peers[peer.ID] = *peer
	return nil
}

// List retrieves all peers.
func (s *peerStore) List() ([]*peerInfo, error) {
	peers := []*peerInfo{}
	for _, peer := range s.peers {
		peer := peer
		peers = append(peers, &peer)
	}
	return peers, nil
}

// Ranked returns a list of peers ordered by score (better peers first).
// Peers with equal scores are returned in an arbitrary order.
//
// This is used to determine which peers to connect to and which peers to evict
// in order to make room for better peers.
//
// FIXME: For now, we simply generate the list on every call, but this can get
// expensive since it's called fairly frequently. We may want to either cache
// this, or store peers in a data structure that maintains order (e.g. a heap or
// ordered map).
func (s *peerStore) Ranked() ([]*peerInfo, error) {
	peers, err := s.List()
	if err != nil {
		return nil, err
	}
	sort.Slice(peers, func(i, j int) bool {
		// FIXME: If necessary, consider precomputing scores before sorting,
		// to reduce the number of Score() calls.
		return peers[i].Score() > peers[j].Score()
	})
	return peers, nil
}

// peerInfo contains peer information stored in a peerStore.
type peerInfo struct {
	ID            NodeID
	AddressInfo   []*addressInfo
	Persistent    bool
	Height        int64
	LastConnected time.Time
}

// AddAddress adds an address to a peer, unless it already exists. It does not
// validate the address. Returns true if the address was new.
func (p *peerInfo) AddAddress(address PeerAddress) bool {
	if p.LookupAddressInfo(address) != nil {
		return false
	}
	p.AddressInfo = append(p.AddressInfo, &addressInfo{Address: address})
	return true
}

// LookupAddressInfo returns address info for an address, or nil if unknown.
func (p *peerInfo) LookupAddressInfo(address PeerAddress) *addressInfo {
	// We just do a linear search for now.
	addressString := address.String()
	for _, info := range p.AddressInfo {
		if info.Address.String() == addressString {
			return info
		}
	}
	return nil
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

// addressInfo contains information and statistics about an address.
type addressInfo struct {
	Address         PeerAddress
	LastDialSuccess time.Time
	LastDialFailure time.Time
	DialFailures    uint32 // since last successful dial
}

// ============================================================================
// Types and business logic below may be deprecated.
//
// TODO: Rename once legacy p2p types are removed.
// ref: https://github.com/tendermint/tendermint/issues/5670
// ============================================================================

//go:generate mockery --case underscore --name Peer

const metricsTickerDuration = 10 * time.Second

// Peer is an interface representing a peer connected on a reactor.
type Peer interface {
	service.Service
	FlushStop()

	ID() NodeID           // peer's cryptographic ID
	RemoteIP() net.IP     // remote IP of the connection
	RemoteAddr() net.Addr // remote address of the connection

	IsOutbound() bool   // did we dial the peer
	IsPersistent() bool // do we redial this peer when we disconnect

	CloseConn() error // close original connection

	NodeInfo() NodeInfo // peer's info
	Status() tmconn.ConnectionStatus
	SocketAddr() *NetAddress // actual address of the socket

	Send(byte, []byte) bool
	TrySend(byte, []byte) bool

	Set(string, interface{})
	Get(string) interface{}
}

//----------------------------------------------------------

// peerConn contains the raw connection and its config.
type peerConn struct {
	outbound   bool
	persistent bool
	conn       Connection
	ip         net.IP // cached RemoteIP()
}

func newPeerConn(outbound, persistent bool, conn Connection) peerConn {
	return peerConn{
		outbound:   outbound,
		persistent: persistent,
		conn:       conn,
	}
}

// ID only exists for SecretConnection.
func (pc peerConn) ID() NodeID {
	return NodeIDFromPubKey(pc.conn.PubKey())
}

// Return the IP from the connection RemoteAddr
func (pc peerConn) RemoteIP() net.IP {
	if pc.ip == nil {
		pc.ip = pc.conn.RemoteEndpoint().IP
	}
	return pc.ip
}

// peer implements Peer.
//
// Before using a peer, you will need to perform a handshake on connection.
type peer struct {
	service.BaseService

	// raw peerConn and the multiplex connection
	peerConn

	// peer's node info and the channel it knows about
	// channels = nodeInfo.Channels
	// cached to avoid copying nodeInfo in hasChannel
	nodeInfo    NodeInfo
	channels    []byte
	reactors    map[byte]Reactor
	onPeerError func(Peer, interface{})

	// User data
	Data *cmap.CMap

	metrics       *Metrics
	metricsTicker *time.Ticker
}

type PeerOption func(*peer)

func newPeer(
	pc peerConn,
	reactorsByCh map[byte]Reactor,
	onPeerError func(Peer, interface{}),
	options ...PeerOption,
) *peer {
	nodeInfo := pc.conn.NodeInfo()
	p := &peer{
		peerConn:      pc,
		nodeInfo:      nodeInfo,
		channels:      nodeInfo.Channels, // TODO
		reactors:      reactorsByCh,
		onPeerError:   onPeerError,
		Data:          cmap.NewCMap(),
		metricsTicker: time.NewTicker(metricsTickerDuration),
		metrics:       NopMetrics(),
	}

	p.BaseService = *service.NewBaseService(nil, "Peer", p)
	for _, option := range options {
		option(p)
	}

	return p
}

// onError calls the peer error callback.
func (p *peer) onError(err interface{}) {
	p.onPeerError(p, err)
}

// String representation.
func (p *peer) String() string {
	if p.outbound {
		return fmt.Sprintf("Peer{%v %v out}", p.conn, p.ID())
	}

	return fmt.Sprintf("Peer{%v %v in}", p.conn, p.ID())
}

//---------------------------------------------------
// Implements service.Service

// SetLogger implements BaseService.
func (p *peer) SetLogger(l log.Logger) {
	p.Logger = l
}

// OnStart implements BaseService.
func (p *peer) OnStart() error {
	if err := p.BaseService.OnStart(); err != nil {
		return err
	}

	go p.processMessages()
	go p.metricsReporter()

	return nil
}

// processMessages processes messages received from the connection.
func (p *peer) processMessages() {
	defer func() {
		if r := recover(); r != nil {
			p.Logger.Error("peer message processing panic", "err", r, "stack", string(debug.Stack()))
			p.onError(fmt.Errorf("panic during peer message processing: %v", r))
		}
	}()

	for {
		chID, msg, err := p.conn.ReceiveMessage()
		if err != nil {
			p.onError(err)
			return
		}
		reactor, ok := p.reactors[chID]
		if !ok {
			p.onError(fmt.Errorf("unknown channel %v", chID))
			return
		}
		reactor.Receive(chID, p, msg)
	}
}

// FlushStop mimics OnStop but additionally ensures that all successful
// .Send() calls will get flushed before closing the connection.
// NOTE: it is not safe to call this method more than once.
func (p *peer) FlushStop() {
	p.metricsTicker.Stop()
	p.BaseService.OnStop()
	if err := p.conn.FlushClose(); err != nil {
		p.Logger.Debug("error while stopping peer", "err", err)
	}
}

// OnStop implements BaseService.
func (p *peer) OnStop() {
	p.metricsTicker.Stop()
	p.BaseService.OnStop()
	if err := p.conn.Close(); err != nil {
		p.Logger.Debug("error while stopping peer", "err", err)
	}
}

//---------------------------------------------------
// Implements Peer

// ID returns the peer's ID - the hex encoded hash of its pubkey.
func (p *peer) ID() NodeID {
	return p.nodeInfo.ID()
}

// IsOutbound returns true if the connection is outbound, false otherwise.
func (p *peer) IsOutbound() bool {
	return p.peerConn.outbound
}

// IsPersistent returns true if the peer is persitent, false otherwise.
func (p *peer) IsPersistent() bool {
	return p.peerConn.persistent
}

// NodeInfo returns a copy of the peer's NodeInfo.
func (p *peer) NodeInfo() NodeInfo {
	return p.nodeInfo
}

// SocketAddr returns the address of the socket.
// For outbound peers, it's the address dialed (after DNS resolution).
// For inbound peers, it's the address returned by the underlying connection
// (not what's reported in the peer's NodeInfo).
func (p *peer) SocketAddr() *NetAddress {
	return p.peerConn.conn.RemoteEndpoint().NetAddress()
}

// Status returns the peer's ConnectionStatus.
func (p *peer) Status() tmconn.ConnectionStatus {
	return p.conn.Status()
}

// Send msg bytes to the channel identified by chID byte. Returns false if the
// send queue is full after timeout, specified by MConnection.
func (p *peer) Send(chID byte, msgBytes []byte) bool {
	if !p.IsRunning() {
		// see Switch#Broadcast, where we fetch the list of peers and loop over
		// them - while we're looping, one peer may be removed and stopped.
		return false
	} else if !p.hasChannel(chID) {
		return false
	}
	res, err := p.conn.SendMessage(chID, msgBytes)
	if err == io.EOF {
		return false
	} else if err != nil {
		p.onError(err)
		return false
	}
	if res {
		labels := []string{
			"peer_id", string(p.ID()),
			"chID", fmt.Sprintf("%#x", chID),
		}
		p.metrics.PeerSendBytesTotal.With(labels...).Add(float64(len(msgBytes)))
	}
	return res
}

// TrySend msg bytes to the channel identified by chID byte. Immediately returns
// false if the send queue is full.
func (p *peer) TrySend(chID byte, msgBytes []byte) bool {
	if !p.IsRunning() {
		return false
	} else if !p.hasChannel(chID) {
		return false
	}
	res, err := p.conn.TrySendMessage(chID, msgBytes)
	if err == io.EOF {
		return false
	} else if err != nil {
		p.onError(err)
		return false
	}
	if res {
		labels := []string{
			"peer_id", string(p.ID()),
			"chID", fmt.Sprintf("%#x", chID),
		}
		p.metrics.PeerSendBytesTotal.With(labels...).Add(float64(len(msgBytes)))
	}
	return res
}

// Get the data for a given key.
func (p *peer) Get(key string) interface{} {
	return p.Data.Get(key)
}

// Set sets the data for the given key.
func (p *peer) Set(key string, data interface{}) {
	p.Data.Set(key, data)
}

// hasChannel returns true if the peer reported
// knowing about the given chID.
func (p *peer) hasChannel(chID byte) bool {
	for _, ch := range p.channels {
		if ch == chID {
			return true
		}
	}
	// NOTE: probably will want to remove this
	// but could be helpful while the feature is new
	p.Logger.Debug(
		"Unknown channel for peer",
		"channel",
		chID,
		"channels",
		p.channels,
	)
	return false
}

// CloseConn closes original connection. Used for cleaning up in cases where the peer had not been started at all.
func (p *peer) CloseConn() error {
	return p.peerConn.conn.Close()
}

//---------------------------------------------------
// methods only used for testing
// TODO: can we remove these?

// CloseConn closes the underlying connection
func (pc *peerConn) CloseConn() {
	pc.conn.Close()
}

// RemoteAddr returns peer's remote network address.
func (p *peer) RemoteAddr() net.Addr {
	endpoint := p.conn.RemoteEndpoint()
	return &net.TCPAddr{
		IP:   endpoint.IP,
		Port: int(endpoint.Port),
	}
}

//---------------------------------------------------

func PeerMetrics(metrics *Metrics) PeerOption {
	return func(p *peer) {
		p.metrics = metrics
	}
}

func (p *peer) metricsReporter() {
	for {
		select {
		case <-p.metricsTicker.C:
			status := p.conn.Status()
			var sendQueueSize float64
			for _, chStatus := range status.Channels {
				sendQueueSize += float64(chStatus.SendQueueSize)
			}

			p.metrics.PeerPendingSendBytes.With("peer_id", string(p.ID())).Set(sendQueueSize)
		case <-p.Quit():
			return
		}
	}
}
