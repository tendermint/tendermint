package p2p

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tmlibs/log"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p/conn"
)

var (
	cfg *config.P2PConfig
)

func init() {
	cfg = config.DefaultP2PConfig()
	cfg.PexReactor = true
	cfg.AllowDuplicateIP = true
}

type PeerMessage struct {
	PeerID  ID
	Bytes   []byte
	Counter int
}

type TestReactor struct {
	BaseReactor

	mtx          sync.Mutex
	channels     []*conn.ChannelDescriptor
	logMessages  bool
	msgsCounter  int
	msgsReceived map[byte][]PeerMessage
}

func NewTestReactor(channels []*conn.ChannelDescriptor, logMessages bool) *TestReactor {
	tr := &TestReactor{
		channels:     channels,
		logMessages:  logMessages,
		msgsReceived: make(map[byte][]PeerMessage),
	}
	tr.BaseReactor = *NewBaseReactor("TestReactor", tr)
	tr.SetLogger(log.TestingLogger())
	return tr
}

func (tr *TestReactor) GetChannels() []*conn.ChannelDescriptor {
	return tr.channels
}

func (tr *TestReactor) AddPeer(peer Peer) {}

func (tr *TestReactor) RemovePeer(peer Peer, reason interface{}) {}

func (tr *TestReactor) Receive(chID byte, peer Peer, msgBytes []byte) {
	if tr.logMessages {
		tr.mtx.Lock()
		defer tr.mtx.Unlock()
		//fmt.Printf("Received: %X, %X\n", chID, msgBytes)
		tr.msgsReceived[chID] = append(tr.msgsReceived[chID], PeerMessage{peer.ID(), msgBytes, tr.msgsCounter})
		tr.msgsCounter++
	}
}

func (tr *TestReactor) getMsgs(chID byte) []PeerMessage {
	tr.mtx.Lock()
	defer tr.mtx.Unlock()
	return tr.msgsReceived[chID]
}

//-----------------------------------------------------------------------------

// convenience method for creating two switches connected to each other.
// XXX: note this uses net.Pipe and not a proper TCP conn
func MakeSwitchPair(
	t testing.TB,
	initSwitch func(int, *Switch) *Switch,
) (*Switch, *Switch) {
	// Create two switches that will be interconnected.
	switches := MakeConnectedSwitches(t, cfg, 2, initSwitch, Connect2Switches)
	return switches[0], switches[1]
}

func initSwitchFunc(i int, sw *Switch) *Switch {
	sw.SetAddrBook(&addrBookMock{
		addrs:    make(map[string]struct{}),
		ourAddrs: make(map[string]struct{})})

	// Make two reactors of two channels each
	sw.AddReactor("foo", NewTestReactor([]*conn.ChannelDescriptor{
		{ID: byte(0x00), Priority: 10},
		{ID: byte(0x01), Priority: 10},
	}, true))
	sw.AddReactor("bar", NewTestReactor([]*conn.ChannelDescriptor{
		{ID: byte(0x02), Priority: 10},
		{ID: byte(0x03), Priority: 10},
	}, true))

	return sw
}

func TestSwitches(t *testing.T) {
	s1, s2 := MakeSwitchPair(t, initSwitchFunc)
	defer s1.Stop()
	defer s2.Stop()

	if s1.Peers().Size() != 1 {
		t.Errorf("Expected exactly 1 peer in s1, got %v", s1.Peers().Size())
	}
	if s2.Peers().Size() != 1 {
		t.Errorf("Expected exactly 1 peer in s2, got %v", s2.Peers().Size())
	}

	// Lets send some messages
	ch0Msg := []byte("channel zero")
	ch1Msg := []byte("channel foo")
	ch2Msg := []byte("channel bar")

	s1.Broadcast(byte(0x00), ch0Msg)
	s1.Broadcast(byte(0x01), ch1Msg)
	s1.Broadcast(byte(0x02), ch2Msg)

	assertMsgReceivedWithTimeout(t, ch0Msg, byte(0x00), s2.Reactor("foo").(*TestReactor), 10*time.Millisecond, 5*time.Second)
	assertMsgReceivedWithTimeout(t, ch1Msg, byte(0x01), s2.Reactor("foo").(*TestReactor), 10*time.Millisecond, 5*time.Second)
	assertMsgReceivedWithTimeout(t, ch2Msg, byte(0x02), s2.Reactor("bar").(*TestReactor), 10*time.Millisecond, 5*time.Second)
}

func assertMsgReceivedWithTimeout(t *testing.T, msgBytes []byte, channel byte, reactor *TestReactor, checkPeriod, timeout time.Duration) {
	ticker := time.NewTicker(checkPeriod)
	for {
		select {
		case <-ticker.C:
			msgs := reactor.getMsgs(channel)
			if len(msgs) > 0 {
				if !bytes.Equal(msgs[0].Bytes, msgBytes) {
					t.Fatalf("Unexpected message bytes. Wanted: %X, Got: %X", msgBytes, msgs[0].Bytes)
				}
				return
			}
		case <-time.After(timeout):
			t.Fatalf("Expected to have received 1 message in channel #%v, got zero", channel)
		}
	}
}

