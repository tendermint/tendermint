package p2p

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p/conn"
)

var (
	cfg *config.P2PConfig
	ctx = context.Background()
)

func init() {
	cfg = config.DefaultP2PConfig()
	cfg.PexReactor = true
	cfg.AllowDuplicateIP = true
}

type PeerMessage struct {
	PeerID  NodeID
	Bytes   []byte
	Counter int
}

type TestReactor struct {
	BaseReactor

	mtx          tmsync.Mutex
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
		// fmt.Printf("Received: %X, %X\n", chID, msgBytes)
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
	sw.SetAddrBook(&AddrBookMock{
		Addrs:    make(map[string]struct{}),
		OurAddrs: make(map[string]struct{})})

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
	t.Cleanup(func() {
		if err := s1.Stop(); err != nil {
			t.Error(err)
		}
	})
	t.Cleanup(func() {
		if err := s2.Stop(); err != nil {
			t.Error(err)
		}
	})

	if s1.Peers().Size() != 1 {
		t.Errorf("expected exactly 1 peer in s1, got %v", s1.Peers().Size())
	}
	if s2.Peers().Size() != 1 {
		t.Errorf("expected exactly 1 peer in s2, got %v", s2.Peers().Size())
	}

	// Lets send some messages
	ch0Msg := []byte("channel zero")
	ch1Msg := []byte("channel foo")
	ch2Msg := []byte("channel bar")

	s1.Broadcast(byte(0x00), ch0Msg)
	s1.Broadcast(byte(0x01), ch1Msg)
	s1.Broadcast(byte(0x02), ch2Msg)

	assertMsgReceivedWithTimeout(t,
		ch0Msg,
		byte(0x00),
		s2.Reactor("foo").(*TestReactor), 10*time.Millisecond, 5*time.Second)
	assertMsgReceivedWithTimeout(t,
		ch1Msg,
		byte(0x01),
		s2.Reactor("foo").(*TestReactor), 10*time.Millisecond, 5*time.Second)
	assertMsgReceivedWithTimeout(t,
		ch2Msg,
		byte(0x02),
		s2.Reactor("bar").(*TestReactor), 10*time.Millisecond, 5*time.Second)
}

func assertMsgReceivedWithTimeout(
	t *testing.T,
	msgBytes []byte,
	channel byte,
	reactor *TestReactor,
	checkPeriod,
	timeout time.Duration,
) {
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
	s1 := MakeSwitch(cfg, 1, "127.0.0.1", "123.123.123", initSwitchFunc, log.TestingLogger())

	// simulate s1 having a public IP by creating a remote peer with the same ID
	rp := &remotePeer{PrivKey: s1.nodeKey.PrivKey, Config: cfg}
	rp.Start()

	// addr should be rejected in addPeer based on the same ID
	err := s1.DialPeerWithAddress(rp.Addr())
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
			func(_ IPeerSet, _ Peer) error { return fmt.Errorf("denied") },
			func(_ IPeerSet, _ Peer) error { return nil },
		}
		sw = MakeSwitch(
			cfg,
			1,
			"testing",
			"123.123.123",
			initSwitchFunc,
			log.TestingLogger(),
			SwitchPeerFilters(filters...),
		)
	)
	err := sw.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := sw.Stop(); err != nil {
			t.Error(err)
		}
	})

	// simulate remote peer
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	t.Cleanup(rp.Stop)

	c, err := sw.transport.Dial(ctx, rp.Addr().Endpoint())
	if err != nil {
		t.Fatal(err)
	}
	peerInfo, _, err := c.Handshake(ctx, sw.nodeInfo, sw.nodeKey.PrivKey)
	if err != nil {
		t.Fatal(err)
	}
	p := newPeer(
		peerInfo,
		newPeerConn(true, false, c),
		sw.reactorsByCh,
		sw.StopPeerForError,
	)

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
			log.TestingLogger(),
			SwitchFilterTimeout(5*time.Millisecond),
			SwitchPeerFilters(filters...),
		)
	)
	err := sw.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := sw.Stop(); err != nil {
			t.Log(err)
		}
	})

	// simulate remote peer
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	c, err := sw.transport.Dial(ctx, rp.Addr().Endpoint())
	if err != nil {
		t.Fatal(err)
	}
	peerInfo, _, err := c.Handshake(ctx, sw.nodeInfo, sw.nodeKey.PrivKey)
	if err != nil {
		t.Fatal(err)
	}
	p := newPeer(
		peerInfo,
		newPeerConn(true, false, c),
		sw.reactorsByCh,
		sw.StopPeerForError,
	)

	err = sw.addPeer(p)
	if _, ok := err.(ErrFilterTimeout); !ok {
		t.Errorf("expected ErrFilterTimeout")
	}
}

