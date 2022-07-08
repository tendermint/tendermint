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

	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	p2pproto "github.com/tendermint/tendermint/proto/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

const (
	// retryNever is returned by retryDelay() when retries are disabled.
	retryNever time.Duration = math.MaxInt64
)

// PeerStatus is a peer status.
//
// The peer manager has many more internal states for a peer (e.g. dialing,
// connected, evicting, and so on), which are tracked separately. PeerStatus is
// for external use outside of the peer manager.
type PeerStatus string

const (
	PeerStatusUp   PeerStatus = "up"   // connected and ready
	PeerStatusDown PeerStatus = "down" // disconnected
	PeerStatusGood PeerStatus = "good" // peer observed as good
	PeerStatusBad  PeerStatus = "bad"  // peer observed as bad
)

type peerConnectionDirection int

const (
	peerConnectionIncoming peerConnectionDirection = iota + 1
	peerConnectionOutgoing
)

// PeerScore is a numeric score assigned to a peer (higher is better).
type PeerScore int16

const (
	PeerScorePersistent       PeerScore = math.MaxInt16 // persistent peers
	MaxPeerScoreNotPersistent PeerScore = PeerScorePersistent - 1
)

// PeerUpdate is a peer update event sent via PeerUpdates.
type PeerUpdate struct {
	NodeID   types.NodeID
	Status   PeerStatus
	Channels ChannelIDSet
}

// PeerUpdates is a peer update subscription with notifications about peer
// events (currently just status changes).
type PeerUpdates struct {
	routerUpdatesCh  chan PeerUpdate
	reactorUpdatesCh chan PeerUpdate
}

// NewPeerUpdates creates a new PeerUpdates subscription. It is primarily for
// internal use, callers should typically use PeerManager.Subscribe(). The
// subscriber must call Close() when done.
func NewPeerUpdates(updatesCh chan PeerUpdate, buf int) *PeerUpdates {
	return &PeerUpdates{
		reactorUpdatesCh: updatesCh,
		routerUpdatesCh:  make(chan PeerUpdate, buf),
	}
}

// Updates returns a channel for consuming peer updates.
func (pu *PeerUpdates) Updates() <-chan PeerUpdate {
	return pu.reactorUpdatesCh
}

// SendUpdate pushes information about a peer into the routing layer,
// presumably from a peer.
func (pu *PeerUpdates) SendUpdate(ctx context.Context, update PeerUpdate) {
	select {
	case <-ctx.Done():
	case pu.routerUpdatesCh <- update:
	}
}

// PeerManagerOptions specifies options for a PeerManager.
type PeerManagerOptions struct {
	// PersistentPeers are peers that we want to maintain persistent connections
	// to. These will be scored higher than other peers, and if
	// MaxConnectedUpgrade is non-zero any lower-scored peers will be evicted if
	// necessary to make room for these.
	PersistentPeers []types.NodeID

	// MaxPeers is the maximum number of peers to track information about, i.e.
	// store in the peer store. When exceeded, the lowest-scored unconnected peers
	// will be deleted. 0 means no limit.
	MaxPeers uint16

	// MaxConnected is the maximum number of connected peers (inbound and
	// outbound). 0 means no limit.
	MaxConnected uint16

	// MaxOutgoingConnections specifies how many outgoing
	// connections a node will maintain. It must be lower than MaxConnected. If it is
	// 0, then all connections can be outgoing. Once this limit is
	// reached, the node will not dial peers, allowing the
	// remaining peer connections to be used by incoming connections.
	MaxOutgoingConnections uint16

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
	// retry times, to avoid thundering herds. 0 disables jitter.
	RetryTimeJitter time.Duration

	// DisconnectCooldownPeriod is the amount of time after we
	// disconnect from a peer before we'll consider dialing a new peer
	DisconnectCooldownPeriod time.Duration

	// PeerScores sets fixed scores for specific peers. It is mainly used
	// for testing. A score of 0 is ignored.
	PeerScores map[types.NodeID]PeerScore

	// PrivatePeerIDs defines a set of NodeID objects which the PEX reactor will
	// consider private and never gossip.
	PrivatePeers map[types.NodeID]struct{}

	// SelfAddress is the address that will be advertised to peers for them to dial back to us.
	// If Hostname and Port are unset, Advertise() will include no self-announcement
	SelfAddress NodeAddress

	// persistentPeers provides fast PersistentPeers lookups. It is built
	// by optimize().
	persistentPeers map[types.NodeID]bool

	// Peer Metrics
	Metrics *Metrics
}

