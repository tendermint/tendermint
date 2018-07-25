package p2p

import (
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"github.com/tendermint/tendermint/config"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/p2p/conn"
)

const (
	// wait a random amount of time from this interval
	// before dialing peers or reconnecting to help prevent DoS
	dialRandomizerIntervalMilliseconds = 3000

	// repeatedly try to reconnect for a few minutes
	// ie. 5 * 20 = 100s
	reconnectAttempts = 20
	reconnectInterval = 5 * time.Second

	// then move into exponential backoff mode for ~1day
	// ie. 3**10 = 16hrs
	reconnectBackOffAttempts    = 10
	reconnectBackOffBaseSeconds = 3

	// keep at least this many outbound peers
	// TODO: move to config
	DefaultMinNumOutboundPeers = 10
)

//-----------------------------------------------------------------------------

// An AddrBook represents an address book from the pex package, which is used
// to store peer addresses.
type AddrBook interface {
	AddAddress(addr *NetAddress, src *NetAddress) error
	AddOurAddress(*NetAddress)
	OurAddress(*NetAddress) bool
	MarkGood(*NetAddress)
	RemoveAddress(*NetAddress)
	HasAddress(*NetAddress) bool
	Save()
}

//-----------------------------------------------------------------------------

// Switch handles peer connections and exposes an API to receive incoming messages
// on `Reactors`.  Each `Reactor` is responsible for handling incoming messages of one
// or more `Channels`.  So while sending outgoing messages is typically performed on the peer,
// incoming messages are received on the reactor.
type Switch struct {
	cmn.BaseService

	config       *config.P2PConfig
	listeners    []Listener
	reactors     map[string]Reactor
	chDescs      []*conn.ChannelDescriptor
	reactorsByCh map[byte]Reactor
	peers        *PeerSet
	dialing      *cmn.CMap
	reconnecting *cmn.CMap
	nodeInfo     NodeInfo // our node info
	nodeKey      *NodeKey // our node privkey
	addrBook     AddrBook

	filterConnByAddr func(net.Addr) error
	filterConnByID   func(ID) error

	mConfig conn.MConnConfig

	rng *cmn.Rand // seed for randomizing dial times and orders

	metrics *Metrics
}

// SwitchOption sets an optional parameter on the Switch.
type SwitchOption func(*Switch)

// NewSwitch creates a new Switch with the given config.
func NewSwitch(cfg *config.P2PConfig, options ...SwitchOption) *Switch {
	sw := &Switch{
		config:       cfg,
		reactors:     make(map[string]Reactor),
		chDescs:      make([]*conn.ChannelDescriptor, 0),
		reactorsByCh: make(map[byte]Reactor),
		peers:        NewPeerSet(),
		dialing:      cmn.NewCMap(),
		reconnecting: cmn.NewCMap(),
		metrics:      NopMetrics(),
	}

	// Ensure we have a completely undeterministic PRNG.
	sw.rng = cmn.NewRand()

	mConfig := conn.DefaultMConnConfig()
	mConfig.FlushThrottle = time.Duration(cfg.FlushThrottleTimeout) * time.Millisecond
	mConfig.SendRate = cfg.SendRate
	mConfig.RecvRate = cfg.RecvRate
	mConfig.MaxPacketMsgPayloadSize = cfg.MaxPacketMsgPayloadSize

	sw.mConfig = mConfig

	sw.BaseService = *cmn.NewBaseService(nil, "P2P Switch", sw)

	for _, option := range options {
		option(sw)
	}

	return sw
}

// WithMetrics sets the metrics.
func WithMetrics(metrics *Metrics) SwitchOption {
	return func(sw *Switch) { sw.metrics = metrics }
}

//---------------------------------------------------------------------
// Switch setup

// AddReactor adds the given reactor to the switch.
// NOTE: Not goroutine safe.
func (sw *Switch) AddReactor(name string, reactor Reactor) Reactor {
	// Validate the reactor.
	// No two reactors can share the same channel.
	reactorChannels := reactor.GetChannels()
	for _, chDesc := range reactorChannels {
		chID := chDesc.ID
		if sw.reactorsByCh[chID] != nil {
			cmn.PanicSanity(fmt.Sprintf("Channel %X has multiple reactors %v & %v", chID, sw.reactorsByCh[chID], reactor))
		}
		sw.chDescs = append(sw.chDescs, chDesc)
		sw.reactorsByCh[chID] = reactor
	}
	sw.reactors[name] = reactor
	reactor.SetSwitch(sw)
	return reactor
}

