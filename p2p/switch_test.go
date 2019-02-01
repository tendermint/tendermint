package p2p

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
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
func MakeSwitchPair(t testing.TB, initSwitch func(int, *Switch) *Switch) (*Switch, *Switch) {
	// Create two switches that will be interconnected.
	switches := MakeConnectedSwitches(cfg, 2, initSwitch, Connect2Switches)
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
		if err, ok := err.(ErrRejected); ok {
			if !err.IsSelf() {
				t.Errorf("expected self to be rejected")
			}
		} else {
			t.Errorf("expected ErrRejected")
		}
	}

	assert.True(t, s1.addrBook.OurAddress(rp.Addr()))
	assert.False(t, s1.addrBook.HasAddress(rp.Addr()))

	rp.Stop()

	assertNoPeersAfterTimeout(t, s1, 100*time.Millisecond)
}

func TestSwitchPeerFilter(t *testing.T) {
	var (
		filters = []PeerFilterFunc{
			func(_ IPeerSet, _ Peer) error { return nil },
			func(_ IPeerSet, _ Peer) error { return fmt.Errorf("denied!") },
			func(_ IPeerSet, _ Peer) error { return nil },
		}
		sw = MakeSwitch(
			cfg,
			1,
			"testing",
			"123.123.123",
			initSwitchFunc,
			SwitchPeerFilters(filters...),
		)
	)
	defer sw.Stop()

	// simulate remote peer
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	p, err := sw.transport.Dial(*rp.Addr(), peerConfig{
		chDescs:      sw.chDescs,
		onPeerError:  sw.StopPeerForError,
		reactorsByCh: sw.reactorsByCh,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = sw.addPeer(p)
	if err, ok := err.(ErrRejected); ok {
		if !err.IsFiltered() {
			t.Errorf("expected peer to be filtered")
		}
	} else {
		t.Errorf("expected ErrRejected")
	}
}

func TestSwitchPeerFilterTimeout(t *testing.T) {
	var (
		filters = []PeerFilterFunc{
			func(_ IPeerSet, _ Peer) error {
				time.Sleep(10 * time.Millisecond)
				return nil
			},
		}
		sw = MakeSwitch(
			cfg,
			1,
			"testing",
			"123.123.123",
			initSwitchFunc,
			SwitchFilterTimeout(5*time.Millisecond),
			SwitchPeerFilters(filters...),
		)
	)
	defer sw.Stop()

	// simulate remote peer
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	p, err := sw.transport.Dial(*rp.Addr(), peerConfig{
		chDescs:      sw.chDescs,
		onPeerError:  sw.StopPeerForError,
		reactorsByCh: sw.reactorsByCh,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = sw.addPeer(p)
	if _, ok := err.(ErrFilterTimeout); !ok {
		t.Errorf("expected ErrFilterTimeout")
	}
}

func TestSwitchPeerFilterDuplicate(t *testing.T) {
	sw := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc)

	// simulate remote peer
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	p, err := sw.transport.Dial(*rp.Addr(), peerConfig{
		chDescs:      sw.chDescs,
		onPeerError:  sw.StopPeerForError,
		reactorsByCh: sw.reactorsByCh,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sw.addPeer(p); err != nil {
		t.Fatal(err)
	}

	err = sw.addPeer(p)
	if err, ok := err.(ErrRejected); ok {
		if !err.IsDuplicate() {
			t.Errorf("expected peer to be duplicate")
		}
	} else {
		t.Errorf("expected ErrRejected")
	}
}

func assertNoPeersAfterTimeout(t *testing.T, sw *Switch, timeout time.Duration) {
	time.Sleep(timeout)
	if sw.Peers().Size() != 0 {
		t.Fatalf("Expected %v to not connect to some peers, got %d", sw, sw.Peers().Size())
	}
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
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	p, err := sw.transport.Dial(*rp.Addr(), peerConfig{
		chDescs:      sw.chDescs,
		onPeerError:  sw.StopPeerForError,
		reactorsByCh: sw.reactorsByCh,
	})
	require.Nil(err)

	err = sw.addPeer(p)
	require.Nil(err)

	require.NotNil(sw.Peers().Get(rp.ID()))

	// simulate failure by closing connection
	p.(*peer).CloseConn()

	assertNoPeersAfterTimeout(t, sw, 100*time.Millisecond)
	assert.False(p.IsRunning())
}

func TestSwitchStopPeerForError(t *testing.T) {
	s := httptest.NewServer(stdprometheus.UninstrumentedHandler())
	defer s.Close()

	scrapeMetrics := func() string {
		resp, _ := http.Get(s.URL)
		buf, _ := ioutil.ReadAll(resp.Body)
		return string(buf)
	}

	namespace, subsystem, name := config.TestInstrumentationConfig().Namespace, MetricsSubsystem, "peers"
	re := regexp.MustCompile(namespace + `_` + subsystem + `_` + name + ` ([0-9\.]+)`)
	peersMetricValue := func() float64 {
		matches := re.FindStringSubmatch(scrapeMetrics())
		f, _ := strconv.ParseFloat(matches[1], 64)
		return f
	}

	p2pMetrics := PrometheusMetrics(namespace)

	// make two connected switches
	sw1, sw2 := MakeSwitchPair(t, func(i int, sw *Switch) *Switch {
		// set metrics on sw1
		if i == 0 {
			opt := WithMetrics(p2pMetrics)
			opt(sw)
		}
		return initSwitchFunc(i, sw)
	})

	assert.Equal(t, len(sw1.Peers().List()), 1)
	assert.EqualValues(t, 1, peersMetricValue())

	// send messages to the peer from sw1
	p := sw1.Peers().List()[0]
	p.Send(0x1, []byte("here's a message to send"))

	// stop sw2. this should cause the p to fail,
	// which results in calling StopPeerForError internally
	sw2.Stop()

	// now call StopPeerForError explicitly, eg. from a reactor
	sw1.StopPeerForError(p, fmt.Errorf("some err"))

	assert.Equal(t, len(sw1.Peers().List()), 0)
	assert.EqualValues(t, 0, peersMetricValue())
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
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	p, err := sw.transport.Dial(*rp.Addr(), peerConfig{
		chDescs:      sw.chDescs,
		onPeerError:  sw.StopPeerForError,
		persistent:   true,
		reactorsByCh: sw.reactorsByCh,
	})
	require.Nil(err)

	require.Nil(sw.addPeer(p))

	require.NotNil(sw.Peers().Get(rp.ID()))

	// simulate failure by closing connection
	p.(*peer).CloseConn()

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
	assert.False(p.IsRunning())

	// simulate another remote peer
	rp = &remotePeer{
		PrivKey: ed25519.GenPrivKey(),
		Config:  cfg,
		// Use different interface to prevent duplicate IP filter, this will break
		// beyond two peers.
		listenAddr: "127.0.0.1:0",
	}
	rp.Start()
	defer rp.Stop()

	// simulate first time dial failure
	conf := config.DefaultP2PConfig()
	conf.TestDialFail = true
	err = sw.addOutboundPeerWithConfig(rp.Addr(), conf, true)
	require.NotNil(err)

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
	switches := MakeConnectedSwitches(cfg, 3, initSwitchFunc, Connect2Switches)
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

func TestSwitchAcceptRoutine(t *testing.T) {
	cfg.MaxNumInboundPeers = 5

	// make switch
	sw := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc)
	err := sw.Start()
	require.NoError(t, err)
	defer sw.Stop()

	remotePeers := make([]*remotePeer, 0)
	assert.Equal(t, 0, sw.Peers().Size())

	// 1. check we connect up to MaxNumInboundPeers
	for i := 0; i < cfg.MaxNumInboundPeers; i++ {
		rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
		remotePeers = append(remotePeers, rp)
		rp.Start()
		c, err := rp.Dial(sw.NodeInfo().NetAddress())
		require.NoError(t, err)
		// spawn a reading routine to prevent connection from closing
		go func(c net.Conn) {
			for {
				one := make([]byte, 1)
				_, err := c.Read(one)
				if err != nil {
					return
				}
			}
		}(c)
	}
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, cfg.MaxNumInboundPeers, sw.Peers().Size())

	// 2. check we close new connections if we already have MaxNumInboundPeers peers
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	conn, err := rp.Dial(sw.NodeInfo().NetAddress())
	require.NoError(t, err)
	// check conn is closed
	one := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	_, err = conn.Read(one)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, cfg.MaxNumInboundPeers, sw.Peers().Size())
	rp.Stop()

	// stop remote peers
	for _, rp := range remotePeers {
		rp.Stop()
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
