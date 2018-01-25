package p2p

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"time"

	"github.com/pkg/errors"

	crypto "github.com/tendermint/go-crypto"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p/conn"
	cmn "github.com/tendermint/tmlibs/common"
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
)

//-----------------------------------------------------------------------------

type AddrBook interface {
	AddAddress(addr *NetAddress, src *NetAddress) error
	Save()
}

//-----------------------------------------------------------------------------

// `Switch` handles peer connections and exposes an API to receive incoming messages
// on `Reactors`.  Each `Reactor` is responsible for handling incoming messages of one
// or more `Channels`.  So while sending outgoing messages is typically performed on the peer,
// incoming messages are received on the reactor.
type Switch struct {
	cmn.BaseService

	config       *cfg.P2PConfig
	peerConfig   *PeerConfig
	listeners    []Listener
	reactors     map[string]Reactor
	chDescs      []*conn.ChannelDescriptor
	reactorsByCh map[byte]Reactor
	peers        *PeerSet
	dialing      *cmn.CMap
	nodeInfo     NodeInfo // our node info
	nodeKey      *NodeKey // our node privkey

	filterConnByAddr   func(net.Addr) error
	filterConnByPubKey func(crypto.PubKey) error

	rng *rand.Rand // seed for randomizing dial times and orders
}