// Reactors returns a map of reactors registered on the switch.
// NOTE: Not goroutine safe.
func (sw *Switch) Reactors() map[string]Reactor {
	return sw.reactors
}

// Reactor returns the reactor with the given name.
// NOTE: Not goroutine safe.
func (sw *Switch) Reactor(name string) Reactor {
	return sw.reactors[name]
}

// AddListener adds the given listener to the switch for listening to incoming peer connections.
// NOTE: Not goroutine safe.
func (sw *Switch) AddListener(l Listener) {
	sw.listeners = append(sw.listeners, l)
}

// Listeners returns the list of listeners the switch listens on.
// NOTE: Not goroutine safe.
func (sw *Switch) Listeners() []Listener {
	return sw.listeners
}

// IsListening returns true if the switch has at least one listener.
// NOTE: Not goroutine safe.
func (sw *Switch) IsListening() bool {
	return len(sw.listeners) > 0
}

// SetNodeInfo sets the switch's NodeInfo for checking compatibility and handshaking with other nodes.
// NOTE: Not goroutine safe.
func (sw *Switch) SetNodeInfo(nodeInfo NodeInfo) {
	sw.nodeInfo = nodeInfo
}

// NodeInfo returns the switch's NodeInfo.
// NOTE: Not goroutine safe.
func (sw *Switch) NodeInfo() NodeInfo {
	return sw.nodeInfo
}

// SetNodeKey sets the switch's private key for authenticated encryption.
// NOTE: Not goroutine safe.
func (sw *Switch) SetNodeKey(nodeKey *NodeKey) {
	sw.nodeKey = nodeKey
}

//---------------------------------------------------------------------
// Service start/stop

// OnStart implements BaseService. It starts all the reactors, peers, and listeners.
func (sw *Switch) OnStart() error {
	// Start reactors
	for _, reactor := range sw.reactors {
		err := reactor.Start()
		if err != nil {
			return cmn.ErrorWrap(err, "failed to start %v", reactor)
		}
	}
	// Start listeners
	for _, listener := range sw.listeners {
		go sw.listenerRoutine(listener)
	}
	return nil
}

// OnStop implements BaseService. It stops all listeners, peers, and reactors.
func (sw *Switch) OnStop() {
	// Stop listeners
	for _, listener := range sw.listeners {
		listener.Stop()
	}
	sw.listeners = nil
	// Stop peers
	for _, peer := range sw.peers.List() {
		peer.Stop()
		sw.peers.Remove(peer)
	}
	// Stop reactors
	sw.Logger.Debug("Switch: Stopping reactors")
	for _, reactor := range sw.reactors {
		reactor.Stop()
	}
}

//---------------------------------------------------------------------
// Peers

// Broadcast runs a go routine for each attempted send, which will block trying
// to send for defaultSendTimeoutSeconds. Returns a channel which receives
// success values for each attempted send (false if times out). Channel will be
// closed once msg bytes are sent to all peers (or time out).
//
// NOTE: Broadcast uses goroutines, so order of broadcast may not be preserved.
func (sw *Switch) Broadcast(chID byte, msgBytes []byte) chan bool {
	successChan := make(chan bool, len(sw.peers.List()))
	sw.Logger.Debug("Broadcast", "channel", chID, "msgBytes", fmt.Sprintf("%X", msgBytes))
	var wg sync.WaitGroup
	for _, peer := range sw.peers.List() {
		wg.Add(1)
		go func(peer Peer) {
			defer wg.Done()
			success := peer.Send(chID, msgBytes)
			successChan <- success
		}(peer)
	}
	go func() {
		wg.Wait()
		close(successChan)
	}()
	return successChan
}

// NumPeers returns the count of outbound/inbound and outbound-dialing peers.
func (sw *Switch) NumPeers() (outbound, inbound, dialing int) {
	peers := sw.peers.List()
	for _, peer := range peers {
		if peer.IsOutbound() {
			outbound++
		} else {
			inbound++
		}
	}
	dialing = sw.dialing.Size()
	return
}

// Peers returns the set of peers that are connected to the switch.
func (sw *Switch) Peers() IPeerSet {
	return sw.peers
}

