package p2p

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/spf13/viper"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/log15"
	. "github.com/tendermint/tmlibs/common"
)

const (
	reconnectAttempts = 30
	reconnectInterval = 3 * time.Second
)

type Reactor interface {
	Service // Start, Stop

	SetSwitch(*Switch)
	GetChannels() []*ChannelDescriptor
	AddPeer(peer *Peer)
	RemovePeer(peer *Peer, reason interface{})
	Receive(chID byte, peer *Peer, msgBytes []byte)
}

//--------------------------------------

type BaseReactor struct {
	BaseService // Provides Start, Stop, .Quit
	Switch      *Switch
}

func NewBaseReactor(log log15.Logger, name string, impl Reactor) *BaseReactor {
	return &BaseReactor{
		BaseService: *NewBaseService(log, name, impl),
		Switch:      nil,
	}
}

func (br *BaseReactor) SetSwitch(sw *Switch) {
	br.Switch = sw
}
func (_ *BaseReactor) GetChannels() []*ChannelDescriptor              { return nil }
func (_ *BaseReactor) AddPeer(peer *Peer)                             {}
func (_ *BaseReactor) RemovePeer(peer *Peer, reason interface{})      {}
func (_ *BaseReactor) Receive(chID byte, peer *Peer, msgBytes []byte) {}

//-----------------------------------------------------------------------------

/*
The `Switch` handles peer connections and exposes an API to receive incoming messages
on `Reactors`.  Each `Reactor` is responsible for handling incoming messages of one
or more `Channels`.  So while sending outgoing messages is typically performed on the peer,
incoming messages are received on the reactor.
*/
type Switch struct {
	BaseService

	config       *viper.Viper
	listeners    []Listener
	reactors     map[string]Reactor
	chDescs      []*ChannelDescriptor
	reactorsByCh map[byte]Reactor
	peers        *PeerSet
	dialing      *CMap
	nodeInfo     *NodeInfo             // our node info
	nodePrivKey  crypto.PrivKeyEd25519 // our node privkey

	filterConnByAddr   func(net.Addr) error
	filterConnByPubKey func(crypto.PubKeyEd25519) error
}

var (
	ErrSwitchDuplicatePeer      = errors.New("Duplicate peer")
	ErrSwitchMaxPeersPerIPRange = errors.New("IP range has too many peers")
)

func NewSwitch(config *viper.Viper) *Switch {
	setConfigDefaults(config)

	sw := &Switch{
		config:       config,
		reactors:     make(map[string]Reactor),
		chDescs:      make([]*ChannelDescriptor, 0),
		reactorsByCh: make(map[byte]Reactor),
		peers:        NewPeerSet(),
		dialing:      NewCMap(),
		nodeInfo:     nil,
	}
	sw.BaseService = *NewBaseService(log, "P2P Switch", sw)
	return sw
}

// Not goroutine safe.
func (sw *Switch) AddReactor(name string, reactor Reactor) Reactor {
	// Validate the reactor.
	// No two reactors can share the same channel.
	reactorChannels := reactor.GetChannels()
	for _, chDesc := range reactorChannels {
		chID := chDesc.ID
		if sw.reactorsByCh[chID] != nil {
			PanicSanity(fmt.Sprintf("Channel %X has multiple reactors %v & %v", chID, sw.reactorsByCh[chID], reactor))
		}
		sw.chDescs = append(sw.chDescs, chDesc)
		sw.reactorsByCh[chID] = reactor
	}
	sw.reactors[name] = reactor
	reactor.SetSwitch(sw)
	return reactor
}

// Not goroutine safe.
func (sw *Switch) Reactors() map[string]Reactor {
	return sw.reactors
}

// Not goroutine safe.
func (sw *Switch) Reactor(name string) Reactor {
	return sw.reactors[name]
}

// Not goroutine safe.
func (sw *Switch) AddListener(l Listener) {
	sw.listeners = append(sw.listeners, l)
}

// Not goroutine safe.
func (sw *Switch) Listeners() []Listener {
	return sw.listeners
}

// Not goroutine safe.
func (sw *Switch) IsListening() bool {
	return len(sw.listeners) > 0
}

// Not goroutine safe.
func (sw *Switch) SetNodeInfo(nodeInfo *NodeInfo) {
	sw.nodeInfo = nodeInfo
}

// Not goroutine safe.
func (sw *Switch) NodeInfo() *NodeInfo {
	return sw.nodeInfo
}