// Validate validates the options.
func (o *PeerManagerOptions) Validate() error {
	for _, id := range o.PersistentPeers {
		if err := id.Validate(); err != nil {
			return fmt.Errorf("invalid PersistentPeer ID %q: %w", id, err)
		}
	}

	for id := range o.PrivatePeers {
		if err := id.Validate(); err != nil {
			return fmt.Errorf("invalid private peer ID %q: %w", id, err)
		}
	}

	if o.MaxConnected > 0 && len(o.PersistentPeers) > int(o.MaxConnected) {
		return fmt.Errorf("number of persistent peers %v can't exceed MaxConnected %v",
			len(o.PersistentPeers), o.MaxConnected)
	}

	if o.MaxPeers > 0 {
		if o.MaxConnected == 0 || o.MaxConnected+o.MaxConnectedUpgrade > o.MaxPeers {
			return fmt.Errorf("MaxConnected %v and MaxConnectedUpgrade %v can't exceed MaxPeers %v",
				o.MaxConnected, o.MaxConnectedUpgrade, o.MaxPeers)
		}
	}

	if o.MaxRetryTime > 0 {
		if o.MinRetryTime == 0 {
			return errors.New("can't set MaxRetryTime without MinRetryTime")
		}
		if o.MinRetryTime > o.MaxRetryTime {
			return fmt.Errorf("MinRetryTime %v is greater than MaxRetryTime %v",
				o.MinRetryTime, o.MaxRetryTime)
		}
	}

	if o.MaxRetryTimePersistent > 0 {
		if o.MinRetryTime == 0 {
			return errors.New("can't set MaxRetryTimePersistent without MinRetryTime")
		}
		if o.MinRetryTime > o.MaxRetryTimePersistent {
			return fmt.Errorf("MinRetryTime %v is greater than MaxRetryTimePersistent %v",
				o.MinRetryTime, o.MaxRetryTimePersistent)
		}
	}

	if o.MaxOutgoingConnections > 0 && o.MaxConnected < o.MaxOutgoingConnections {
		return errors.New("cannot set MaxOutgoingConnections to a value larger than MaxConnected")
	}

	return nil
}

// isPersistent checks if a peer is in PersistentPeers. It will panic
// if called before optimize().
func (o *PeerManagerOptions) isPersistent(id types.NodeID) bool {
	if o.persistentPeers == nil {
		panic("isPersistent() called before optimize()")
	}
	return o.persistentPeers[id]
}

// optimize optimizes operations by pregenerating lookup structures. It's a
// separate method instead of memoizing during calls to avoid dealing with
// concurrency and mutex overhead.
func (o *PeerManagerOptions) optimize() {
	o.persistentPeers = make(map[types.NodeID]bool, len(o.PersistentPeers))
	for _, p := range o.PersistentPeers {
		o.persistentPeers[p] = true
	}
}

// PeerManager manages peer lifecycle information, using a peerStore for
// underlying storage. Its primary purpose is to determine which peer to connect
// to next (including retry timers), make sure a peer only has a single active
// connection (either inbound or outbound), and evict peers to make room for
// higher-scored peers. It does not manage actual connections (this is handled
// by the Router), only the peer lifecycle state.
//
// For an outbound connection, the flow is as follows:
// - DialNext: return a peer address to dial, mark peer as dialing.
// - DialFailed: report a dial failure, unmark as dialing.
// - Dialed: report a dial success, unmark as dialing and mark as connected
//   (errors if already connected, e.g. by Accepted).
// - Ready: report routing is ready, mark as ready and broadcast PeerStatusUp.
// - Disconnected: report peer disconnect, unmark as connected and broadcasts
//   PeerStatusDown.
//
// For an inbound connection, the flow is as follows:
// - Accepted: report inbound connection success, mark as connected (errors if
//   already connected, e.g. by Dialed).
// - Ready: report routing is ready, mark as ready and broadcast PeerStatusUp.
// - Disconnected: report peer disconnect, unmark as connected and broadcasts
//   PeerStatusDown.
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
type PeerManager struct {
	selfID     types.NodeID
	options    PeerManagerOptions
	metrics    *Metrics
	rand       *rand.Rand
	dialWaker  *tmsync.Waker // wakes up DialNext() on relevant peer changes
	evictWaker *tmsync.Waker // wakes up EvictNext() on relevant peer changes

	mtx           sync.Mutex
	store         *peerStore
	subscriptions map[*PeerUpdates]*PeerUpdates            // keyed by struct identity (address)
	dialing       map[types.NodeID]bool                    // peers being dialed (DialNext → Dialed/DialFail)
	upgrading     map[types.NodeID]types.NodeID            // peers claimed for upgrade (DialNext → Dialed/DialFail)
	connected     map[types.NodeID]peerConnectionDirection // connected peers (Dialed/Accepted → Disconnected)
	ready         map[types.NodeID]bool                    // ready peers (Ready → Disconnected)
	evict         map[types.NodeID]bool                    // peers scheduled for eviction (Connected → EvictNext)
	evicting      map[types.NodeID]bool                    // peers being evicted (EvictNext → Disconnected)
}