// StopPeerForError disconnects from a peer due to external error.
// If the peer is persistent, it will attempt to reconnect.
// TODO: make record depending on reason.
func (sw *Switch) StopPeerForError(peer Peer, reason interface{}) {
	sw.Logger.Error("Stopping peer for error", "peer", peer, "err", reason)
	sw.stopAndRemovePeer(peer, reason)

	if peer.IsPersistent() {
		addr := peer.OriginalAddr()
		if addr == nil {
			// FIXME: persistent peers can't be inbound right now.
			// self-reported address for inbound persistent peers
			addr = peer.NodeInfo().NetAddress()
		}
		go sw.reconnectToPeer(addr)
	}
}

// StopPeerGracefully disconnects from a peer gracefully.
// TODO: handle graceful disconnects.
func (sw *Switch) StopPeerGracefully(peer Peer) {
	sw.Logger.Info("Stopping peer gracefully")
	sw.stopAndRemovePeer(peer, nil)
}

func (sw *Switch) stopAndRemovePeer(peer Peer, reason interface{}) {
	sw.peers.Remove(peer)
	sw.metrics.Peers.Add(float64(-1))
	peer.Stop()
	for _, reactor := range sw.reactors {
		reactor.RemovePeer(peer, reason)
	}
}

// reconnectToPeer tries to reconnect to the addr, first repeatedly
// with a fixed interval, then with exponential backoff.
// If no success after all that, it stops trying, and leaves it
// to the PEX/Addrbook to find the peer with the addr again
// NOTE: this will keep trying even if the handshake or auth fails.
// TODO: be more explicit with error types so we only retry on certain failures
//  - ie. if we're getting ErrDuplicatePeer we can stop
//  	because the addrbook got us the peer back already
func (sw *Switch) reconnectToPeer(addr *NetAddress) {
	if sw.reconnecting.Has(string(addr.ID)) {
		return
	}
	sw.reconnecting.Set(string(addr.ID), addr)
	defer sw.reconnecting.Delete(string(addr.ID))

	start := time.Now()
	sw.Logger.Info("Reconnecting to peer", "addr", addr)
	for i := 0; i < reconnectAttempts; i++ {
		if !sw.IsRunning() {
			return
		}

		err := sw.DialPeerWithAddress(addr, true)
		if err == nil {
			return // success
		}

		sw.Logger.Info("Error reconnecting to peer. Trying again", "tries", i, "err", err, "addr", addr)
		// sleep a set amount
		sw.randomSleep(reconnectInterval)
		continue
	}

	sw.Logger.Error("Failed to reconnect to peer. Beginning exponential backoff",
		"addr", addr, "elapsed", time.Since(start))
	for i := 0; i < reconnectBackOffAttempts; i++ {
		if !sw.IsRunning() {
			return
		}

		// sleep an exponentially increasing amount
		sleepIntervalSeconds := math.Pow(reconnectBackOffBaseSeconds, float64(i))
		sw.randomSleep(time.Duration(sleepIntervalSeconds) * time.Second)
		err := sw.DialPeerWithAddress(addr, true)
		if err == nil {
			return // success
		}
		sw.Logger.Info("Error reconnecting to peer. Trying again", "tries", i, "err", err, "addr", addr)
	}
	sw.Logger.Error("Failed to reconnect to peer. Giving up", "addr", addr, "elapsed", time.Since(start))
}

// SetAddrBook allows to set address book on Switch.
func (sw *Switch) SetAddrBook(addrBook AddrBook) {
	sw.addrBook = addrBook
}

// MarkPeerAsGood marks the given peer as good when it did something useful
// like contributed to consensus.
func (sw *Switch) MarkPeerAsGood(peer Peer) {
	if sw.addrBook != nil {
		sw.addrBook.MarkGood(peer.NodeInfo().NetAddress())
	}
}

//---------------------------------------------------------------------
// Dialing

// IsDialing returns true if the switch is currently dialing the given ID.
func (sw *Switch) IsDialing(id ID) bool {
	return sw.dialing.Has(string(id))
}