func TestSwitchPeerFilterDuplicate(t *testing.T) {
	sw := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc, log.TestingLogger())
	err := sw.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := sw.Stop(); err != nil {
			t.Error(err)
		}
	})

	// simulate remote peer
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	c, err := sw.transport.Dial(ctx, rp.Addr().Endpoint())
	if err != nil {
		t.Fatal(err)
	}
	peerInfo, _, err := c.Handshake(ctx, sw.nodeInfo, sw.nodeKey.PrivKey)
	if err != nil {
		t.Fatal(err)
	}
	p := newPeer(
		peerInfo,
		newPeerConn(true, false, c),
		sw.reactorsByCh,
		sw.StopPeerForError,
	)

	if err := sw.addPeer(p); err != nil {
		t.Fatal(err)
	}

	err = sw.addPeer(p)
	if errRej, ok := err.(ErrRejected); ok {
		if !errRej.IsDuplicate() {
			t.Errorf("expected peer to be duplicate. got %v", errRej)
		}
	} else {
		t.Errorf("expected ErrRejected, got %v", err)
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

	sw := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc, log.TestingLogger())
	err := sw.Start()
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(func() {
		if err := sw.Stop(); err != nil {
			t.Error(err)
		}
	})

	// simulate remote peer
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	c, err := sw.transport.Dial(ctx, rp.Addr().Endpoint())
	if err != nil {
		t.Fatal(err)
	}
	peerInfo, _, err := c.Handshake(ctx, sw.nodeInfo, sw.nodeKey.PrivKey)
	if err != nil {
		t.Fatal(err)
	}
	p := newPeer(
		peerInfo,
		newPeerConn(true, false, c),
		sw.reactorsByCh,
		sw.StopPeerForError,
	)

	err = sw.addPeer(p)
	require.Nil(err)

	require.NotNil(sw.Peers().Get(rp.ID()))

	// simulate failure by closing connection
	err = p.CloseConn()
	require.NoError(err)

	assertNoPeersAfterTimeout(t, sw, 100*time.Millisecond)
	assert.False(p.IsRunning())
}

func TestSwitchStopPeerForError(t *testing.T) {
	s := httptest.NewServer(promhttp.Handler())
	defer s.Close()

	scrapeMetrics := func() string {
		resp, err := http.Get(s.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
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
	t.Cleanup(func() {
		if err := sw2.Stop(); err != nil {
			t.Error(err)
		}
	})

	// now call StopPeerForError explicitly, eg. from a reactor
	sw1.StopPeerForError(p, fmt.Errorf("some err"))

	assert.Equal(t, len(sw1.Peers().List()), 0)
	assert.EqualValues(t, 0, peersMetricValue())
}

func TestSwitchReconnectsToOutboundPersistentPeer(t *testing.T) {
	sw := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc, log.TestingLogger())
	err := sw.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := sw.Stop(); err != nil {
			t.Error(err)
		}
	})

	// 1. simulate failure by closing connection
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	err = sw.AddPersistentPeers([]string{rp.Addr().String()})
	require.NoError(t, err)

	err = sw.DialPeerWithAddress(rp.Addr())
	require.Nil(t, err)
	require.NotNil(t, sw.Peers().Get(rp.ID()))

	p := sw.Peers().List()[0]
	err = p.(*peer).CloseConn()
	require.NoError(t, err)

	waitUntilSwitchHasAtLeastNPeers(sw, 1)
	assert.False(t, p.IsRunning())        // old peer instance
	assert.Equal(t, 1, sw.Peers().Size()) // new peer instance

	// 2. simulate first time dial failure
	rp = &remotePeer{
		PrivKey: ed25519.GenPrivKey(),
		Config:  cfg,
		// Use different interface to prevent duplicate IP filter, this will break
		// beyond two peers.
		listenAddr: "127.0.0.1:0",
	}
	rp.Start()
	defer rp.Stop()

	conf := config.DefaultP2PConfig()
	conf.TestDialFail = true // will trigger a reconnect
	err = sw.addOutboundPeerWithConfig(rp.Addr(), conf)
	require.NotNil(t, err)
	// DialPeerWithAddres - sw.peerConfig resets the dialer
	waitUntilSwitchHasAtLeastNPeers(sw, 2)
	assert.Equal(t, 2, sw.Peers().Size())
}