// NewPeerManager creates a new peer manager.
func NewPeerManager(selfID types.NodeID, peerDB dbm.DB, options PeerManagerOptions) (*PeerManager, error) {
	if selfID == "" {
		return nil, errors.New("self ID not given")
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}

	options.optimize()

	store, err := newPeerStore(peerDB)
	if err != nil {
		return nil, err
	}

	peerManager := &PeerManager{
		selfID:     selfID,
		options:    options,
		rand:       rand.New(rand.NewSource(time.Now().UnixNano())), // nolint:gosec
		dialWaker:  tmsync.NewWaker(),
		evictWaker: tmsync.NewWaker(),
		metrics:    NopMetrics(),

		store:         store,
		dialing:       map[types.NodeID]bool{},
		upgrading:     map[types.NodeID]types.NodeID{},
		connected:     map[types.NodeID]peerConnectionDirection{},
		ready:         map[types.NodeID]bool{},
		evict:         map[types.NodeID]bool{},
		evicting:      map[types.NodeID]bool{},
		subscriptions: map[*PeerUpdates]*PeerUpdates{},
	}

	if options.Metrics != nil {
		peerManager.metrics = options.Metrics
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
// configuration, e.g. PersistentPeers. It also removes ourself, if we're in the
// peer store. The caller must hold the mutex lock.
func (m *PeerManager) configurePeers() error {
	if err := m.store.Delete(m.selfID); err != nil {
		return err
	}

	configure := map[types.NodeID]bool{}
	for _, id := range m.options.PersistentPeers {
		configure[id] = true
	}
	for id := range m.options.PeerScores {
		configure[id] = true
	}
	for id := range configure {
		if peer, ok := m.store.Get(id); ok {
			if err := m.store.Set(m.configurePeer(peer)); err != nil {
				return err
			}
		}
	}

	m.metrics.PeersStored.Add(float64(m.store.Size()))

	return nil
}

// configurePeer configures a peer with ephemeral runtime configuration.
func (m *PeerManager) configurePeer(peer peerInfo) peerInfo {
	peer.Persistent = m.options.isPersistent(peer.ID)
	peer.FixedScore = m.options.PeerScores[peer.ID]
	return peer
}

// newPeerInfo creates a peerInfo for a new peer.
func (m *PeerManager) newPeerInfo(id types.NodeID) peerInfo {
	peerInfo := peerInfo{
		ID:          id,
		AddressInfo: map[NodeAddress]*peerAddressInfo{},
	}
	return m.configurePeer(peerInfo)
}

// prunePeers removes low-scored peers from the peer store if it contains more
// than MaxPeers peers. The caller must hold the mutex lock.
func (m *PeerManager) prunePeers() error {
	if m.options.MaxPeers == 0 || m.store.Size() <= int(m.options.MaxPeers) {
		return nil
	}

	ranked := m.store.Ranked()
	for i := len(ranked) - 1; i >= 0; i-- {
		peerID := ranked[i].ID

		switch {
		case m.store.Size() <= int(m.options.MaxPeers):
			return nil
		case m.dialing[peerID]:
		case m.isConnected(peerID):
		default:
			if err := m.store.Delete(peerID); err != nil {
				return err
			}
			m.metrics.PeersStored.Add(-1)
		}
	}
	return nil
}

func (m *PeerManager) isConnected(peerID types.NodeID) bool {
	_, ok := m.connected[peerID]
	return ok
}

type connectionStats struct {
	incoming uint16
	outgoing uint16
}

func (m *PeerManager) getConnectedInfo() connectionStats {
	out := connectionStats{}
	for _, direction := range m.connected {
		switch direction {
		case peerConnectionIncoming:
			out.incoming++
		case peerConnectionOutgoing:
			out.outgoing++
		}
	}
	return out
}

// Add adds a peer to the manager, given as an address. If the peer already
// exists, the address is added to it if it isn't already present. This will push
// low scoring peers out of the address book if it exceeds the maximum size.
func (m *PeerManager) Add(address NodeAddress) (bool, error) {
	if err := address.Validate(); err != nil {
		return false, err
	}
	if address.NodeID == m.selfID {
		return false, fmt.Errorf("can't add self (%v) to peer store", m.selfID)
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	peer, ok := m.store.Get(address.NodeID)
	if !ok {
		peer = m.newPeerInfo(address.NodeID)
	}
	_, ok = peer.AddressInfo[address]
	// if we already have the peer address, there's no need to continue
	if ok {
		return false, nil
	}
	if peer.Inactive {
		return false, nil
	}

	// else add the new address
	peer.AddressInfo[address] = &peerAddressInfo{Address: address}
	if err := m.store.Set(peer); err != nil {
		return false, err
	}

	m.metrics.PeersStored.Add(1)
	if err := m.prunePeers(); err != nil {
		return true, err
	}
	m.dialWaker.Wake()
	return true, nil
}

// PeerRatio returns the ratio of peer addresses stored to the maximum size.
func (m *PeerManager) PeerRatio() float64 {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.options.MaxPeers == 0 {
		return 0
	}

	return float64(m.store.Size()) / float64(m.options.MaxPeers)
}

func (m *PeerManager) HasMaxPeerCapacity() bool {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	return len(m.connected) >= int(m.options.MaxConnected)
}

func (m *PeerManager) HasDialedMaxPeers() bool {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	stats := m.getConnectedInfo()

	return stats.outgoing >= m.options.MaxOutgoingConnections
}

// DialNext finds an appropriate peer address to dial, and marks it as dialing.
// If no peer is found, or all connection slots are full, it blocks until one
// becomes available. The caller must call Dialed() or DialFailed() for the
// returned peer.
func (m *PeerManager) DialNext(ctx context.Context) (NodeAddress, error) {
	for {
		if address := m.TryDialNext(); (address != NodeAddress{}) {
			return address, nil
		}

		select {
		case <-m.dialWaker.Sleep():
			continue
		case <-ctx.Done():
			return NodeAddress{}, ctx.Err()
		}
	}
}

// TryDialNext is equivalent to DialNext(), but immediately returns an empty
// address if no peers or connection slots are available.
func (m *PeerManager) TryDialNext() NodeAddress {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// We allow dialing MaxConnected+MaxConnectedUpgrade peers. Including
	// MaxConnectedUpgrade allows us to probe additional peers that have a
	// higher score than any other peers, and if successful evict it.
	if m.options.MaxConnected > 0 && len(m.connected)+len(m.dialing) >= int(m.options.MaxConnected)+int(m.options.MaxConnectedUpgrade) {
		return NodeAddress{}
	}

	cinfo := m.getConnectedInfo()
	if m.options.MaxOutgoingConnections > 0 && cinfo.outgoing >= m.options.MaxOutgoingConnections {
		return NodeAddress{}
	}

	for _, peer := range m.store.Ranked() {
		if m.dialing[peer.ID] || m.isConnected(peer.ID) {
			continue
		}

		if !peer.LastDisconnected.IsZero() && time.Since(peer.LastDisconnected) < m.options.DisconnectCooldownPeriod {
			continue
		}

		for _, addressInfo := range peer.AddressInfo {
			if time.Since(addressInfo.LastDialFailure) < m.retryDelay(addressInfo.DialFailures, peer.Persistent) {
				continue
			}

			if id, ok := m.store.Resolve(addressInfo.Address); ok && (m.isConnected(id) || m.dialing[id]) {
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
					return NodeAddress{}
				}
				m.upgrading[upgradeFromPeer] = peer.ID
			}

			m.dialing[peer.ID] = true
			return addressInfo.Address
		}
	}
	return NodeAddress{}
}

// DialFailed reports a failed dial attempt. This will make the peer available
// for dialing again when appropriate (possibly after a retry timeout).
func (m *PeerManager) DialFailed(ctx context.Context, address NodeAddress) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.metrics.PeersConnectedFailure.Add(1)

	delete(m.dialing, address.NodeID)
	for from, to := range m.upgrading {
		if to == address.NodeID {
			delete(m.upgrading, from) // Unmark failed upgrade attempt.
		}
	}

	peer, ok := m.store.Get(address.NodeID)
	if !ok { // Peer may have been removed while dialing, ignore.
		return nil
	}
	addressInfo, ok := peer.AddressInfo[address]
	if !ok {
		return nil // Assume the address has been removed, ignore.
	}

	addressInfo.LastDialFailure = time.Now().UTC()
	addressInfo.DialFailures++

	if err := m.store.Set(peer); err != nil {
		return err
	}

	// We spawn a goroutine that notifies DialNext() again when the retry
	// timeout has elapsed, so that we can consider dialing it again. We
	// calculate the retry delay outside the goroutine, since it must hold
	// the mutex lock.
	if d := m.retryDelay(addressInfo.DialFailures, peer.Persistent); d != 0 && d != retryNever {
		go func() {
			// Use an explicit timer with deferred cleanup instead of
			// time.After(), to avoid leaking goroutines on PeerManager.Close().
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-timer.C:
				m.dialWaker.Wake()
			case <-ctx.Done():
			}
		}()
	} else {
		m.dialWaker.Wake()
	}

	return nil
}