func TestConnAddrFilter(t *testing.T) {
	s0 := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc)
	s1 := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc)
	defer s0.Stop()
	defer s1.Stop()

	s1.SetAddrFilter(func(addr net.Addr) error {
		if addr.String() == "pipe" {
			return fmt.Errorf("Error: pipe is blacklisted")
		}
		return nil
	})

	Connect2Switches(t, s0, s1)

	assertNoPeersAfterTimeout(t, s0, 400*time.Millisecond)
	assertNoPeersAfterTimeout(t, s1, 400*time.Millisecond)
}

func TestSwitchFiltersOutItself(t *testing.T) {
	s1 := MakeSwitch(cfg, 1, "127.0.0.1", "123.123.123", initSwitchFunc)
	// addr := s1.NodeInfo().NetAddress()

	// // add ourselves like we do in node.go#427
	// s1.addrBook.AddOurAddress(addr)

	// simulate s1 having a public IP by creating a remote peer with the same ID
	rp := &remotePeer{PrivKey: s1.nodeKey.PrivKey, Config: cfg}
	rp.Start()

	// addr should be rejected in addPeer based on the same ID
	err := s1.DialPeerWithAddress(rp.Addr(), false)
	if assert.Error(t, err) {
		assert.Equal(t, ErrSwitchConnectToSelf{rp.Addr()}.Error(), err.Error())
	}

	assert.True(t, s1.addrBook.OurAddress(rp.Addr()))

	assert.False(t, s1.addrBook.HasAddress(rp.Addr()))

	rp.Stop()

	assertNoPeersAfterTimeout(t, s1, 100*time.Millisecond)
}

func assertNoPeersAfterTimeout(t *testing.T, sw *Switch, timeout time.Duration) {
	time.Sleep(timeout)
	if sw.Peers().Size() != 0 {
		t.Fatalf("Expected %v to not connect to some peers, got %d", sw, sw.Peers().Size())
	}
}

func TestConnIDFilter(t *testing.T) {
	sw0 := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc)
	sw1 := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc)
	defer sw0.Stop()
	defer sw1.Stop()

	sw0.SetIDFilter(func(id ID) error {
		if id == sw1.nodeInfo.ID {
			return fmt.Errorf("Error: pipe is blacklisted")
		}
		return nil
	})

	sw1.SetIDFilter(func(id ID) error {
		if id == sw0.nodeInfo.ID {
			return fmt.Errorf("Error: pipe is blacklisted")
		}
		return nil
	})

	Connect2Switches(t, sw0, sw1)

	assertNoPeersAfterTimeout(t, sw0, 400*time.Millisecond)
	assertNoPeersAfterTimeout(t, sw1, 400*time.Millisecond)
}