// Not goroutine safe.
// NOTE: Overwrites sw.nodeInfo.PubKey
func (sw *Switch) SetNodePrivKey(nodePrivKey crypto.PrivKeyEd25519) {
	sw.nodePrivKey = nodePrivKey
	if sw.nodeInfo != nil {
		sw.nodeInfo.PubKey = nodePrivKey.PubKey().Unwrap().(crypto.PubKeyEd25519)
	}
}

// Switch.Start() starts all the reactors, peers, and listeners.
func (sw *Switch) OnStart() error {
	sw.BaseService.OnStart()
	// Start reactors
	for _, reactor := range sw.reactors {
		_, err := reactor.Start()
		if err != nil {
			return err
		}
	}
	// Start peers
	for _, peer := range sw.peers.List() {
		sw.startInitPeer(peer)
	}
	// Start listeners
	for _, listener := range sw.listeners {
		go sw.listenerRoutine(listener)
	}
	return nil
}

func (sw *Switch) OnStop() {
	sw.BaseService.OnStop()
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
	for _, reactor := range sw.reactors {
		reactor.Stop()
	}
}

// NOTE: This performs a blocking handshake before the peer is added.
// CONTRACT: If error is returned, peer is nil, and conn is immediately closed.
func (sw *Switch) AddPeer(peer *Peer) error {
	if err := sw.FilterConnByAddr(peer.Addr()); err != nil {
		return err
	}

	if err := sw.FilterConnByPubKey(peer.PubKey()); err != nil {
		return err
	}

	if err := peer.HandshakeTimeout(sw.nodeInfo, time.Duration(sw.config.GetInt(configKeyHandshakeTimeoutSeconds))*time.Second); err != nil {
		return err
	}

	// Avoid self
	if sw.nodeInfo.PubKey.Equals(peer.PubKey().Wrap()) {
		return errors.New("Ignoring connection from self")
	}

	// Check version, chain id
	if err := sw.nodeInfo.CompatibleWith(peer.NodeInfo); err != nil {
		return err
	}

	// Add the peer to .peers
	// ignore if duplicate or if we already have too many for that IP range
	if err := sw.peers.Add(peer); err != nil {
		log.Notice("Ignoring peer", "error", err, "peer", peer)
		peer.Stop()
		return err
	}

	// Start peer
	if sw.IsRunning() {
		sw.startInitPeer(peer)
	}

	log.Notice("Added peer", "peer", peer)
	return nil
}

func (sw *Switch) FilterConnByAddr(addr net.Addr) error {
	if sw.filterConnByAddr != nil {
		return sw.filterConnByAddr(addr)
	}
	return nil
}

func (sw *Switch) FilterConnByPubKey(pubkey crypto.PubKeyEd25519) error {
	if sw.filterConnByPubKey != nil {
		return sw.filterConnByPubKey(pubkey)
	}
	return nil

}

func (sw *Switch) SetAddrFilter(f func(net.Addr) error) {
	sw.filterConnByAddr = f
}

func (sw *Switch) SetPubKeyFilter(f func(crypto.PubKeyEd25519) error) {
	sw.filterConnByPubKey = f
}

func (sw *Switch) startInitPeer(peer *Peer) {
	peer.Start() // spawn send/recv routines
	for _, reactor := range sw.reactors {
		reactor.AddPeer(peer)
	}
}

// Dial a list of seeds asynchronously in random order
func (sw *Switch) DialSeeds(addrBook *AddrBook, seeds []string) error {

	netAddrs, err := NewNetAddressStrings(seeds)
	if err != nil {
		return err
	}

	if addrBook != nil {
		// add seeds to `addrBook`
		ourAddrS := sw.nodeInfo.ListenAddr
		ourAddr, _ := NewNetAddressString(ourAddrS)
		for _, netAddr := range netAddrs {
			// do not add ourselves
			if netAddr.Equals(ourAddr) {
				continue
			}
			addrBook.AddAddress(netAddr, ourAddr)
		}
		addrBook.Save()
	}

	// permute the list, dial them in random order.
	perm := rand.Perm(len(netAddrs))
	for i := 0; i < len(perm); i++ {
		go func(i int) {
			time.Sleep(time.Duration(rand.Int63n(3000)) * time.Millisecond)
			j := perm[i]
			sw.dialSeed(netAddrs[j])
		}(i)
	}
	return nil
}

