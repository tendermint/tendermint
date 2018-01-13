package p2p

import (
	"math/rand"
	"net"

	crypto "github.com/tendermint/go-crypto"
	cfg "github.com/tendermint/tendermint/config"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
)

//------------------------------------------------------------------
// Connects switches via arbitrary net.Conn. Used for testing.

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
	c1, c2 := netPipe()
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
	peer, err := newInboundPeer(conn, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodeKey.PrivKey, sw.peerConfig)
	if err != nil {
		if err := conn.Close(); err != nil {
			sw.Logger.Error("Error closing connection", "err", err)
		}
		return err
	}
	peer.SetLogger(sw.Logger.With("peer", conn.RemoteAddr()))
	if err = sw.addPeer(peer); err != nil {
		peer.CloseConn()
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

func makeSwitch(cfg *cfg.P2PConfig, i int, network, version string, initSwitch func(int, *Switch) *Switch) *Switch {
	// new switch, add reactors
	// TODO: let the config be passed in?
	nodeKey := &NodeKey{
		PrivKey: crypto.GenPrivKeyEd25519().Wrap(),
	}
	s := initSwitch(i, NewSwitch(cfg))
	s.SetNodeInfo(&NodeInfo{
		PubKey:     nodeKey.PubKey(),
		Moniker:    cmn.Fmt("switch%d", i),
		Network:    network,
		Version:    version,
		ListenAddr: cmn.Fmt("%v:%v", network, rand.Intn(64512)+1023),
	})
	s.SetNodeKey(nodeKey)
	s.SetLogger(log.TestingLogger())
	return s
}
