package p2p

import (
	"fmt"
	golog "log"
	"net"
	"time"

	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p/conn"
)

const testCh = 0x01

func AddPeerToSwitch(sw *Switch, peer Peer) {
	sw.peers.Add(peer)
}

func CreateRandomPeer(outbound bool) *peer {
	addr, netAddr := CreateRoutableAddr()
	p := &peer{
		peerConn: peerConn{
			outbound: outbound,
		},
		nodeInfo: NodeInfo{
			ID:         netAddr.ID,
			ListenAddr: netAddr.DialString(),
		},
		mconn: &conn.MConnection{},
	}
	p.SetLogger(log.TestingLogger().With("peer", addr))
	return p
}

func CreateRoutableAddr() (addr string, netAddr *NetAddress) {
	for {
		var err error
		addr = cmn.Fmt("%X@%v.%v.%v.%v:46656", cmn.RandBytes(20), cmn.RandInt()%256, cmn.RandInt()%256, cmn.RandInt()%256, cmn.RandInt()%256)
		netAddr, err = NewNetAddressString(addr)
		if err != nil {
			panic(err)
		}
		if netAddr.Routable() {
			break
		}
	}
	return
}

//------------------------------------------------------------------
// Connects switches via arbitrary net.Conn. Used for testing.

const TEST_HOST = "localhost"

// MakeConnectedSwitches returns n switches, connected according to the connect func.
// If connect==Connect2Switches, the switches will be fully connected.
// initSwitch defines how the i'th switch should be initialized (ie. with what reactors).
// NOTE: panics if any switch fails to start.
func MakeConnectedSwitches(cfg *cfg.P2PConfig, n int, initSwitch func(int, *Switch) *Switch, connect func([]*Switch, int, int)) []*Switch {
	switches := make([]*Switch, n)
	for i := 0; i < n; i++ {
		switches[i] = MakeSwitch(cfg, i, TEST_HOST, "123.123.123", initSwitch)
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

	p1 := &remotePeer{
		Config:   switchJ.peerConfig,
		PrivKey:  switchJ.nodeKey.PrivKey,
		channels: switchJ.NodeInfo().Channels,
	}
	p1.Start()

	c1, err := net.DialTimeout(
		"tcp",
		fmt.Sprintf("%s:%d", p1.addr.IP.String(), p1.addr.Port),
		100*time.Millisecond,
	)
	if err != nil {
		panic(err)
	}

	p2 := &remotePeer{
		Config:   switchI.peerConfig,
		PrivKey:  switchI.nodeKey.PrivKey,
		channels: switchI.NodeInfo().Channels,
	}
	p2.Start()

	c2, err := net.DialTimeout(
		"tcp",
		fmt.Sprintf("%s:%d", p2.addr.IP.String(), p2.addr.Port),
		100*time.Millisecond,
	)
	if err != nil {
		panic(err)
	}

	doneCh := make(chan struct{})
	go func() {
		err := switchI.addPeerWithConnection(c1)
		if err != nil {
			panic(err)
		}
		doneCh <- struct{}{}
	}()
	go func() {
		err := switchJ.addPeerWithConnection(c2)
		if err != nil {
			panic(err)
		}
		doneCh <- struct{}{}
	}()
	<-doneCh
	<-doneCh
}

func (sw *Switch) addPeerWithConnection(conn net.Conn) error {
	pc, err := newInboundPeerConn(conn, sw.peerConfig, sw.nodeKey.PrivKey)
	if err != nil {
		if err := conn.Close(); err != nil {
			sw.Logger.Error("Error closing connection", "err", err)
		}
		return err
	}
	if err = sw.addPeer(pc); err != nil {
		pc.CloseConn()
		return err
	}

	return nil
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

func MakeSwitch(cfg *cfg.P2PConfig, i int, network, version string, initSwitch func(int, *Switch) *Switch) *Switch {
	// new switch, add reactors
	// TODO: let the config be passed in?
	nodeKey := &NodeKey{
		PrivKey: crypto.GenPrivKeyEd25519(),
	}
	sw := NewSwitch(cfg)
	sw.SetLogger(log.TestingLogger())
	sw = initSwitch(i, sw)
	ni := NodeInfo{
		ID:         nodeKey.ID(),
		Moniker:    cmn.Fmt("switch%d", i),
		Network:    network,
		Version:    version,
		ListenAddr: cmn.Fmt("%v:%v", network, cmn.RandIntn(64512)+1023),
	}
	for ch := range sw.reactorsByCh {
		ni.Channels = append(ni.Channels, ch)
	}
	sw.SetNodeInfo(ni)
	sw.SetNodeKey(nodeKey)
	return sw
}

type remotePeer struct {
	PrivKey  crypto.PrivKey
	Config   *PeerConfig
	addr     *NetAddress
	quit     chan struct{}
	channels cmn.HexBytes
}

func (rp *remotePeer) Addr() *NetAddress {
	return rp.addr
}

func (rp *remotePeer) ID() ID {
	return PubKeyToID(rp.PrivKey.PubKey())
}

func (rp *remotePeer) Start() {
	l, e := net.Listen("tcp", "127.0.0.1:0") // any available address
	if e != nil {
		golog.Fatalf("net.Listen tcp :0: %+v", e)
	}
	rp.addr = NewNetAddress(PubKeyToID(rp.PrivKey.PubKey()), l.Addr())
	rp.quit = make(chan struct{})
	if rp.channels == nil {
		rp.channels = []byte{testCh}
	}
	go rp.accept(l)
}

func (rp *remotePeer) Stop() {
	close(rp.quit)
}

func (rp *remotePeer) accept(l net.Listener) {
	conns := []net.Conn{}

	for {
		conn, err := l.Accept()
		if err != nil {
			golog.Fatalf("Failed to accept conn: %+v", err)
		}

		pc, err := newInboundPeerConn(conn, rp.Config, rp.PrivKey)
		if err != nil {
			golog.Fatalf("Failed to create a peer: %+v", err)
		}

		_, err = pc.HandshakeTimeout(NodeInfo{
			ID:         rp.Addr().ID,
			Moniker:    "remote_peer",
			Network:    "localhost",
			Version:    "123.123.123",
			ListenAddr: l.Addr().String(),
			Channels:   rp.channels,
		}, 1*time.Second)
		if err != nil {
			golog.Fatalf("Failed to perform handshake: %+v", err)
		}

		conns = append(conns, conn)

		select {
		case <-rp.quit:
			for _, conn := range conns {
				if err := conn.Close(); err != nil {
					golog.Fatal(err)
				}
			}
			return
		default:
		}
	}
}