// Dialed marks a peer as successfully dialed. Any further connections will be
// rejected, and once disconnected the peer may be dialed again.
func (m *PeerManager) Dialed(address NodeAddress) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.metrics.PeersConnectedSuccess.Add(1)

	delete(m.dialing, address.NodeID)

	var upgradeFromPeer types.NodeID
	for from, to := range m.upgrading {
		if to == address.NodeID {
			delete(m.upgrading, from)
			upgradeFromPeer = from
			// Don't break, just in case this peer was marked as upgrading for
			// multiple lower-scored peers (shouldn't really happen).
		}
	}
	if address.NodeID == m.selfID {
		return fmt.Errorf("rejecting connection to self (%v)", address.NodeID)
	}
	if m.isConnected(address.NodeID) {
		return fmt.Errorf("peer %v is already connected", address.NodeID)
	}
	if m.options.MaxConnected > 0 && len(m.connected) >= int(m.options.MaxConnected) {
		if upgradeFromPeer == "" || len(m.connected) >= int(m.options.MaxConnected)+int(m.options.MaxConnectedUpgrade) {
			return fmt.Errorf("already connected to maximum number of peers")
		}
	}

	peer, ok := m.store.Get(address.NodeID)
	if !ok {
		return fmt.Errorf("peer %q was removed while dialing", address.NodeID)
	}
	now := time.Now().UTC()
	if peer.Inactive {
		m.metrics.PeersInactivated.Add(-1)
	}
	peer.Inactive = false

	peer.LastConnected = now
	if addressInfo, ok := peer.AddressInfo[address]; ok {
		addressInfo.DialFailures = 0
		addressInfo.LastDialSuccess = now
		// If not found, assume address has been removed.
	}

	if err := m.store.Set(peer); err != nil {
		return err
	}

	if upgradeFromPeer != "" && m.options.MaxConnected > 0 && len(m.connected) >= int(m.options.MaxConnected) {
		// Look for an even lower-scored peer that may have appeared since we
		// started the upgrade.
		if p, ok := m.store.Get(upgradeFromPeer); ok {
			if u := m.findUpgradeCandidate(p.ID, p.Score()); u != "" {
				upgradeFromPeer = u
			}
		}
		m.evict[upgradeFromPeer] = true
		m.evictWaker.Wake()
	}

	m.metrics.PeersConnectedOutgoing.Add(1)
	m.connected[peer.ID] = peerConnectionOutgoing

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
func (m *PeerManager) Accepted(peerID types.NodeID) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if peerID == m.selfID {
		return fmt.Errorf("rejecting connection from self (%v)", peerID)
	}
	if m.isConnected(peerID) {
		return fmt.Errorf("peer %q is already connected", peerID)
	}
	if m.options.MaxConnected > 0 && len(m.connected) >= int(m.options.MaxConnected)+int(m.options.MaxConnectedUpgrade) {
		return fmt.Errorf("already connected to maximum number of peers")
	}

	peer, ok := m.store.Get(peerID)
	if !ok {
		peer = m.newPeerInfo(peerID)
	}

	// reset this to avoid penalizing peers for their past transgressions
	for _, addr := range peer.AddressInfo {
		addr.DialFailures = 0
	}

	// If all connections slots are full, but we allow upgrades (and we checked
	// above that we have upgrade capacity), then we can look for a lower-scored
	// peer to replace and if found accept the connection anyway and evict it.
	var upgradeFromPeer types.NodeID
	if m.options.MaxConnected > 0 && len(m.connected) >= int(m.options.MaxConnected) {
		upgradeFromPeer = m.findUpgradeCandidate(peer.ID, peer.Score())
		if upgradeFromPeer == "" {
			return fmt.Errorf("already connected to maximum number of peers")
		}
	}

	if peer.Inactive {
		m.metrics.PeersInactivated.Add(-1)
	}
	peer.Inactive = false
	peer.LastConnected = time.Now().UTC()
	if err := m.store.Set(peer); err != nil {
		return err
	}

	m.metrics.PeersConnectedIncoming.Add(1)
	m.connected[peerID] = peerConnectionIncoming
	if upgradeFromPeer != "" {
		m.evict[upgradeFromPeer] = true
	}
	m.evictWaker.Wake()
	return nil
}