func (sw *Switch) dialSeed(addr *NetAddress) {
	peer, err := sw.DialPeerWithAddress(addr, true)
	if err != nil {
		log.Error("Error dialing seed", "error", err)
		return
	} else {
		log.Notice("Connected to seed", "peer", peer)
	}
}

func (sw *Switch) DialPeerWithAddress(addr *NetAddress, persistent bool) (*Peer, error) {
	sw.dialing.Set(addr.IP.String(), addr)
	defer sw.dialing.Delete(addr.IP.String())

	peer, err := newOutboundPeerWithConfig(addr, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey, peerConfigFromGoConfig(sw.config))
	if err != nil {
		log.Info("Failed dialing peer", "address", addr, "error", err)
		return nil, err
	}
	if persistent {
		peer.makePersistent()
	}
	err = sw.AddPeer(peer)
	if err != nil {
		log.Info("Failed adding peer", "address", addr, "error", err)
		peer.CloseConn()
		return nil, err
	}
	log.Notice("Dialed and added peer", "address", addr, "peer", peer)
	return peer, nil
}

func (sw *Switch) IsDialing(addr *NetAddress) bool {
	return sw.dialing.Has(addr.IP.String())
}

// Broadcast runs a go routine for each attempted send, which will block
// trying to send for defaultSendTimeoutSeconds. Returns a channel
// which receives success values for each attempted send (false if times out)
// NOTE: Broadcast uses goroutines, so order of broadcast may not be preserved.
func (sw *Switch) Broadcast(chID byte, msg interface{}) chan bool {
	successChan := make(chan bool, len(sw.peers.List()))
	log.Debug("Broadcast", "channel", chID, "msg", msg)
	for _, peer := range sw.peers.List() {
		go func(peer *Peer) {
			success := peer.Send(chID, msg)
			successChan <- success
		}(peer)
	}
	return successChan
}

// Returns the count of outbound/inbound and outbound-dialing peers.
func (sw *Switch) NumPeers() (outbound, inbound, dialing int) {
	peers := sw.peers.List()
	for _, peer := range peers {
		if peer.outbound {
			outbound++
		} else {
			inbound++
		}
	}
	dialing = sw.dialing.Size()
	return
}

func (sw *Switch) Peers() IPeerSet {
	return sw.peers
}

// Disconnect from a peer due to external error, retry if it is a persistent peer.
// TODO: make record depending on reason.
func (sw *Switch) StopPeerForError(peer *Peer, reason interface{}) {
	addr := NewNetAddress(peer.Addr())
	log.Notice("Stopping peer for error", "peer", peer, "error", reason)
	sw.stopAndRemovePeer(peer, reason)

	if peer.IsPersistent() {
		go func() {
			log.Notice("Reconnecting to peer", "peer", peer)
			for i := 1; i < reconnectAttempts; i++ {
				if !sw.IsRunning() {
					return
				}

				peer, err := sw.DialPeerWithAddress(addr, true)
				if err != nil {
					if i == reconnectAttempts {
						log.Notice("Error reconnecting to peer. Giving up", "tries", i, "error", err)
						return
					}
					log.Notice("Error reconnecting to peer. Trying again", "tries", i, "error", err)
					time.Sleep(reconnectInterval)
					continue
				}

				log.Notice("Reconnected to peer", "peer", peer)
				return
			}
		}()
	}
}

// Disconnect from a peer gracefully.
// TODO: handle graceful disconnects.
func (sw *Switch) StopPeerGracefully(peer *Peer) {
	log.Notice("Stopping peer gracefully")
	sw.stopAndRemovePeer(peer, nil)
}

func (sw *Switch) stopAndRemovePeer(peer *Peer, reason interface{}) {
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
		maxPeers := sw.config.GetInt(configKeyMaxNumPeers)
		if maxPeers <= sw.peers.Size() {
			log.Info("Ignoring inbound connection: already have enough peers", "address", inConn.RemoteAddr().String(), "numPeers", sw.peers.Size(), "max", maxPeers)
			continue
		}

		// New inbound connection!
		err := sw.addPeerWithConnectionAndConfig(inConn, peerConfigFromGoConfig(sw.config))
		if err != nil {
			log.Notice("Ignoring inbound connection: error while adding peer", "address", inConn.RemoteAddr().String(), "error", err)
			continue
		}

		// NOTE: We don't yet have the listening port of the
		// remote (if they have a listener at all).
		// The peerHandshake will handle that
	}

	// cleanup
}

//-----------------------------------------------------------------------------

type SwitchEventNewPeer struct {
	Peer *Peer
}

