package p2p

import (
	"context"
	"fmt"
	mrand "math/rand"
	"net"

	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/types"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/p2p/conn"
)

const testCh = 0x01

//------------------------------------------------

func AddPeerToSwitchPeerSet(sw *Switch, peer Peer) {
	sw.peers.Add(peer) //nolint:errcheck // ignore error
}

func CreateRandomPeer(outbound bool) Peer {
	addr, netAddr := CreateRoutableAddr()
	p := &peer{
		peerConn: peerConn{outbound: outbound},
		nodeInfo: types.NodeInfo{
			NodeID:     netAddr.ID,
			ListenAddr: netAddr.DialString(),
		},
		metrics: NopMetrics(),
	}
	p.SetLogger(log.TestingLogger().With("peer", addr))
	return p
}

// nolint:gosec // G404: Use of weak random number generator
func CreateRoutableAddr() (addr string, netAddr *NetAddress) {
	for {
		var err error
		addr = fmt.Sprintf("%X@%v.%v.%v.%v:26656",
			tmrand.Bytes(20),
			mrand.Int()%256,
			mrand.Int()%256,
			mrand.Int()%256,
			mrand.Int()%256)
		netAddr, err = types.NewNetAddressString(addr)
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

const TestHost = "localhost"

// MakeConnectedSwitches returns n switches, connected according to the connect func.
// If connect==Connect2Switches, the switches will be fully connected.
// initSwitch defines how the i'th switch should be initialized (ie. with what reactors).
// NOTE: panics if any switch fails to start.
func MakeConnectedSwitches(cfg *config.P2PConfig,
	n int,
	initSwitch func(int, *Switch) *Switch,
	connect func([]*Switch, int, int),
) []*Switch {
	switches := make([]*Switch, n)
	for i := 0; i < n; i++ {
		switches[i] = MakeSwitch(cfg, i, TestHost, "123.123.123", initSwitch, log.TestingLogger())
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
	pc, err := testInboundPeerConn(sw.transport.(*MConnTransport), conn)
	if err != nil {
		if err := conn.Close(); err != nil {
			sw.Logger.Error("Error closing connection", "err", err)
		}
		return err
	}
	peerNodeInfo, _, err := pc.conn.Handshake(context.Background(), sw.nodeInfo, sw.nodeKey.PrivKey)
	if err != nil {
		if err := conn.Close(); err != nil {
			sw.Logger.Error("Error closing connection", "err", err)
		}
		return err
	}

	p := newPeer(
		peerNodeInfo,
		pc,
		sw.reactorsByCh,
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
	logger log.Logger,
	opts ...SwitchOption,
) *Switch {

	nodeKey := types.GenNodeKey()
	nodeInfo := testNodeInfo(nodeKey.ID, fmt.Sprintf("node%d", i))
	addr, err := types.NewNetAddressString(
		nodeKey.ID.AddressString(nodeInfo.ListenAddr),
	)
	if err != nil {
		panic(err)
	}

	swLogger := logger.With("switch", i)
	t := NewMConnTransport(swLogger, MConnConfig(cfg),
		[]*ChannelDescriptor{}, MConnTransportOptions{})

	// TODO: let the config be passed in?
	sw := initSwitch(i, NewSwitch(cfg, t, opts...))
	sw.SetLogger(swLogger)
	sw.SetNodeKey(nodeKey)

	if err := t.Listen(NewEndpoint(addr)); err != nil {
		panic(err)
	}

	ni := nodeInfo
	ni.Channels = []byte{}
	for ch := range sw.reactorsByCh {
		ni.Channels = append(ni.Channels, ch)
	}
	nodeInfo = ni

	// TODO: We need to setup reactors ahead of time so the NodeInfo is properly
	// populated and we don't have to do those awkward overrides and setters.
	sw.SetNodeInfo(nodeInfo)

	return sw
}

func testInboundPeerConn(
	transport *MConnTransport,
	conn net.Conn,
) (peerConn, error) {
	return testPeerConn(transport, conn, false, false)
}

func testPeerConn(
	transport *MConnTransport,
	rawConn net.Conn,
	outbound, persistent bool,
) (pc peerConn, err error) {

	conn := newMConnConnection(transport.logger, rawConn, transport.mConnConfig, transport.channelDescs)

	return newPeerConn(outbound, persistent, conn), nil
}

//----------------------------------------------------------------
// rand node info

func testNodeInfo(id types.NodeID, name string) types.NodeInfo {
	return testNodeInfoWithNetwork(id, name, "testing")
}

func testNodeInfoWithNetwork(id types.NodeID, name, network string) types.NodeInfo {
	return types.NodeInfo{
		ProtocolVersion: defaultProtocolVersion,
		NodeID:          id,
		ListenAddr:      fmt.Sprintf("127.0.0.1:%d", getFreePort()),
		Network:         network,
		Version:         "1.2.3-rc0-deadbeef",
		Channels:        []byte{testCh},
		Moniker:         name,
		Other: types.NodeInfoOther{
			TxIndex:    "on",
			RPCAddress: fmt.Sprintf("127.0.0.1:%d", getFreePort()),
		},
	}
}

func getFreePort() int {
	port, err := tmnet.GetFreePort()
	if err != nil {
		panic(err)
	}
	return port
}

type AddrBookMock struct {
	Addrs        map[string]struct{}
	OurAddrs     map[string]struct{}
	PrivateAddrs map[string]struct{}
}

var _ AddrBook = (*AddrBookMock)(nil)

func (book *AddrBookMock) AddAddress(addr *NetAddress, src *NetAddress) error {
	book.Addrs[addr.String()] = struct{}{}
	return nil
}
func (book *AddrBookMock) AddOurAddress(addr *NetAddress) { book.OurAddrs[addr.String()] = struct{}{} }
func (book *AddrBookMock) OurAddress(addr *NetAddress) bool {
	_, ok := book.OurAddrs[addr.String()]
	return ok
}
func (book *AddrBookMock) MarkGood(types.NodeID) {}
func (book *AddrBookMock) HasAddress(addr *NetAddress) bool {
	_, ok := book.Addrs[addr.String()]
	return ok
}
func (book *AddrBookMock) RemoveAddress(addr *NetAddress) {
	delete(book.Addrs, addr.String())
}
func (book *AddrBookMock) Save() {}
func (book *AddrBookMock) AddPrivateIDs(addrs []string) {
	for _, addr := range addrs {
		book.PrivateAddrs[addr] = struct{}{}
	}
}