func TestSwitchReconnectsToInboundPersistentPeer(t *testing.T) {
	sw := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc, log.TestingLogger())
	err := sw.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := sw.Stop(); err != nil {
			t.Error(err)
		}
	})

	// 1. simulate failure by closing the connection
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	err = sw.AddPersistentPeers([]string{rp.Addr().String()})
	require.NoError(t, err)

	conn, err := rp.Dial(sw.NetAddress())
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	require.NotNil(t, sw.Peers().Get(rp.ID()))

	conn.Close()

	waitUntilSwitchHasAtLeastNPeers(sw, 1)
	assert.Equal(t, 1, sw.Peers().Size())
}

func TestSwitchDialPeersAsync(t *testing.T) {
	if testing.Short() {
		return
	}

	sw := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc, log.TestingLogger())
	err := sw.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := sw.Stop(); err != nil {
			t.Error(err)
		}
	})

	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	err = sw.DialPeersAsync([]string{rp.Addr().String()})
	require.NoError(t, err)
	time.Sleep(dialRandomizerIntervalMilliseconds * time.Millisecond)
	require.NotNil(t, sw.Peers().Get(rp.ID()))
}

func waitUntilSwitchHasAtLeastNPeers(sw *Switch, n int) {
	for i := 0; i < 20; i++ {
		time.Sleep(250 * time.Millisecond)
		has := sw.Peers().Size()
		if has >= n {
			break
		}
	}
}