type SwitchEventDonePeer struct {
	Peer  *Peer
	Error interface{}
}

//------------------------------------------------------------------
// Switches connected via arbitrary net.Conn; useful for testing

// Returns n switches, connected according to the connect func.
// If connect==Connect2Switches, the switches will be fully connected.
// initSwitch defines how the ith switch should be initialized (ie. with what reactors).
// NOTE: panics if any switch fails to start.
func MakeConnectedSwitches(n int, initSwitch func(int, *Switch) *Switch, connect func([]*Switch, int, int)) []*Switch {
	switches := make([]*Switch, n)
	for i := 0; i < n; i++ {
		switches[i] = makeSwitch(i, "testing", "123.123.123", initSwitch)
	}

	if err := StartSwitches(switches); err != nil {
		panic(err)
	}

	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			connect(switches, i, j)
		}
	}

	return switches
}

var PanicOnAddPeerErr = false

// Will connect switches i and j via net.Pipe()
// Blocks until a conection is established.
// NOTE: caller ensures i and j are within bounds
func Connect2Switches(switches []*Switch, i, j int) {
	switchI := switches[i]
	switchJ := switches[j]
	c1, c2 := net.Pipe()
	doneCh := make(chan struct{})
	go func() {
		err := switchI.addPeerWithConnection(c1)
		if PanicOnAddPeerErr && err != nil {
			panic(err)
		}
		doneCh <- struct{}{}
	}()
	go func() {
		err := switchJ.addPeerWithConnection(c2)
		if PanicOnAddPeerErr && err != nil {
			panic(err)
		}
		doneCh <- struct{}{}
	}()
	<-doneCh
	<-doneCh
}

func StartSwitches(switches []*Switch) error {
	for _, s := range switches {
		_, err := s.Start() // start switch and reactors
		if err != nil {
			return err
		}
	}
	return nil
}

func makeSwitch(i int, network, version string, initSwitch func(int, *Switch) *Switch) *Switch {
	privKey := crypto.GenPrivKeyEd25519()
	// new switch, add reactors
	// TODO: let the config be passed in?
	s := initSwitch(i, NewSwitch(viper.New()))
	s.SetNodeInfo(&NodeInfo{
		PubKey:     privKey.PubKey().Unwrap().(crypto.PubKeyEd25519),
		Moniker:    Fmt("switch%d", i),
		Network:    network,
		Version:    version,
		RemoteAddr: Fmt("%v:%v", network, rand.Intn(64512)+1023),
		ListenAddr: Fmt("%v:%v", network, rand.Intn(64512)+1023),
	})
	s.SetNodePrivKey(privKey)
	return s
}

func (sw *Switch) addPeerWithConnection(conn net.Conn) error {
	peer, err := newInboundPeer(conn, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey)
	if err != nil {
		conn.Close()
		return err
	}

	if err = sw.AddPeer(peer); err != nil {
		conn.Close()
		return err
	}

	return nil
}

func (sw *Switch) addPeerWithConnectionAndConfig(conn net.Conn, config *PeerConfig) error {
	peer, err := newInboundPeerWithConfig(conn, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey, config)
	if err != nil {
		conn.Close()
		return err
	}

	if err = sw.AddPeer(peer); err != nil {
		conn.Close()
		return err
	}

	return nil
}

func peerConfigFromGoConfig(config *viper.Viper) *PeerConfig {
	return &PeerConfig{
		AuthEnc:          config.GetBool(configKeyAuthEnc),
		Fuzz:             config.GetBool(configFuzzEnable),
		HandshakeTimeout: time.Duration(config.GetInt(configKeyHandshakeTimeoutSeconds)) * time.Second,
		DialTimeout:      time.Duration(config.GetInt(configKeyDialTimeoutSeconds)) * time.Second,
		MConfig: &MConnConfig{
			SendRate: int64(config.GetInt(configKeySendRate)),
			RecvRate: int64(config.GetInt(configKeyRecvRate)),
		},
		FuzzConfig: &FuzzConnConfig{
			Mode:         config.GetInt(configFuzzMode),
			MaxDelay:     time.Duration(config.GetInt(configFuzzMaxDelayMilliseconds)) * time.Millisecond,
			ProbDropRW:   config.GetFloat64(configFuzzProbDropRW),
			ProbDropConn: config.GetFloat64(configFuzzProbDropConn),
			ProbSleep:    config.GetFloat64(configFuzzProbSleep),
		},
	}
}