// DialPeersAsync dials a list of peers asynchronously in random order (optionally, making them persistent).
// Used to dial peers from config on startup or from unsafe-RPC (trusted sources).
// TODO: remove addrBook arg since it's now set on the switch
func (sw *Switch) DialPeersAsync(addrBook AddrBook, peers []string, persistent bool) error {
	netAddrs, errs := NewNetAddressStrings(peers)
	// only log errors, dial correct addresses
	for _, err := range errs {
		sw.Logger.Error("Error in peer's address", "err", err)
	}

	ourAddr := sw.nodeInfo.NetAddress()

	// TODO: this code feels like it's in the wrong place.
	// The integration tests depend on the addrBook being saved
	// right away but maybe we can change that. Recall that
	// the addrBook is only written to disk every 2min
	if addrBook != nil {
		// add peers to `addrBook`
		for _, netAddr := range netAddrs {
			// do not add our address or ID
			if !netAddr.Same(ourAddr) {
				if err := addrBook.AddAddress(netAddr, ourAddr); err != nil {
					sw.Logger.Error("Can't add peer's address to addrbook", "err", err)
				}
			}
		}
		// Persist some peers to disk right away.
		// NOTE: integration tests depend on this
		addrBook.Save()
	}

	// permute the list, dial them in random order.
	perm := sw.rng.Perm(len(netAddrs))
	for i := 0; i < len(perm); i++ {
		go func(i int) {
			j := perm[i]

			addr := netAddrs[j]
			// do not dial ourselves
			if addr.Same(ourAddr) {
				return
			}

			sw.randomSleep(0)
			err := sw.DialPeerWithAddress(addr, persistent)
			if err != nil {
				switch err.(type) {
				case ErrSwitchConnectToSelf, ErrSwitchDuplicatePeerID:
					sw.Logger.Debug("Error dialing peer", "err", err)
				default:
					sw.Logger.Error("Error dialing peer", "err", err)
				}
			}
		}(i)
	}
	return nil
}

// DialPeerWithAddress dials the given peer and runs sw.addPeer if it connects and authenticates successfully.
// If `persistent == true`, the switch will always try to reconnect to this peer if the connection ever fails.
func (sw *Switch) DialPeerWithAddress(addr *NetAddress, persistent bool) error {
	sw.dialing.Set(string(addr.ID), addr)
	defer sw.dialing.Delete(string(addr.ID))
	return sw.addOutboundPeerWithConfig(addr, sw.config, persistent)
}

// sleep for interval plus some random amount of ms on [0, dialRandomizerIntervalMilliseconds]
func (sw *Switch) randomSleep(interval time.Duration) {
	r := time.Duration(sw.rng.Int63n(dialRandomizerIntervalMilliseconds)) * time.Millisecond
	time.Sleep(r + interval)
}

//------------------------------------------------------------------------------------
// Connection filtering

// FilterConnByAddr returns an error if connecting to the given address is forbidden.
func (sw *Switch) FilterConnByAddr(addr net.Addr) error {
	if sw.filterConnByAddr != nil {
		return sw.filterConnByAddr(addr)
	}
	return nil
}

// FilterConnByID returns an error if connecting to the given peer ID is forbidden.
func (sw *Switch) FilterConnByID(id ID) error {
	if sw.filterConnByID != nil {
		return sw.filterConnByID(id)
	}
	return nil

}

// SetAddrFilter sets the function for filtering connections by address.
func (sw *Switch) SetAddrFilter(f func(net.Addr) error) {
	sw.filterConnByAddr = f
}

// SetIDFilter sets the function for filtering connections by peer ID.
func (sw *Switch) SetIDFilter(f func(ID) error) {
	sw.filterConnByID = f
}

//------------------------------------------------------------------------------------

func (sw *Switch) listenerRoutine(l Listener) {
	for {
		inConn, ok := <-l.Connections()
		if !ok {
			break
		}

		// ignore connection if we already have enough
		// leave room for MinNumOutboundPeers
		maxPeers := sw.config.MaxNumPeers - DefaultMinNumOutboundPeers
		if maxPeers <= sw.peers.Size() {
			sw.Logger.Info("Ignoring inbound connection: already have enough peers", "address", inConn.RemoteAddr().String(), "numPeers", sw.peers.Size(), "max", maxPeers)
			inConn.Close()
			continue
		}

		// New inbound connection!
		err := sw.addInboundPeerWithConfig(inConn, sw.config)
		if err != nil {
			sw.Logger.Info("Ignoring inbound connection: error while adding peer", "address", inConn.RemoteAddr().String(), "err", err)
			continue
		}
	}

	// cleanup
}