func TestSwitchFullConnectivity(t *testing.T) {
	switches := MakeConnectedSwitches(cfg, 3, initSwitchFunc, Connect2Switches)
	defer func() {
		for _, sw := range switches {
			sw := sw
			t.Cleanup(func() {
				if err := sw.Stop(); err != nil {
					t.Error(err)
				}
			})
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

	// Create some unconditional peers.
	const unconditionalPeersNum = 2
	var (
		unconditionalPeers   = make([]*remotePeer, unconditionalPeersNum)
		unconditionalPeerIDs = make([]string, unconditionalPeersNum)
	)
	for i := 0; i < unconditionalPeersNum; i++ {
		peer := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
		peer.Start()
		unconditionalPeers[i] = peer
		unconditionalPeerIDs[i] = string(peer.ID())
	}

	// make switch
	sw := MakeSwitch(cfg, 1, "testing", "123.123.123", initSwitchFunc, log.TestingLogger())
	err := sw.AddUnconditionalPeerIDs(unconditionalPeerIDs)
	require.NoError(t, err)
	err = sw.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		err := sw.Stop()
		require.NoError(t, err)
	})

	// 0. check there are no peers
	assert.Equal(t, 0, sw.Peers().Size())

	// 1. check we connect up to MaxNumInboundPeers
	peers := make([]*remotePeer, 0)
	for i := 0; i < cfg.MaxNumInboundPeers; i++ {
		peer := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
		peers = append(peers, peer)
		peer.Start()
		c, err := peer.Dial(sw.NetAddress())
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
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, cfg.MaxNumInboundPeers, sw.Peers().Size())

	// 2. check we close new connections if we already have MaxNumInboundPeers peers
	peer := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	peer.Start()
	conn, err := peer.Dial(sw.NetAddress())
	require.NoError(t, err)
	// check conn is closed
	one := make([]byte, 1)
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	_, err = conn.Read(one)
	assert.Error(t, err)
	assert.Equal(t, cfg.MaxNumInboundPeers, sw.Peers().Size())
	peer.Stop()

	// 3. check we connect to unconditional peers despite the limit.
	for _, peer := range unconditionalPeers {
		c, err := peer.Dial(sw.NetAddress())
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
	assert.Equal(t, cfg.MaxNumInboundPeers+unconditionalPeersNum, sw.Peers().Size())

	for _, peer := range peers {
		peer.Stop()
	}
	for _, peer := range unconditionalPeers {
		peer.Stop()
	}
}

type errorTransport struct {
	acceptErr error
}

func (et errorTransport) String() string {
	return "error"
}

func (et errorTransport) Protocols() []Protocol {
	return []Protocol{"error"}
}

func (et errorTransport) Accept() (Connection, error) {
	return nil, et.acceptErr
}
func (errorTransport) Dial(context.Context, Endpoint) (Connection, error) {
	panic("not implemented")
}
func (errorTransport) Close() error          { panic("not implemented") }
func (errorTransport) FlushClose() error     { panic("not implemented") }
func (errorTransport) Endpoints() []Endpoint { panic("not implemented") }

func TestSwitchAcceptRoutineErrorCases(t *testing.T) {
	sw := NewSwitch(cfg, errorTransport{ErrFilterTimeout{}})
	assert.NotPanics(t, func() {
		err := sw.Start()
		require.NoError(t, err)
		err = sw.Stop()
		require.NoError(t, err)
	})

	sw = NewSwitch(cfg, errorTransport{ErrRejected{conn: nil, err: errors.New("filtered"), isFiltered: true}})
	assert.NotPanics(t, func() {
		err := sw.Start()
		require.NoError(t, err)
		err = sw.Stop()
		require.NoError(t, err)
	})
	// TODO(melekes) check we remove our address from addrBook

	sw = NewSwitch(cfg, errorTransport{ErrTransportClosed{}})
	assert.NotPanics(t, func() {
		err := sw.Start()
		require.NoError(t, err)
		err = sw.Stop()
		require.NoError(t, err)
	})
}

// mockReactor checks that InitPeer never called before RemovePeer. If that's
// not true, InitCalledBeforeRemoveFinished will return true.
type mockReactor struct {
	*BaseReactor

	// atomic
	removePeerInProgress           uint32
	initCalledBeforeRemoveFinished uint32
}

func (r *mockReactor) GetChannels() []*ChannelDescriptor {
	return []*ChannelDescriptor{{ID: testCh, Priority: 10}}
}

func (r *mockReactor) RemovePeer(peer Peer, reason interface{}) {
	atomic.StoreUint32(&r.removePeerInProgress, 1)
	defer atomic.StoreUint32(&r.removePeerInProgress, 0)
	time.Sleep(100 * time.Millisecond)
}

func (r *mockReactor) InitPeer(peer Peer) Peer {
	if atomic.LoadUint32(&r.removePeerInProgress) == 1 {
		atomic.StoreUint32(&r.initCalledBeforeRemoveFinished, 1)
	}

	return peer
}

func (r *mockReactor) InitCalledBeforeRemoveFinished() bool {
	return atomic.LoadUint32(&r.initCalledBeforeRemoveFinished) == 1
}

// see stopAndRemovePeer
func TestSwitchInitPeerIsNotCalledBeforeRemovePeer(t *testing.T) {
	// make reactor
	reactor := &mockReactor{}
	reactor.BaseReactor = NewBaseReactor("mockReactor", reactor)

	// make switch
	sw := MakeSwitch(cfg, 1, "testing", "123.123.123", func(i int, sw *Switch) *Switch {
		sw.AddReactor("mock", reactor)
		return sw
	}, log.TestingLogger())
	err := sw.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := sw.Stop(); err != nil {
			t.Error(err)
		}
	})

	// add peer
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()
	_, err = rp.Dial(sw.NetAddress())
	require.NoError(t, err)

	// wait till the switch adds rp to the peer set, then stop the peer asynchronously
	for {
		time.Sleep(20 * time.Millisecond)
		if peer := sw.Peers().Get(rp.ID()); peer != nil {
			go sw.StopPeerForError(peer, "test")
			break
		}
	}

	// simulate peer reconnecting to us
	_, err = rp.Dial(sw.NetAddress())
	require.NoError(t, err)
	// wait till the switch adds rp to the peer set
	time.Sleep(50 * time.Millisecond)

	// make sure reactor.RemovePeer is finished before InitPeer is called
	assert.False(t, reactor.InitCalledBeforeRemoveFinished())
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

	b.Cleanup(func() {
		if err := s1.Stop(); err != nil {
			b.Error(err)
		}
	})

	b.Cleanup(func() {
		if err := s2.Stop(); err != nil {
			b.Error(err)
		}
	})

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
