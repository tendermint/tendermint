package p2p

import (
	"fmt"
	"net"
	"time"

	crypto "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p/conn"
)

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
		addr = fmt.Sprintf("%X@%v.%v.%v.%v:26656", cmn.RandBytes(20), cmn.RandInt()%256, cmn.RandInt()%256, cmn.RandInt()%256, cmn.RandInt()%256)
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
func MakeConnectedSwitches(cfg *config.P2PConfig, n int, initSwitch func(int, *Switch) *Switch, connect func([]*Switch, int, int)) []*Switch {
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

	c1, c2 := conn.NetPipe()

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
	pc, err := testInboundPeerConn(conn, sw.config, sw.nodeKey.PrivKey)
	if err != nil {
		if err := conn.Close(); err != nil {
			sw.Logger.Error("Error closing connection", "err", err)
		}
		return err
	}

	ni, err := handshake(conn, 50*time.Millisecond, sw.nodeInfo)
	if err != nil {
		if err := conn.Close(); err != nil {
			sw.Logger.Error("Error closing connection", "err", err)
		}
		return err
	}

	p := newPeer(
		pc,
		sw.mConfig,
		ni,
		sw.reactorsByCh,
		sw.chDescs,
		sw.StopPeerForError,
	)

	if err = sw.addPeer(p); err != nil {
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

func MakeSwitch(
	cfg *config.P2PConfig,
	i int,
	network, version string,
	initSwitch func(int, *Switch) *Switch,
	opts ...SwitchOption,
) *Switch {
	var (
		nodeKey = NodeKey{
			PrivKey: ed25519.GenPrivKey(),
		}
		ni = NodeInfo{
			ID:         nodeKey.ID(),
			Moniker:    fmt.Sprintf("switch%d", i),
			Network:    network,
			Version:    version,
			ListenAddr: fmt.Sprintf("127.0.0.1:%d", cmn.RandIntn(64512)+1023),
			Other: NodeInfoOther{
				AminoVersion:     "1.0",
				P2PVersion:       "1.0",
				ConsensusVersion: "1.0",
				RPCVersion:       "1.0",
				TxIndex:          "off",
				RPCAddress:       fmt.Sprintf("127.0.0.1:%d", cmn.RandIntn(64512)+1023),
			},
		}
	)

	addr, err := NewNetAddressStringWithOptionalID(
		IDAddressString(nodeKey.ID(), ni.ListenAddr),
	)
	if err != nil {
		panic(err)
	}

	t := NewMultiplexTransport(ni, nodeKey)

	if err := t.Listen(*addr); err != nil {
		panic(err)
	}

	// TODO: let the config be passed in?
	sw := initSwitch(i, NewSwitch(cfg, t, opts...))
	sw.SetLogger(log.TestingLogger())
	sw.SetNodeKey(&nodeKey)

	for ch := range sw.reactorsByCh {
		ni.Channels = append(ni.Channels, ch)
	}

	// TODO: We need to setup reactors ahead of time so the NodeInfo is properly
	// populated and we don't have to do those awkward overrides and setters.
	t.nodeInfo = ni
	sw.SetNodeInfo(ni)

	return sw
}

func testInboundPeerConn(
	conn net.Conn,
	config *config.P2PConfig,
	ourNodePrivKey crypto.PrivKey,
) (peerConn, error) {
	return testPeerConn(conn, config, false, false, ourNodePrivKey, nil)
}

func testPeerConn(
	rawConn net.Conn,
	cfg *config.P2PConfig,
	outbound, persistent bool,
	ourNodePrivKey crypto.PrivKey,
	originalAddr *NetAddress,
) (pc peerConn, err error) {
	conn := rawConn

	// Fuzz connection
	if cfg.TestFuzz {
		// so we have time to do peer handshakes and get set up
		conn = FuzzConnAfterFromConfig(conn, 10*time.Second, cfg.TestFuzzConfig)
	}

	// Encrypt connection
	conn, err = upgradeSecretConn(conn, cfg.HandshakeTimeout, ourNodePrivKey)
	if err != nil {
		return pc, cmn.ErrorWrap(err, "Error creating peer")
	}

	// Only the information we already have
	return peerConn{
		config:       cfg,
		outbound:     outbound,
		persistent:   persistent,
		conn:         conn,
		originalAddr: originalAddr,
	}, nil
}