func NewSwitch(config *cfg.P2PConfig) *Switch {
	sw := &Switch{
		config:       config,
		peerConfig:   DefaultPeerConfig(),
		reactors:     make(map[string]Reactor),
		chDescs:      make([]*conn.ChannelDescriptor, 0),
		reactorsByCh: make(map[byte]Reactor),
		peers:        NewPeerSet(),
		dialing:      cmn.NewCMap(),
	}

	// Ensure we have a completely undeterministic PRNG. cmd.RandInt64() draws
	// from a seed that's initialized with OS entropy on process start.
	sw.rng = rand.New(rand.NewSource(cmn.RandInt64()))

	// TODO: collapse the peerConfig into the config ?
	sw.peerConfig.MConfig.FlushThrottle = time.Duration(config.FlushThrottleTimeout) * time.Millisecond
	sw.peerConfig.MConfig.SendRate = config.SendRate
	sw.peerConfig.MConfig.RecvRate = config.RecvRate
	sw.peerConfig.MConfig.MaxMsgPacketPayloadSize = config.MaxMsgPacketPayloadSize

	sw.BaseService = *cmn.NewBaseService(nil, "P2P Switch", sw)
	return sw
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
			return errors.Wrapf(err, "failed to start %v", reactor)
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

// Peers returns the set of peers that are connected to the switch.
func (sw *Switch) Peers() IPeerSet {
	return sw.peers
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

// Broadcast runs a go routine for each attempted send, which will block
// trying to send for defaultSendTimeoutSeconds. Returns a channel
// which receives success values for each attempted send (false if times out).
// NOTE: Broadcast uses goroutines, so order of broadcast may not be preserved.
// TODO: Something more intelligent.
func (sw *Switch) Broadcast(chID byte, msg interface{}) chan bool {
	successChan := make(chan bool, len(sw.peers.List()))
	sw.Logger.Debug("Broadcast", "channel", chID, "msg", msg)
	for _, peer := range sw.peers.List() {
		go func(peer Peer) {
			success := peer.Send(chID, msg)
			successChan <- success
		}(peer)
	}
	return successChan
}

// StopPeerForError disconnects from a peer due to external error.
// If the peer is persistent, it will attempt to reconnect.
// TODO: make record depending on reason.
func (sw *Switch) StopPeerForError(peer Peer, reason interface{}) {
	sw.Logger.Error("Stopping peer for error", "peer", peer, "err", reason)
	sw.stopAndRemovePeer(peer, reason)

	if peer.IsPersistent() {
		go func(addr NetAddress) {
			sw.reconnectToPeer(addr)
		}(*peer.NodeInfo().NetAddress())
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
	peer.Stop()
	for _, reactor := range sw.reactors {
		reactor.RemovePeer(peer, reason)
	}
}

// reconnectToPeer tries to reconnect to the peer, first repeatedly
// with a fixed interval, then with exponential backoff.
// If no success after all that, it stops trying, and leaves it
// to the PEX/Addrbook to find the peer again
func (sw *Switch) reconnectToPeer(peerAddr NetAddress) {
	// NOTE this will connect to the self reported address,
	// not necessarily the original we dialed
	start := time.Now()
	sw.Logger.Info("Reconnecting to peer", "addr", peerAddr)
	for i := 0; i < reconnectAttempts; i++ {
		if !sw.IsRunning() {
			return
		}

		peer, err := sw.DialPeerWithAddress(&peerAddr, true)
		if err != nil {
			sw.Logger.Info("Error reconnecting to peer. Trying again", "tries", i, "err", err, "addr", peerAddr)
			// sleep a set amount
			sw.randomSleep(reconnectInterval)
			continue
		} else {
			sw.Logger.Info("Reconnected to peer", "peer", peer)
			return
		}
	}

	sw.Logger.Error("Failed to reconnect to peer. Beginning exponential backoff",
		"addr", peerAddr, "elapsed", time.Since(start))
	for i := 0; i < reconnectBackOffAttempts; i++ {
		if !sw.IsRunning() {
			return
		}

		// sleep an exponentially increasing amount
		sleepIntervalSeconds := math.Pow(reconnectBackOffBaseSeconds, float64(i))
		sw.randomSleep(time.Duration(sleepIntervalSeconds) * time.Second)
		peer, err := sw.DialPeerWithAddress(&peerAddr, true)
		if err != nil {
			sw.Logger.Info("Error reconnecting to peer. Trying again", "tries", i, "err", err, "addr", peerAddr)
			continue
		} else {
			sw.Logger.Info("Reconnected to peer", "peer", peer)
			return
		}
	}
	sw.Logger.Error("Failed to reconnect to peer. Giving up", "addr", peerAddr, "elapsed", time.Since(start))
}

//---------------------------------------------------------------------
// Dialing

// IsDialing returns true if the switch is currently dialing the given ID.
func (sw *Switch) IsDialing(id ID) bool {
	return sw.dialing.Has(string(id))
}

// DialPeersAsync dials a list of peers asynchronously in random order (optionally, making them persistent).
func (sw *Switch) DialPeersAsync(addrBook AddrBook, peers []string, persistent bool) error {
	netAddrs, errs := NewNetAddressStrings(peers)
	for _, err := range errs {
		sw.Logger.Error("Error in peer's address", "err", err)
	}

	if addrBook != nil {
		// add peers to `addrBook`
		ourAddr := sw.nodeInfo.NetAddress()
		for _, netAddr := range netAddrs {
			// do not add our address or ID
			if netAddr.Same(ourAddr) {
				continue
			}
			// TODO: move this out of here ?
			addrBook.AddAddress(netAddr, ourAddr)
		}
		// Persist some peers to disk right away.
		// NOTE: integration tests depend on this
		addrBook.Save()
	}

	// permute the list, dial them in random order.
	perm := sw.rng.Perm(len(netAddrs))
	for i := 0; i < len(perm); i++ {
		go func(i int) {
			sw.randomSleep(0)
			j := perm[i]
			peer, err := sw.DialPeerWithAddress(netAddrs[j], persistent)
			if err != nil {
				sw.Logger.Error("Error dialing peer", "err", err)
			} else {
				sw.Logger.Info("Connected to peer", "peer", peer)
			}
		}(i)
	}
	return nil
}

// DialPeerWithAddress dials the given peer and runs sw.addPeer if it connects and authenticates successfully.
// If `persistent == true`, the switch will always try to reconnect to this peer if the connection ever fails.
func (sw *Switch) DialPeerWithAddress(addr *NetAddress, persistent bool) (Peer, error) {
	sw.dialing.Set(string(addr.ID), addr)
	defer sw.dialing.Delete(string(addr.ID))
	return sw.addOutboundPeerWithConfig(addr, sw.peerConfig, persistent)
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

// FilterConnByPubKey returns an error if connecting to the given public key is forbidden.
func (sw *Switch) FilterConnByPubKey(pubkey crypto.PubKey) error {
	if sw.filterConnByPubKey != nil {
		return sw.filterConnByPubKey(pubkey)
	}
	return nil

}

// SetAddrFilter sets the function for filtering connections by address.
func (sw *Switch) SetAddrFilter(f func(net.Addr) error) {
	sw.filterConnByAddr = f
}

// SetPubKeyFilter sets the function for filtering connections by public key.
func (sw *Switch) SetPubKeyFilter(f func(crypto.PubKey) error) {
	sw.filterConnByPubKey = f
}

//------------------------------------------------------------------------------------

func (sw *Switch) listenerRoutine(l Listener) {
	for {
		inConn, ok := <-l.Connections()
		if !ok {
			break
		}

		// ignore connection if we already have enough
		maxPeers := sw.config.MaxNumPeers
		if maxPeers <= sw.peers.Size() {
			sw.Logger.Info("Ignoring inbound connection: already have enough peers", "address", inConn.RemoteAddr().String(), "numPeers", sw.peers.Size(), "max", maxPeers)
			continue
		}

		// New inbound connection!
		err := sw.addInboundPeerWithConfig(inConn, sw.peerConfig)
		if err != nil {
			sw.Logger.Info("Ignoring inbound connection: error while adding peer", "address", inConn.RemoteAddr().String(), "err", err)
			continue
		}
	}

	// cleanup
}

func (sw *Switch) addInboundPeerWithConfig(conn net.Conn, config *PeerConfig) error {
	peer, err := newInboundPeer(conn, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodeKey.PrivKey, config)
	if err != nil {
		conn.Close() // peer is nil
		return err
	}
	peer.SetLogger(sw.Logger.With("peer", conn.RemoteAddr()))
	if err = sw.addPeer(peer); err != nil {
		peer.CloseConn()
		return err
	}

	return nil
}

// dial the peer; make secret connection; authenticate against the dialed ID;
// add the peer.
func (sw *Switch) addOutboundPeerWithConfig(addr *NetAddress, config *PeerConfig, persistent bool) (Peer, error) {
	sw.Logger.Info("Dialing peer", "address", addr)
	peer, err := newOutboundPeer(addr, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodeKey.PrivKey, config, persistent)
	if err != nil {
		sw.Logger.Error("Failed to dial peer", "address", addr, "err", err)
		return nil, err
	}
	peer.SetLogger(sw.Logger.With("peer", addr))

	// authenticate peer
	if addr.ID == "" {
		peer.Logger.Info("Dialed peer with unknown ID - unable to authenticate", "addr", addr)
	} else if addr.ID != peer.ID() {
		peer.CloseConn()
		return nil, ErrSwitchAuthenticationFailure{addr, peer.ID()}
	}

	err = sw.addPeer(peer)
	if err != nil {
		sw.Logger.Error("Failed to add peer", "address", addr, "err", err)
		peer.CloseConn()
		return nil, err
	}
	sw.Logger.Info("Dialed and added peer", "address", addr, "peer", peer)
	return peer, nil
}

// addPeer performs the Tendermint P2P handshake with a peer
// that already has a SecretConnection. If all goes well,
// it starts the peer and adds it to the switch.
// NOTE: This performs a blocking handshake before the peer is added.
// NOTE: If error is returned, caller is responsible for calling peer.CloseConn()
func (sw *Switch) addPeer(peer *peer) error {
	// Avoid self
	if sw.nodeKey.ID() == peer.ID() {
		return ErrSwitchConnectToSelf
	}

	// Avoid duplicate
	if sw.peers.Has(peer.ID()) {
		return ErrSwitchDuplicatePeer

	}

	// Filter peer against white list
	if err := sw.FilterConnByAddr(peer.Addr()); err != nil {
		return err
	}
	if err := sw.FilterConnByPubKey(peer.PubKey()); err != nil {
		return err
	}

	// Exchange NodeInfo with the peer
	if err := peer.HandshakeTimeout(sw.nodeInfo, time.Duration(sw.peerConfig.HandshakeTimeout*time.Second)); err != nil {
		return err
	}

	// Validate the peers nodeInfo against the pubkey
	if err := peer.NodeInfo().Validate(peer.PubKey()); err != nil {
		return err
	}

	// Check version, chain id
	if err := sw.nodeInfo.CompatibleWith(peer.NodeInfo()); err != nil {
		return err
	}

	// All good. Start peer
	if sw.IsRunning() {
		sw.startInitPeer(peer)
	}

	// Add the peer to .peers.
	// We start it first so that a peer in the list is safe to Stop.
	// It should not err since we already checked peers.Has().
	if err := sw.peers.Add(peer); err != nil {
		return err
	}

	sw.Logger.Info("Added peer", "peer", peer)
	return nil
}

func (sw *Switch) startInitPeer(peer *peer) {
	err := peer.Start() // spawn send/recv routines
	if err != nil {
		// Should never happen
		sw.Logger.Error("Error starting peer", "peer", peer, "err", err)
	}

	for _, reactor := range sw.reactors {
		reactor.AddPeer(peer)
	}
}
