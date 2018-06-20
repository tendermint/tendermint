package p2p

import (
	"fmt"
	"net"
	"testing"
	"time"

	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

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
		addr = cmn.Fmt("%X@%v.%v.%v.%v:26656", cmn.RandBytes(20), cmn.RandInt()%256, cmn.RandInt()%256, cmn.RandInt()%256, cmn.RandInt()%256)
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

// MakeConnectedSwitches returns n switches, connected according to the connect
// func. If connect==Connect2Switches, the switches will be fully connected.
// initSwitch defines how the i'th switch should be initialized (ie. with what
// reactors).
// NOTE: panics if any switch fails to start.
func MakeConnectedSwitches(
	t testing.TB,
	cfg *config.P2PConfig,
	n int,
	initSwitch func(int, *Switch) *Switch,
	connect func(testing.TB, []*Switch, int, int),
) []*Switch {
	switches := make([]*Switch, n)
	for i := 0; i < n; i++ {
		switches[i] = MakeSwitch(t, cfg, i, TEST_HOST, "123.123.123", initSwitch)
	}

	if err := StartSwitches(switches); err != nil {
		panic(err)
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			connect(t, switches, i, j)
		}
	}

	return switches
}

// Connect2Switches will connect switches i and j via net.Pipe().
// Blocks until a connection is established, returns early if timeout
// reached.
func Connect2Switches(t testing.TB, ss []*Switch, i, j int) {
	var (
		addc     = make(chan struct{})
		c0, c1   = conn.NetPipe()
		sw0, sw1 = ss[i], ss[j]
	)

	addPeer := func(sw *Switch, c net.Conn, donec chan<- struct{}) {
		p, err := upgrade(c, 10*time.Millisecond, peerConfig{
			chDescs:      sw.chDescs,
			mConfig:      conn.DefaultMConnConfig(),
			nodeInfo:     sw.nodeInfo,
			nodeKey:      *sw.nodeKey,
			onPeerError:  sw.StopPeerForError,
			outbound:     true,
			p2pConfig:    *sw.config,
			reactorsByCh: sw.reactorsByCh,
		})
		if err != nil {
			t.Fatal(err)
		}

		err = sw.addPeer(p)
		if err != nil {
			t.Fatal(err)
		}

		donec <- struct{}{}
	}

	go addPeer(sw0, c0, addc)
	go addPeer(sw1, c1, addc)

	donec := make(chan struct{})
	go func() {
		<-addc
		<-addc
		close(donec)
	}()

	select {
	case <-donec:
	case <-time.After(20 * time.Millisecond):
		panic("Connect2Switches timed out")
	}
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
	t testing.TB,
	cfg *config.P2PConfig,
	i int,
	network, version string,
	initSwitch func(int, *Switch) *Switch,
) *Switch {
	// new switch, add reactors
	// TODO: let the config be passed in?
	var (
		nodeKey = &NodeKey{
			PrivKey: crypto.GenPrivKeyEd25519(),
		}
		nodeInfo = NodeInfo{
			ID:         nodeKey.ID(),
			Moniker:    cmn.Fmt("switch%d", i),
			Network:    network,
			Version:    version,
			ListenAddr: fmt.Sprintf("127.0.0.1:%d", cmn.RandIntn(64512)+1023),
		}
	)

	addr, err := NewNetAddressStringWithOptionalID(nodeInof.ListenAddr)
	if err != nil {
		t.Fatal(err)
	}

	transport := NewMTransport(*addr, nodeInfo, *nodeKey)

	sw := NewSwitch(cfg, transport)
	sw.SetLogger(log.TestingLogger())
	sw.SetNodeInfo(nodeInfo)
	sw.SetNodeKey(nodeKey)
	sw = initSwitch(i, sw)
	for ch := range sw.reactorsByCh {
		nodeInfo.Channels = append(nodeInfo.Channels, ch)
	}
	return sw
}