// Ready marks a peer as ready, broadcasting status updates to
// subscribers. The peer must already be marked as connected. This is
// separate from Dialed() and Accepted() to allow the router to set up
// its internal queues before reactors start sending messages. The
// channels set here are passed in the peer update broadcast to
// reactors, which can then mediate their own behavior based on the
// capability of the peers.
func (m *PeerManager) Ready(ctx context.Context, peerID types.NodeID, channels ChannelIDSet) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.isConnected(peerID) {
		m.ready[peerID] = true
		m.broadcast(ctx, PeerUpdate{
			NodeID:   peerID,
			Status:   PeerStatusUp,
			Channels: channels,
		})
	}
}

// EvictNext returns the next peer to evict (i.e. disconnect). If no evictable
// peers are found, the call will block until one becomes available.
func (m *PeerManager) EvictNext(ctx context.Context) (types.NodeID, error) {
	for {
		id, err := m.TryEvictNext()
		if err != nil || id != "" {
			return id, err
		}
		select {
		case <-m.evictWaker.Sleep():
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// TryEvictNext is equivalent to EvictNext, but immediately returns an empty
// node ID if no evictable peers are found.
func (m *PeerManager) TryEvictNext() (types.NodeID, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// If any connected peers are explicitly scheduled for eviction, we return a
	// random one.
	for peerID := range m.evict {
		delete(m.evict, peerID)
		if m.isConnected(peerID) && !m.evicting[peerID] {
			m.evicting[peerID] = true
			return peerID, nil
		}
	}

	// If we're below capacity, we don't need to evict anything.
	if m.options.MaxConnected == 0 ||
		len(m.connected)-len(m.evicting) <= int(m.options.MaxConnected) {
		return "", nil
	}

	// If we're above capacity (shouldn't really happen), just pick the
	// lowest-ranked peer to evict.
	ranked := m.store.Ranked()
	for i := len(ranked) - 1; i >= 0; i-- {
		peer := ranked[i]
		if m.isConnected(peer.ID) && !m.evicting[peer.ID] {
			m.evicting[peer.ID] = true
			return peer.ID, nil
		}
	}

	return "", nil
}

// Disconnected unmarks a peer as connected, allowing it to be dialed or
// accepted again as appropriate.
func (m *PeerManager) Disconnected(ctx context.Context, peerID types.NodeID) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	switch m.connected[peerID] {
	case peerConnectionIncoming:
		m.metrics.PeersConnectedIncoming.Add(-1)
	case peerConnectionOutgoing:
		m.metrics.PeersConnectedOutgoing.Add(-1)
	}

	ready := m.ready[peerID]

	delete(m.connected, peerID)
	delete(m.upgrading, peerID)
	delete(m.evict, peerID)
	delete(m.evicting, peerID)
	delete(m.ready, peerID)

	if peer, ok := m.store.Get(peerID); ok {
		peer.LastDisconnected = time.Now()
		_ = m.store.Set(peer)
		// launch a thread to ping the dialWaker when the
		// disconnected peer can be dialed again.
		go func() {
			timer := time.NewTimer(m.options.DisconnectCooldownPeriod)
			defer timer.Stop()
			select {
			case <-timer.C:
				m.dialWaker.Wake()
			case <-ctx.Done():
			}
		}()
	}

	if ready {
		m.broadcast(ctx, PeerUpdate{
			NodeID: peerID,
			Status: PeerStatusDown,
		})
	}

	m.dialWaker.Wake()
}

// Errored reports a peer error, causing the peer to be evicted if it's
// currently connected.
//
// FIXME: This should probably be replaced with a peer behavior API, see
// PeerError comments for more details.
//
// FIXME: This will cause the peer manager to immediately try to reconnect to
// the peer, which is probably not always what we want.
func (m *PeerManager) Errored(peerID types.NodeID, err error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.isConnected(peerID) {
		m.evict[peerID] = true
	}

	m.evictWaker.Wake()
}

// Inactivate marks a peer as inactive which means we won't attempt to
// dial this peer again. A peer can be reactivated by successfully
// dialing and connecting to the node.
func (m *PeerManager) Inactivate(peerID types.NodeID) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	peer, ok := m.store.peers[peerID]
	if !ok {
		return nil
	}

	peer.Inactive = true
	m.metrics.PeersInactivated.Add(1)
	return m.store.Set(*peer)
}

// Advertise returns a list of peer addresses to advertise to a peer.
//
// It sorts all peers in the peer store, and assembles a list of peers
// that is most likely to include the highest priority of peers.
func (m *PeerManager) Advertise(peerID types.NodeID, limit uint16) []NodeAddress {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	addresses := make([]NodeAddress, 0, limit)

	// advertise ourselves, to let everyone know how to dial us back
	// and enable mutual address discovery
	if m.options.SelfAddress.Hostname != "" && m.options.SelfAddress.Port != 0 {
		addresses = append(addresses, m.options.SelfAddress)
	}

	var numAddresses int
	var totalAbsScore int
	ranked := m.store.Ranked()
	seenAddresses := map[NodeAddress]struct{}{}
	scores := map[types.NodeID]int{}

	// get the total number of possible addresses
	for _, peer := range ranked {
		if peer.ID == peerID {
			continue
		}
		score := int(peer.Score())
		if score < 0 {
			totalAbsScore += -score
		} else {
			totalAbsScore += score
		}

		scores[peer.ID] = score
		for addr := range peer.AddressInfo {
			if _, ok := m.options.PrivatePeers[addr.NodeID]; !ok {
				numAddresses++
			}
		}
	}

	meanAbsScore := (totalAbsScore + 1) / (len(scores) + 1)

	var attempts uint16
	var addedLastIteration bool

	// if the number of addresses is less than the number of peers
	// to advertise, adjust the limit downwards
	if numAddresses < int(limit) {
		limit = uint16(numAddresses)
	}

	// collect addresses until we have the number requested
	// (limit), or we've added all known addresses, or we've tried
	// at least 256 times and the last time we iterated over
	// remaining addresses we added no new candidates.
	for len(addresses) < int(limit) && (attempts < (limit*2) || !addedLastIteration) {
		attempts++
		addedLastIteration = false

		for idx, peer := range ranked {
			if peer.ID == peerID {
				continue
			}

			if len(addresses) >= int(limit) {
				break
			}

			for nodeAddr, addressInfo := range peer.AddressInfo {
				if len(addresses) >= int(limit) {
					break
				}

				// only look at each address once, by
				// tracking a set of addresses seen
				if _, ok := seenAddresses[addressInfo.Address]; ok {
					continue
				}

				// only add non-private NodeIDs
				if _, ok := m.options.PrivatePeers[nodeAddr.NodeID]; !ok {
					// add the peer if the total number of ranked addresses is
					// will fit within the limit, or otherwise adding
					// addresses based on a coin flip.

					// the coinflip is based on the score, commonly, but
					// 10% of the time we'll randomly insert a "loosing"
					// peer.

					// nolint:gosec // G404: Use of weak random number generator
					if numAddresses <= int(limit) || rand.Intn((meanAbsScore*2)+1) <= scores[peer.ID]+1 || rand.Intn((idx+1)*10) <= idx+1 {
						addresses = append(addresses, addressInfo.Address)
						addedLastIteration = true
						seenAddresses[addressInfo.Address] = struct{}{}
					}
				} else {
					seenAddresses[addressInfo.Address] = struct{}{}
					// if the number of addresses
					// is the same as the limit,
					// we should remove private
					// addresses from the limit so
					// we can still return early.
					if numAddresses == int(limit) {
						limit--
					}
				}
			}
		}
	}

	return addresses
}

// PeerEventSubscriber describes the type of the subscription method, to assist
// in isolating reactors specific construction and lifecycle from the
// peer manager.
type PeerEventSubscriber func(context.Context) *PeerUpdates

// Subscribe subscribes to peer updates. The caller must consume the peer
// updates in a timely fashion and close the subscription when done, otherwise
// the PeerManager will halt.
func (m *PeerManager) Subscribe(ctx context.Context) *PeerUpdates {
	// FIXME: We use a size 1 buffer here. When we broadcast a peer update
	// we have to loop over all of the subscriptions, and we want to avoid
	// having to block and wait for a context switch before continuing on
	// to the next subscriptions. This also prevents tail latencies from
	// compounding. Limiting it to 1 means that the subscribers are still
	// reasonably in sync. However, this should probably be benchmarked.
	peerUpdates := NewPeerUpdates(make(chan PeerUpdate, 1), 1)
	m.Register(ctx, peerUpdates)
	return peerUpdates
}

// Register allows you to inject a custom PeerUpdate instance into the
// PeerManager, rather than relying on the instance constructed by the
// Subscribe method, which wraps the functionality of the Register
// method.
//
// The caller must consume the peer updates from this PeerUpdates
// instance in a timely fashion and close the subscription when done,
// otherwise the PeerManager will halt.
func (m *PeerManager) Register(ctx context.Context, peerUpdates *PeerUpdates) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.subscriptions[peerUpdates] = peerUpdates

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case pu := <-peerUpdates.routerUpdatesCh:
				m.processPeerEvent(ctx, pu)
			}
		}
	}()

	go func() {
		<-ctx.Done()
		m.mtx.Lock()
		defer m.mtx.Unlock()
		delete(m.subscriptions, peerUpdates)
	}()
}