// closes conn if err is returned
func (sw *Switch) addInboundPeerWithConfig(
	conn net.Conn,
	config *config.P2PConfig,
) error {
	peerConn, err := newInboundPeerConn(conn, config, sw.nodeKey.PrivKey)
	if err != nil {
		conn.Close() // peer is nil
		return err
	}
	if err = sw.addPeer(peerConn); err != nil {
		peerConn.CloseConn()
		return err
	}

	return nil
}

// dial the peer; make secret connection; authenticate against the dialed ID;
// add the peer.
// if dialing fails, start the reconnect loop. If handhsake fails, its over.
// If peer is started succesffuly, reconnectLoop will start when
// StopPeerForError is called
func (sw *Switch) addOutboundPeerWithConfig(
	addr *NetAddress,
	config *config.P2PConfig,
	persistent bool,
) error {
	sw.Logger.Info("Dialing peer", "address", addr)
	peerConn, err := newOutboundPeerConn(
		addr,
		config,
		persistent,
		sw.nodeKey.PrivKey,
	)
	if err != nil {
		if persistent {
			go sw.reconnectToPeer(addr)
		}
		return err
	}

	if err := sw.addPeer(peerConn); err != nil {
		peerConn.CloseConn()
		return err
	}
	return nil
}

// addPeer performs the Tendermint P2P handshake with a peer
// that already has a SecretConnection. If all goes well,
// it starts the peer and adds it to the switch.
// NOTE: This performs a blocking handshake before the peer is added.
// NOTE: If error is returned, caller is responsible for calling
// peer.CloseConn()
func (sw *Switch) addPeer(pc peerConn) error {

	addr := pc.conn.RemoteAddr()
	if err := sw.FilterConnByAddr(addr); err != nil {
		return err
	}

	// Exchange NodeInfo on the conn
	peerNodeInfo, err := pc.HandshakeTimeout(sw.nodeInfo, time.Duration(sw.config.HandshakeTimeout))
	if err != nil {
		return err
	}

	peerID := peerNodeInfo.ID

	// ensure connection key matches self reported key
	connID := pc.ID()

	if peerID != connID {
		return fmt.Errorf(
			"nodeInfo.ID() (%v) doesn't match conn.ID() (%v)",
			peerID,
			connID,
		)
	}

	// Validate the peers nodeInfo
	if err := peerNodeInfo.Validate(); err != nil {
		return err
	}

	// Avoid self
	if sw.nodeKey.ID() == peerID {
		addr := peerNodeInfo.NetAddress()
		// remove the given address from the address book
		// and add to our addresses to avoid dialing again
		sw.addrBook.RemoveAddress(addr)
		sw.addrBook.AddOurAddress(addr)
		return ErrSwitchConnectToSelf{addr}
	}

	// Avoid duplicate
	if sw.peers.Has(peerID) {
		return ErrSwitchDuplicatePeerID{peerID}
	}

	// Check for duplicate connection or peer info IP.
	if !sw.config.AllowDuplicateIP &&
		(sw.peers.HasIP(pc.RemoteIP()) ||
			sw.peers.HasIP(peerNodeInfo.NetAddress().IP)) {
		return ErrSwitchDuplicatePeerIP{pc.RemoteIP()}
	}

	// Filter peer against ID white list
	if err := sw.FilterConnByID(peerID); err != nil {
		return err
	}

	// Check version, chain id
	if err := sw.nodeInfo.CompatibleWith(peerNodeInfo); err != nil {
		return err
	}

	peer := newPeer(pc, sw.mConfig, peerNodeInfo, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError)
	peer.SetLogger(sw.Logger.With("peer", addr))

	peer.Logger.Info("Successful handshake with peer", "peerNodeInfo", peerNodeInfo)

	// All good. Start peer
	if sw.IsRunning() {
		if err = sw.startInitPeer(peer); err != nil {
			return err
		}
	}

	// Add the peer to .peers.
	// We start it first so that a peer in the list is safe to Stop.
	// It should not err since we already checked peers.Has().
	if err := sw.peers.Add(peer); err != nil {
		return err
	}
	sw.metrics.Peers.Add(float64(1))

	sw.Logger.Info("Added peer", "peer", peer)
	return nil
}

func (sw *Switch) startInitPeer(peer *peer) error {
	err := peer.Start() // spawn send/recv routines
	if err != nil {
		// Should never happen
		sw.Logger.Error("Error starting peer", "peer", peer, "err", err)
		return err
	}

	for _, reactor := range sw.reactors {
		reactor.AddPeer(peer)
	}

	return nil
}
