package p2p

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/pkg/errors"

	crypto "github.com/libp2p/go-libp2p-crypto"
	inet "github.com/libp2p/go-libp2p-net"
	lpeer "github.com/libp2p/go-libp2p-peer"
	ps "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
	wdata "github.com/tendermint/go-wire/data"
	cfg "github.com/tendermint/tendermint/config"
	cmn "github.com/tendermint/tmlibs/common"
)

const (
	// wait a random amount of time from this interval
	// before dialing seeds or reconnecting to help prevent DoS
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

type Reactor interface {
	cmn.Service // Start, Stop

	SetSwitch(*Switch)
	GetChannels() []*ChannelDescriptor
	AddPeer(peer Peer)
	RemovePeer(peer Peer, reason interface{})
	Receive(chID byte, peer Peer, msgBytes []byte) // CONTRACT: msgBytes are not nil
}

//--------------------------------------

type BaseReactor struct {
	cmn.BaseService // Provides Start, Stop, .Quit
	Switch          *Switch
}

func NewBaseReactor(name string, impl Reactor) *BaseReactor {
	return &BaseReactor{
		BaseService: *cmn.NewBaseService(nil, name, impl),
		Switch:      nil,
	}
}

func (br *BaseReactor) SetSwitch(sw *Switch) {
	br.Switch = sw
}
func (_ *BaseReactor) GetChannels() []*ChannelDescriptor             { return nil }
func (_ *BaseReactor) AddPeer(peer Peer)                             {}
func (_ *BaseReactor) RemovePeer(peer Peer, reason interface{})      {}
func (_ *BaseReactor) Receive(chID byte, peer Peer, msgBytes []byte) {}

//-----------------------------------------------------------------------------

/*
The `Switch` handles peer connections and exposes an API to receive incoming messages
on `Reactors`.  Each `Reactor` is responsible for handling incoming messages of one
or more `Channels`.  So while sending outgoing messages is typically performed on the peer,
incoming messages are received on the reactor.
*/
type Switch struct {
	cmn.BaseService

	config          *cfg.P2PConfig
	peerConfig      *PeerConfig
	defaultListener *DefaultListener
	listeners       []Listener
	reactors        map[string]Reactor
	chDescs         []*ChannelDescriptor
	reactorsByCh    map[byte]Reactor
	peers           *PeerSet
	dialing         *cmn.CMap
	nodeInfo        *NodeInfo      // our node info
	nodePrivKey     crypto.PrivKey // our node privkey
	peerStore       ps.Peerstore

	filterConnByAddr   func(ma.Multiaddr) error
	filterConnByPubKey func(crypto.PubKey) error

	rng *rand.Rand // seed for randomizing dial times and orders
}

var (
	ErrSwitchDuplicatePeer = errors.New("Duplicate peer")
)

func NewSwitch(config *cfg.P2PConfig) *Switch {
	sw := &Switch{
		config:       config,
		peerConfig:   DefaultPeerConfig(),
		reactors:     make(map[string]Reactor),
		chDescs:      make([]*ChannelDescriptor, 0),
		reactorsByCh: make(map[byte]Reactor),
		peers:        NewPeerSet(),
		dialing:      cmn.NewCMap(),
		nodeInfo:     nil,
	}

	// Ensure we have a completely undeterministic PRNG. cmd.RandInt64() draws
	// from a seed that's initialized with OS entropy on process start.
	sw.rng = rand.New(rand.NewSource(cmn.RandInt64()))

	// TODO: collapse the peerConfig into the config ?
	sw.peerConfig.MConfig.flushThrottle = time.Duration(config.FlushThrottleTimeout) * time.Millisecond
	sw.peerConfig.MConfig.SendRate = config.SendRate
	sw.peerConfig.MConfig.RecvRate = config.RecvRate
	sw.peerConfig.MConfig.maxMsgPacketPayloadSize = config.MaxMsgPacketPayloadSize

	sw.BaseService = *cmn.NewBaseService(nil, "P2P Switch", sw)
	return sw
}

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
func (sw *Switch) SetNodeInfo(nodeInfo *NodeInfo) {
	sw.nodeInfo = nodeInfo
}

// NodeInfo returns the switch's NodeInfo.
// NOTE: Not goroutine safe.
func (sw *Switch) NodeInfo() *NodeInfo {
	return sw.nodeInfo
}

// SetPeerStore sets the peer store.
func (sw *Switch) SetPeerStore(p ps.Peerstore) {
	sw.peerStore = p
}

// GetPeerStore returns the peer store.
func (sw *Switch) GetPeerStore() ps.Peerstore {
	return sw.peerStore
}

// SetNodePrivKey sets the switch's private key for authenticated encryption.
// NOTE: Overwrites sw.nodeInfo.PubKey.
// NOTE: Not goroutine safe.
func (sw *Switch) SetNodePrivKey(nodePrivKey crypto.PrivKey) {
	sw.nodePrivKey = nodePrivKey
	if sw.nodeInfo != nil {
		pubKey := nodePrivKey.GetPublic()
		pubKeyData, err := pubKey.Bytes()
		if err != nil {
			panic(err)
		}
		pubKeyStrData, err := wdata.Encoder.Marshal(pubKeyData)
		if err != nil {
			panic(err)
		}
		sw.nodeInfo.PubKey = string(pubKeyStrData)
	}
}

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

// addPeer checks the given peer's validity, performs a handshake, and adds the
// peer to the switch and to all registered reactors.
// NOTE: This performs a blocking handshake before the peer is added.
// NOTE: If error is returned, caller is responsible for calling peer.CloseConn()
func (sw *Switch) addPeer(peer *peer) error {
	/*
		if err := sw.FilterConnByAddr(peer.Connection().RemoteAddress()); err != nil {
			return err
		}
	*/

	if err := sw.FilterConnByPubKey(peer.PubKey()); err != nil {
		return err
	}

	if err := peer.HandshakeTimeout(sw.nodeInfo, time.Duration(sw.peerConfig.HandshakeTimeout*time.Second)); err != nil {
		return err
	}

	// Avoid self
	peer.nodeInfo.SetPublicKey(peer.PubKey())
	if sw.nodeInfo.PubKey == peer.nodeInfo.PubKey {
		return errors.New("Ignoring connection from self")
	}

	// Check version, chain id
	if err := sw.nodeInfo.CompatibleWith(peer.NodeInfo()); err != nil {
		return err
	}

	// Check for duplicate peer
	if sw.peers.Has(peer.Key()) {
		return ErrSwitchDuplicatePeer

	}

	// Start peer
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

// FilterConnByAddr returns an error if connecting to the given address is forbidden.
func (sw *Switch) FilterConnByAddr(addr ma.Multiaddr) error {
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
func (sw *Switch) SetAddrFilter(f func(ma.Multiaddr) error) {
	sw.filterConnByAddr = f
}

// SetPubKeyFilter sets the function for filtering connections by public key.
func (sw *Switch) SetPubKeyFilter(f func(crypto.PubKey) error) {
	sw.filterConnByPubKey = f
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

// DialSeeds dials a list of seed peers.
func (sw *Switch) DialSeeds(peerBook *PeerBook, seeds []ps.PeerInfo) error {
	ourPubKey, err := sw.nodeInfo.ParsePublicKey()
	if err != nil {
		return err
	}

	ourPeerId, err := lpeer.IDFromPublicKey(ourPubKey)
	if err != nil {
		return err
	}

	if peerBook != nil {
		for _, peerInfo := range seeds {
			peerBook.AddPeer(peerInfo.ID, ourPeerId)
		}
		peerBook.Save()
	}

	// permute the list, dial them in random order.
	perm := sw.rng.Perm(len(seeds))
	for i := 0; i < len(perm); i++ {
		go func(i int) {
			sw.randomSleep(0)
			j := perm[i]
			sw.DialPeer(seeds[j].ID, true, seeds[j].Addrs...)
		}(i)
	}

	return nil
}

// sleep for interval plus some random amount of ms on [0, dialRandomizerIntervalMilliseconds]
func (sw *Switch) randomSleep(interval time.Duration) {
	r := time.Duration(sw.rng.Int63n(dialRandomizerIntervalMilliseconds)) * time.Millisecond
	time.Sleep(r + interval)
}

func (sw *Switch) dialSeed(id lpeer.ID) {
	_, err := sw.DialPeer(id, true)
	if err != nil {
		sw.Logger.Error("Error dialing seed", "err", err, "peer-id", id.Pretty())
	} else {
		sw.Logger.Info("Connected to seed", "peer", id.Pretty())
	}
}

// DialPeer dials the given peer and runs sw.addPeer if it connects successfully.
// If `persistent == true`, the switch will always try to reconnect to this peer if the connection ever fails.
func (sw *Switch) DialPeer(peerId lpeer.ID, persistent bool, tryAddrs ...ma.Multiaddr) (Peer, error) {
	sw.dialing.Set(peerId.Pretty(), peerId)
	defer sw.dialing.Delete(peerId.Pretty())

	ctxParent := context.Background()

	sw.Logger.Info("Attempting to connect to peer", "peer-id", peerId.Pretty())
	dl := sw.listeners[0].(*DefaultListener)

	ctx, ctxCancel := context.WithTimeout(ctxParent, time.Second*10)
	defer ctxCancel()

	err := dl.host.Connect(ctx, ps.PeerInfo{
		ID:    peerId,
		Addrs: tryAddrs,
	})
	if err != nil {
		sw.Logger.Error("Failed to reach peer", "err", err)
		return nil, err
	}

	outStream, err := dl.host.NewStream(ctxParent, peerId, Protocol)
	if err != nil {
		sw.Logger.Error("Failed to open stream to peer", "err", err, "protocol", Protocol)
	}

	peer, err := newOutboundPeer(outStream, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey, sw.peerConfig)
	if err != nil {
		return nil, err
	}

	peer.SetLogger(sw.Logger.With("peer", peerId.Pretty()))
	if persistent {
		peer.makePersistent()
	}
	err = sw.addPeer(peer)
	if err != nil {
		sw.Logger.Error("Failed to add peer", "peer", peerId.Pretty())
		peer.CloseConn()
		return nil, err
	}
	sw.Logger.Info("Dialed and added peer", "peer", peerId.Pretty())
	return peer, nil
}

// IsDialing returns true if the switch is currently dialing the given peer.
func (sw *Switch) IsDialing(peerId string) bool {
	return sw.dialing.Has(peerId)
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
		go sw.reconnectToPeer(peer)
	}
}

// reconnectToPeer tries to reconnect to the peer, first repeatedly
// with a fixed interval, then with exponential backoff.
// If no success after all that, it stops trying, and leaves it
// to the PEX/Addrbook to find the peer again
func (sw *Switch) reconnectToPeer(peer Peer) {
	peerId := peer.PeerID()
	start := time.Now()
	sw.Logger.Info("Reconnecting to peer", "peer", peerId.Pretty())
	for i := 0; i < reconnectAttempts; i++ {
		if !sw.IsRunning() {
			return
		}

		_, err := sw.DialPeer(peerId, true)
		if err != nil {
			sw.Logger.Info("Error reconnecting to peer. Trying again", "tries", i, "err", err, "peer", peerId.Pretty())
			// sleep a set amount
			sw.randomSleep(reconnectInterval)
			continue
		} else {
			sw.Logger.Info("Reconnected to peer", "peer", peerId.Pretty())
			return
		}
	}

	sw.Logger.Error("Failed to reconnect to peer. Beginning exponential backoff",
		"peer", peerId.Pretty(), "elapsed", time.Since(start))
	for i := 0; i < reconnectBackOffAttempts; i++ {
		if !sw.IsRunning() {
			return
		}

		// sleep an exponentially increasing amount
		sleepIntervalSeconds := math.Pow(reconnectBackOffBaseSeconds, float64(i))
		sw.randomSleep(time.Duration(sleepIntervalSeconds) * time.Second)
		_, err := sw.DialPeer(peerId, true)
		if err != nil {
			sw.Logger.Info("Error reconnecting to peer. Trying again", "tries", i, "err", err, "peer", peerId.Pretty())
			continue
		} else {
			sw.Logger.Info("Reconnected to peer", "peer", peerId.Pretty())
			return
		}
	}
	sw.Logger.Error("Failed to reconnect to peer. Giving up", "peer", peerId.Pretty(), "elapsed", time.Since(start))
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

func (sw *Switch) listenerRoutine(l Listener) {
	for {
		inConn, ok := <-l.Connections()
		if !ok {
			break
		}

		// ignore connection if we already have enough
		/*
			maxPeers := sw.config.MaxNumPeers
			if maxPeers <= sw.peers.Size() {
				sw.Logger.Info("Ignoring inbound connection: already have enough peers", "address", inConn.RemoteAddr().String(), "numPeers", sw.peers.Size(), "max", maxPeers)
				inConn.Close()
				continue
			}
		*/

		// New inbound connection!
		err := sw.addPeerWithConnectionAndConfig(inConn, sw.peerConfig)
		if err != nil {
			sw.Logger.Info("Ignoring inbound connection: error while adding peer", "err", err)
			inConn.Close()
			continue
		}

		// NOTE: We don't yet have the listening port of the
		// remote (if they have a listener at all).
		// The peerHandshake will handle that.
	}

	// cleanup
}

//------------------------------------------------------------------
// Connects switches via arbitrary inet.Stream. Used for testing.

// MakeConnectedSwitches returns n switches, connected according to the connect func.
// If connect==Connect2Switches, the switches will be fully connected.
// initSwitch defines how the i'th switch should be initialized (ie. with what reactors).
// NOTE: panics if any switch fails to start.
func MakeConnectedSwitches(cfg *cfg.P2PConfig, n int, initSwitch func(int, *Switch) *Switch, connect func([]*Switch, int, int)) []*Switch {
	switches := make([]*Switch, n)
	for i := 0; i < n; i++ {
		switches[i] = makeSwitch(cfg, i, "testing", "123.123.123", initSwitch)
	}

	if err := StartSwitches(switches); err != nil {
		panic(err)
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			connect(switches, i, j)
		}
	}

	return switches
}

// Connect2Switches will connect switches i and j via net.Pipe().
// Blocks until a connection is established.
// NOTE: caller ensures i and j are within bounds.
func Connect2Switches(switches []*Switch, i, j int) {
	switchI := switches[i]
	switchJ := switches[j]
	defListenerI := switchI.defaultListener
	defListenerJ := switchJ.defaultListener
	// c1, c2 := netPipe()
	doneCh := make(chan struct{})
	go func() {
		err := defListenerI.host.Connect(context.Background(), ps.PeerInfo{
			ID: defListenerJ.host.ID(),
		})
		if err != nil {
			panic(err)
		}
		doneCh <- struct{}{}
	}()
	go func() {
		err := defListenerJ.host.Connect(context.Background(), ps.PeerInfo{
			ID: defListenerJ.host.ID(),
		})
		if err != nil {
			panic(err)
		}
		doneCh <- struct{}{}
	}()
	<-doneCh
	<-doneCh
}

// StartSwitches calls sw.Start() for each given switch.
// It returns the first encountered error.
func StartSwitches(switches []*Switch) error {
	for _, s := range switches {
		err := s.Start() // start switch and reactors
		if err != nil {
			return err
		}
	}
	return nil
}

func makeSwitch(cfg *cfg.P2PConfig, i int, network, version string, initSwitch func(int, *Switch) *Switch) *Switch {
	privKey, pubKey, err := crypto.GenerateEd25519Key(crand.Reader)
	if err != nil {
		panic(err)
	}

	// new switch, add reactors
	// TODO: let the config be passed in?
	s := initSwitch(i, NewSwitch(cfg))
	ni := &NodeInfo{
		Moniker:    cmn.Fmt("switch%d", i),
		Network:    network,
		Version:    version,
		RemoteAddr: cmn.Fmt("%v:%v", network, rand.Intn(64512)+1023),
		ListenAddr: cmn.Fmt("%v:%v", network, rand.Intn(64512)+1023),
	}
	if err := ni.SetPublicKey(pubKey); err != nil {
		panic(err)
	}
	s.SetNodeInfo(ni)
	s.SetNodePrivKey(privKey)
	return s
}

func (sw *Switch) addPeerWithConnection(conn inet.Stream) error {
	peer, err := newInboundPeer(conn, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey, sw.peerConfig)
	if err != nil {
		if err := conn.Close(); err != nil {
			sw.Logger.Error("Error closing connection", "err", err)
		}
		return err
	}
	peer.SetLogger(sw.Logger.With("peer", conn.Conn().RemotePeer().Pretty()))
	if err = sw.addPeer(peer); err != nil {
		peer.CloseConn()
		return err
	}

	return nil
}

func (sw *Switch) addPeerWithConnectionAndConfig(conn inet.Stream, config *PeerConfig) error {
	peer, err := newInboundPeer(conn, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey, config)
	if err != nil {
		if err := conn.Close(); err != nil {
			sw.Logger.Error("Error closing connection", "err", err)
		}
		return err
	}
	peer.SetLogger(sw.Logger.With("peer", conn.Conn().RemotePeer().Pretty()))
	if err = sw.addPeer(peer); err != nil {
		peer.CloseConn()
		return err
	}

	return nil
}