func (m *PeerManager) processPeerEvent(ctx context.Context, pu PeerUpdate) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if ctx.Err() != nil {
		return
	}

	if _, ok := m.store.peers[pu.NodeID]; !ok {
		m.store.peers[pu.NodeID] = &peerInfo{}
	}

	switch pu.Status {
	case PeerStatusBad:
		if m.store.peers[pu.NodeID].MutableScore == math.MinInt16 {
			return
		}
		m.store.peers[pu.NodeID].MutableScore--
	case PeerStatusGood:
		if m.store.peers[pu.NodeID].MutableScore == math.MaxInt16 {
			return
		}
		m.store.peers[pu.NodeID].MutableScore++
	}
}

// broadcast broadcasts a peer update to all subscriptions. The caller must
// already hold the mutex lock, to make sure updates are sent in the same order
// as the PeerManager processes them, but this means subscribers must be
// responsive at all times or the entire PeerManager will halt.
//
// FIXME: Consider using an internal channel to buffer updates while also
// maintaining order if this is a problem.
func (m *PeerManager) broadcast(ctx context.Context, peerUpdate PeerUpdate) {
	for _, sub := range m.subscriptions {
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case sub.reactorUpdatesCh <- peerUpdate:
		}
	}
}

// Addresses returns all known addresses for a peer, primarily for testing.
// The order is arbitrary.
func (m *PeerManager) Addresses(peerID types.NodeID) []NodeAddress {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	addresses := []NodeAddress{}
	if peer, ok := m.store.Get(peerID); ok {
		for _, addressInfo := range peer.AddressInfo {
			addresses = append(addresses, addressInfo.Address)
		}
	}
	return addresses
}