func TestSwitchStopsNonPersistentPeerOnError(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	sw := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc)
	err := sw.Start()
	if err != nil {
		t.Error(err)
	}
	defer sw.Stop()

	// simulate remote peer
	rp := &remotePeer{PrivKey: crypto.GenPrivKeyEd25519(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	timeout := 10 * time.Millisecond

	c, err := rp.Addr().DialTimeout(timeout)
	if err != nil {
		t.Fatal(err)
	}

	p, err := upgrade(c, timeout, peerConfig{
		chDescs:      sw.chDescs,
		mConfig:      conn.DefaultMConnConfig(),
		nodeInfo:     sw.nodeInfo,
		nodeKey:      *sw.nodeKey,
		onPeerError:  sw.StopPeerForError,
		outbound:     true,
		p2pConfig:    *sw.config,
		reactorsByCh: sw.reactorsByCh,
	})
	require.Nil(err)
	require.Nil(sw.addPeer(p))

	peer := sw.Peers().Get(rp.ID())
	require.NotNil(peer)

	// simulate failure by closing connection
	p.CloseConn()

	assertNoPeersAfterTimeout(t, sw, 100*time.Millisecond)
	assert.False(peer.IsRunning())
}

func TestSwitchReconnectsToPersistentPeer(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	sw := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc)
	err := sw.Start()
	if err != nil {
		t.Error(err)
	}
	defer sw.Stop()

	// simulate remote peer
	rp := &remotePeer{PrivKey: crypto.GenPrivKeyEd25519(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	timeout := 10 * time.Millisecond

	c, err := rp.Addr().DialTimeout(timeout)
	if err != nil {
		t.Fatal(err)
	}

	p, err := upgrade(c, timeout, peerConfig{
		chDescs:      sw.chDescs,
		mConfig:      conn.DefaultMConnConfig(),
		nodeInfo:     sw.nodeInfo,
		nodeKey:      *sw.nodeKey,
		onPeerError:  sw.StopPeerForError,
		outbound:     true,
		p2pConfig:    *sw.config,
		reactorsByCh: sw.reactorsByCh,
	})
	require.Nil(err)

	require.Nil(sw.addPeer(p))

	peer := sw.Peers().Get(rp.ID())
	require.NotNil(peer)

	// simulate failure by closing connection
	p.CloseConn()

	// TODO: remove sleep, detect the disconnection, wait for reconnect
	npeers := sw.Peers().Size()
	for i := 0; i < 20; i++ {
		time.Sleep(250 * time.Millisecond)
		npeers = sw.Peers().Size()
		if npeers > 0 {
			break
		}
	}
	assert.NotZero(npeers)
	assert.False(peer.IsRunning())

	// simulate another remote peer
	rp = &remotePeer{
		PrivKey: crypto.GenPrivKeyEd25519(),
		Config:  cfg,
		// Use different interface to prevent duplicate IP filter, this will break
		// beyond two peers.
		listenAddr: "127.0.0.1:0",
	}
	rp.Start()
	defer rp.Stop()

	// simulate first time dial failure
	c, err = rp.Addr().DialTimeout(timeout)
	if err != nil {
		t.Fatal(err)
	}

	conf := config.DefaultP2PConfig()
	conf.TestDialFail = true

	p, err = upgrade(c, timeout, peerConfig{
		chDescs:      sw.chDescs,
		mConfig:      conn.DefaultMConnConfig(),
		nodeInfo:     sw.nodeInfo,
		nodeKey:      *sw.nodeKey,
		onPeerError:  sw.StopPeerForError,
		outbound:     true,
		p2pConfig:    *conf,
		reactorsByCh: sw.reactorsByCh,
	})
	require.NotNil(err)

	require.Nil(sw.addPeer(p))

	// DialPeerWithAddres - sw.peerConfig resets the dialer

	// TODO: same as above
	for i := 0; i < 20; i++ {
		time.Sleep(250 * time.Millisecond)
		npeers = sw.Peers().Size()
		if npeers > 1 {
			break
		}
	}
	assert.EqualValues(2, npeers)
}

func TestSwitchFullConnectivity(t *testing.T) {
	switches := MakeConnectedSwitches(t, cfg, 3, initSwitchFunc, Connect2Switches)
	defer func() {
		for _, sw := range switches {
			sw.Stop()
		}
	}()

	for i, sw := range switches {
		if sw.Peers().Size() != 2 {
			t.Fatalf("Expected each switch to be connected to 2 other, but %d switch only connected to %d", sw.Peers().Size(), i)
		}
	}
}

func BenchmarkSwitchBroadcast(b *testing.B) {
	s1, s2 := MakeSwitchPair(b, func(i int, sw *Switch) *Switch {
		// Make bar reactors of bar channels each
		sw.AddReactor("foo", NewTestReactor([]*conn.ChannelDescriptor{
			{ID: byte(0x00), Priority: 10},
			{ID: byte(0x01), Priority: 10},
		}, false))
		sw.AddReactor("bar", NewTestReactor([]*conn.ChannelDescriptor{
			{ID: byte(0x02), Priority: 10},
			{ID: byte(0x03), Priority: 10},
		}, false))
		return sw
	})
	defer s1.Stop()
	defer s2.Stop()

	// Allow time for goroutines to boot up
	time.Sleep(1 * time.Second)

	b.ResetTimer()

	numSuccess, numFailure := 0, 0

	// Send random message from foo channel to another
	for i := 0; i < b.N; i++ {
		chID := byte(i % 4)
		successChan := s1.Broadcast(chID, []byte("test data"))
		for s := range successChan {
			if s {
				numSuccess++
			} else {
				numFailure++
			}
		}
	}

	b.Logf("success: %v, failure: %v", numSuccess, numFailure)
}

type addrBookMock struct {
	addrs    map[string]struct{}
	ourAddrs map[string]struct{}
}

var _ AddrBook = (*addrBookMock)(nil)

func (book *addrBookMock) AddAddress(addr *NetAddress, src *NetAddress) error {
	book.addrs[addr.String()] = struct{}{}
	return nil
}
func (book *addrBookMock) AddOurAddress(addr *NetAddress) { book.ourAddrs[addr.String()] = struct{}{} }
func (book *addrBookMock) OurAddress(addr *NetAddress) bool {
	_, ok := book.ourAddrs[addr.String()]
	return ok
}
func (book *addrBookMock) MarkGood(*NetAddress) {}
func (book *addrBookMock) HasAddress(addr *NetAddress) bool {
	_, ok := book.addrs[addr.String()]
	return ok
}
func (book *addrBookMock) RemoveAddress(addr *NetAddress) {
	delete(book.addrs, addr.String())
}
func (book *addrBookMock) Save() {}