// Peers returns all known peers, primarily for testing. The order is arbitrary.
func (m *PeerManager) Peers() []types.NodeID {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	peers := []types.NodeID{}
	for _, peer := range m.store.Ranked() {
		peers = append(peers, peer.ID)
	}
	return peers
}

// Scores returns the peer scores for all known peers, primarily for testing.
func (m *PeerManager) Scores() map[types.NodeID]PeerScore {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	scores := map[types.NodeID]PeerScore{}
	for _, peer := range m.store.Ranked() {
		scores[peer.ID] = peer.Score()
	}
	return scores
}

// Status returns the status for a peer, primarily for testing.
func (m *PeerManager) Status(id types.NodeID) PeerStatus {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	switch {
	case m.ready[id]:
		return PeerStatusUp
	default:
		return PeerStatusDown
	}
}

// findUpgradeCandidate looks for a lower-scored peer that we could evict
// to make room for the given peer. Returns an empty ID if none is found.
// If the peer is already being upgraded to, we return that same upgrade.
// The caller must hold the mutex lock.
func (m *PeerManager) findUpgradeCandidate(id types.NodeID, score PeerScore) types.NodeID {
	for from, to := range m.upgrading {
		if to == id {
			return from
		}
	}

	ranked := m.store.Ranked()
	for i := len(ranked) - 1; i >= 0; i-- {
		candidate := ranked[i]
		switch {
		case candidate.ID == id:
			continue
		case candidate.Score() >= score:
			return "" // no further peers can be scored lower, due to sorting
		case !m.isConnected(candidate.ID):
		case m.evict[candidate.ID]:
		case m.evicting[candidate.ID]:
		case m.upgrading[candidate.ID] != "":
		default:
			return candidate.ID
		}
	}
	return ""
}

// retryDelay calculates a dial retry delay using exponential backoff, based on
// retry settings in PeerManagerOptions. If retries are disabled (i.e.
// MinRetryTime is 0), this returns retryNever (i.e. an infinite retry delay).
// The caller must hold the mutex lock (for m.rand which is not thread-safe).
func (m *PeerManager) retryDelay(failures uint32, persistent bool) time.Duration {
	if failures == 0 {
		return 0
	}
	if m.options.MinRetryTime == 0 {
		return retryNever
	}
	maxDelay := m.options.MaxRetryTime
	if persistent && m.options.MaxRetryTimePersistent > 0 {
		maxDelay = m.options.MaxRetryTimePersistent
	}

	delay := m.options.MinRetryTime * time.Duration(failures)
	if m.options.RetryTimeJitter > 0 {
		delay += time.Duration(m.rand.Int63n(int64(m.options.RetryTimeJitter)))
	}

	if maxDelay > 0 && delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// peerStore stores information about peers. It is not thread-safe, assuming it
// is only used by PeerManager which handles concurrency control. This allows
// the manager to execute multiple operations atomically via its own mutex.
//
// The entire set of peers is kept in memory, for performance. It is loaded
// from disk on initialization, and any changes are written back to disk
// (without fsync, since we can afford to lose recent writes).
type peerStore struct {
	db     dbm.DB
	peers  map[types.NodeID]*peerInfo
	index  map[NodeAddress]types.NodeID
	ranked []*peerInfo // cache for Ranked(), nil invalidates cache
}

// newPeerStore creates a new peer store, loading all persisted peers from the
// database into memory.
func newPeerStore(db dbm.DB) (*peerStore, error) {
	if db == nil {
		return nil, errors.New("no database provided")
	}
	store := &peerStore{db: db}
	if err := store.loadPeers(); err != nil {
		return nil, err
	}
	return store, nil
}

// loadPeers loads all peers from the database into memory.
func (s *peerStore) loadPeers() error {
	peers := map[types.NodeID]*peerInfo{}
	addrs := map[NodeAddress]types.NodeID{}

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
		for addr := range peer.AddressInfo {
			// TODO maybe check to see if we've seen this
			// addr before for a different peer, there
			// could be duplicates.
			addrs[addr] = peer.ID
		}
	}
	if iter.Error() != nil {
		return iter.Error()
	}
	s.peers = peers
	s.index = addrs
	s.ranked = nil // invalidate cache if populated
	return nil
}

// Get fetches a peer. The boolean indicates whether the peer existed or not.
// The returned peer info is a copy, and can be mutated at will.
func (s *peerStore) Get(id types.NodeID) (peerInfo, bool) {
	peer, ok := s.peers[id]
	return peer.Copy(), ok
}

// Resolve returns the peer ID for a given node address if known.
func (s *peerStore) Resolve(addr NodeAddress) (types.NodeID, bool) {
	id, ok := s.index[addr]
	return id, ok
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
	for addr := range peer.AddressInfo {
		s.index[addr] = peer.ID
	}

	return nil
}

// Delete deletes a peer, or does nothing if it does not exist.
func (s *peerStore) Delete(id types.NodeID) error {
	peer, ok := s.peers[id]
	if !ok {
		return nil
	}
	for _, addr := range peer.AddressInfo {
		delete(s.index, addr.Address)
	}
	delete(s.peers, id)
	s.ranked = nil

	if err := s.db.Delete(keyPeerInfo(id)); err != nil {
		return err
	}

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
	ID               types.NodeID
	AddressInfo      map[NodeAddress]*peerAddressInfo
	LastConnected    time.Time
	LastDisconnected time.Time

	// These fields are ephemeral, i.e. not persisted to the database.
	Persistent bool
	Height     int64
	FixedScore PeerScore // mainly for tests

	MutableScore int64 // updated by router
	Inactive     bool
}

// peerInfoFromProto converts a Protobuf PeerInfo message to a peerInfo,
// erroring if the data is invalid.
func peerInfoFromProto(msg *p2pproto.PeerInfo) (*peerInfo, error) {
	p := &peerInfo{
		ID:          types.NodeID(msg.ID),
		AddressInfo: map[NodeAddress]*peerAddressInfo{},
		Inactive:    msg.Inactive,
	}
	if msg.LastConnected != nil {
		p.LastConnected = *msg.LastConnected
	}
	for _, a := range msg.AddressInfo {
		addressInfo, err := peerAddressInfoFromProto(a)
		if err != nil {
			return nil, err
		}
		p.AddressInfo[addressInfo.Address] = addressInfo

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
		Inactive:      p.Inactive,
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

// LastDialed returns when the peer was last dialed, and if that dial
// attempt was successful. If the peer was never dialed the time stamp
// is zero time.
func (p *peerInfo) LastDialed() (time.Time, bool) {
	var (
		last    time.Time
		success bool
	)
	last = last.Add(-1) // so it's after the epoch

	for _, addr := range p.AddressInfo {
		if addr.LastDialFailure.Equal(addr.LastDialSuccess) {
			if addr.LastDialFailure.IsZero() {
				continue
			}
			if last.After(addr.LastDialSuccess) {
				continue
			}
			success = true
			last = addr.LastDialSuccess
		}
		if addr.LastDialFailure.After(last) {
			success = false
			last = addr.LastDialFailure
		}
		if addr.LastDialSuccess.After(last) || last.Equal(addr.LastDialSuccess) {
			success = true
			last = addr.LastDialSuccess
		}
	}

	// if we never modified last, then we should return it to the
	// zero value
	if last.Add(1).IsZero() {
		return time.Time{}, success
	}

	return last, success
}

// Score calculates a score for the peer. Higher-scored peers will be
// preferred over lower scores.
func (p *peerInfo) Score() PeerScore {
	if p.FixedScore > 0 {
		return p.FixedScore
	}
	if p.Persistent {
		return PeerScorePersistent
	}

	score := p.MutableScore
	if score > int64(MaxPeerScoreNotPersistent) {
		score = int64(MaxPeerScoreNotPersistent)
	}

	for _, addr := range p.AddressInfo {
		// DialFailures is reset when dials succeed, so this
		// is either the number of dial failures or 0.
		score -= int64(addr.DialFailures)
	}

	if score < math.MinInt16 {
		score = math.MinInt16
	}

	return PeerScore(score)
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
	Address         NodeAddress
	LastDialSuccess time.Time
	LastDialFailure time.Time
	DialFailures    uint32 // since last successful dial
}

// peerAddressInfoFromProto converts a Protobuf PeerAddressInfo message
// to a peerAddressInfo.
func peerAddressInfoFromProto(msg *p2pproto.PeerAddressInfo) (*peerAddressInfo, error) {
	address, err := ParseNodeAddress(msg.Address)
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

// Database key prefixes.
const (
	prefixPeerInfo int64 = 1
)

// keyPeerInfo generates a peerInfo database key.
func keyPeerInfo(id types.NodeID) []byte {
	key, err := orderedcode.Append(nil, prefixPeerInfo, string(id))
	if err != nil {
		panic(err)
	}
	return key
}

// keyPeerInfoRange generates start/end keys for the entire peerInfo key range.
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
