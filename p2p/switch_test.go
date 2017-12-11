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
	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/tmlibs/log"

	cfg "github.com/tendermint/tendermint/config"
)

var (
	config *cfg.P2PConfig
)

func init() {
	config = cfg.DefaultP2PConfig()
	config.PexReactor = true
}

type PeerMessage struct {
	PeerKey string
	Bytes   []byte
	Counter int
}

type TestReactor struct {
	BaseReactor

	mtx          sync.Mutex
	channels     []*ChannelDescriptor
	peersAdded   []Peer
	peersRemoved []Peer
	logMessages  bool
	msgsCounter  int
	msgsReceived map[byte][]PeerMessage
}

func NewTestReactor(channels []*ChannelDescriptor, logMessages bool) *TestReactor {
	tr := &TestReactor{
		channels:     channels,
		logMessages:  logMessages,
		msgsReceived: make(map[byte][]PeerMessage),
	}
	tr.BaseReactor = *NewBaseReactor("TestReactor", tr)
	tr.SetLogger(log.TestingLogger())
	return tr
}

func (tr *TestReactor) GetChannels() []*ChannelDescriptor {
	return tr.channels
}

func (tr *TestReactor) AddPeer(peer Peer) {
	tr.mtx.Lock()
	defer tr.mtx.Unlock()
	tr.peersAdded = append(tr.peersAdded, peer)
}

func (tr *TestReactor) RemovePeer(peer Peer, reason interface{}) {
	tr.mtx.Lock()
	defer tr.mtx.Unlock()
	tr.peersRemoved = append(tr.peersRemoved, peer)
}

func (tr *TestReactor) Receive(chID byte, peer Peer, msgBytes []byte) {
	if tr.logMessages {
		tr.mtx.Lock()
		defer tr.mtx.Unlock()
		//fmt.Printf("Received: %X, %X\n", chID, msgBytes)
		tr.msgsReceived[chID] = append(tr.msgsReceived[chID], PeerMessage{peer.Key(), msgBytes, tr.msgsCounter})
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
func makeSwitchPair(t testing.TB, initSwitch func(int, *Switch) *Switch) (*Switch, *Switch) {
	// Create two switches that will be interconnected.
	switches := MakeConnectedSwitches(config, 2, initSwitch, Connect2Switches)
	return switches[0], switches[1]
}

func initSwitchFunc(i int, sw *Switch) *Switch {
	// Make two reactors of two channels each
	sw.AddReactor("foo", NewTestReactor([]*ChannelDescriptor{
		{ID: byte(0x00), Priority: 10},
		{ID: byte(0x01), Priority: 10},
	}, true))
	sw.AddReactor("bar", NewTestReactor([]*ChannelDescriptor{
		{ID: byte(0x02), Priority: 10},
		{ID: byte(0x03), Priority: 10},
	}, true))
	return sw
}

func TestSwitches(t *testing.T) {
	s1, s2 := makeSwitchPair(t, initSwitchFunc)
	defer s1.Stop()
	defer s2.Stop()

	if s1.Peers().Size() != 1 {
		t.Errorf("Expected exactly 1 peer in s1, got %v", s1.Peers().Size())
	}
	if s2.Peers().Size() != 1 {
		t.Errorf("Expected exactly 1 peer in s2, got %v", s2.Peers().Size())
	}

	// Lets send some messages
	ch0Msg := "channel zero"
	ch1Msg := "channel foo"
	ch2Msg := "channel bar"

	s1.Broadcast(byte(0x00), ch0Msg)
	s1.Broadcast(byte(0x01), ch1Msg)
	s1.Broadcast(byte(0x02), ch2Msg)

	assertMsgReceivedWithTimeout(t, ch0Msg, byte(0x00), s2.Reactor("foo").(*TestReactor), 10*time.Millisecond, 5*time.Second)
	assertMsgReceivedWithTimeout(t, ch1Msg, byte(0x01), s2.Reactor("foo").(*TestReactor), 10*time.Millisecond, 5*time.Second)
	assertMsgReceivedWithTimeout(t, ch2Msg, byte(0x02), s2.Reactor("bar").(*TestReactor), 10*time.Millisecond, 5*time.Second)
}

func assertMsgReceivedWithTimeout(t *testing.T, msg string, channel byte, reactor *TestReactor, checkPeriod, timeout time.Duration) {
	ticker := time.NewTicker(checkPeriod)
	for {
		select {
		case <-ticker.C:
			msgs := reactor.getMsgs(channel)
			if len(msgs) > 0 {
				if !bytes.Equal(msgs[0].Bytes, wire.BinaryBytes(msg)) {
					t.Fatalf("Unexpected message bytes. Wanted: %X, Got: %X", wire.BinaryBytes(msg), msgs[0].Bytes)
				}
				return
			}
		case <-time.After(timeout):
			t.Fatalf("Expected to have received 1 message in channel #%v, got zero", channel)
		}
	}
}

func TestConnAddrFilter(t *testing.T) {
	s1 := makeSwitch(config, 1, "testing", "123.123.123", initSwitchFunc)
	s2 := makeSwitch(config, 1, "testing", "123.123.123", initSwitchFunc)
	defer s1.Stop()
	defer s2.Stop()

	c1, c2 := netPipe()

	s1.SetAddrFilter(func(addr net.Addr) error {
		if addr.String() == c1.RemoteAddr().String() {
			return fmt.Errorf("Error: pipe is blacklisted")
		}
		return nil
	})

	// connect to good peer
	go func() {
		err := s1.addPeerWithConnection(c1)
		assert.NotNil(t, err, "expected err")
	}()
	go func() {
		err := s2.addPeerWithConnection(c2)
		assert.NotNil(t, err, "expected err")
	}()

	assertNoPeersAfterTimeout(t, s1, 400*time.Millisecond)
	assertNoPeersAfterTimeout(t, s2, 400*time.Millisecond)
}

func assertNoPeersAfterTimeout(t *testing.T, sw *Switch, timeout time.Duration) {
	time.Sleep(timeout)
	if sw.Peers().Size() != 0 {
		t.Fatalf("Expected %v to not connect to some peers, got %d", sw, sw.Peers().Size())
	}
}

func TestConnPubKeyFilter(t *testing.T) {
	s1 := makeSwitch(config, 1, "testing", "123.123.123", initSwitchFunc)
	s2 := makeSwitch(config, 1, "testing", "123.123.123", initSwitchFunc)
	defer s1.Stop()
	defer s2.Stop()

	c1, c2 := netPipe()

	// set pubkey filter
	s1.SetPubKeyFilter(func(pubkey crypto.PubKeyEd25519) error {
		if bytes.Equal(pubkey.Bytes(), s2.nodeInfo.PubKey.Bytes()) {
			return fmt.Errorf("Error: pipe is blacklisted")
		}
		return nil
	})

	// connect to good peer
	go func() {
		err := s1.addPeerWithConnection(c1)
		assert.NotNil(t, err, "expected error")
	}()
	go func() {
		err := s2.addPeerWithConnection(c2)
		assert.NotNil(t, err, "expected error")
	}()

	assertNoPeersAfterTimeout(t, s1, 400*time.Millisecond)
	assertNoPeersAfterTimeout(t, s2, 400*time.Millisecond)
}

func TestSwitchStopsNonPersistentPeerOnError(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	sw := makeSwitch(config, 1, "testing", "123.123.123", initSwitchFunc)
	err := sw.Start()
	if err != nil {
		t.Error(err)
	}
	defer sw.Stop()

	// simulate remote peer
	rp := &remotePeer{PrivKey: crypto.GenPrivKeyEd25519(), Config: DefaultPeerConfig()}
	rp.Start()
	defer rp.Stop()

	peer, err := newOutboundPeer(rp.Addr(), sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey, DefaultPeerConfig())
	require.Nil(err)
	err = sw.addPeer(peer)
	require.Nil(err)

	// simulate failure by closing connection
	peer.CloseConn()

	assertNoPeersAfterTimeout(t, sw, 100*time.Millisecond)
	assert.False(peer.IsRunning())
}

func TestSwitchReconnectsToPersistentPeer(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	sw := makeSwitch(config, 1, "testing", "123.123.123", initSwitchFunc)
	err := sw.Start()
	if err != nil {
		t.Error(err)
	}
	defer sw.Stop()

	// simulate remote peer
	rp := &remotePeer{PrivKey: crypto.GenPrivKeyEd25519(), Config: DefaultPeerConfig()}
	rp.Start()
	defer rp.Stop()

	peer, err := newOutboundPeer(rp.Addr(), sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey, DefaultPeerConfig())
	peer.makePersistent()
	require.Nil(err)
	err = sw.addPeer(peer)
	require.Nil(err)

	// simulate failure by closing connection
	peer.CloseConn()

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
}

func TestSwitchFullConnectivity(t *testing.T) {
	switches := MakeConnectedSwitches(config, 3, initSwitchFunc, Connect2Switches)
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

func BenchmarkSwitches(b *testing.B) {
	b.StopTimer()

	s1, s2 := makeSwitchPair(b, func(i int, sw *Switch) *Switch {
		// Make bar reactors of bar channels each
		sw.AddReactor("foo", NewTestReactor([]*ChannelDescriptor{
			{ID: byte(0x00), Priority: 10},
			{ID: byte(0x01), Priority: 10},
		}, false))
		sw.AddReactor("bar", NewTestReactor([]*ChannelDescriptor{
			{ID: byte(0x02), Priority: 10},
			{ID: byte(0x03), Priority: 10},
		}, false))
		return sw
	})
	defer s1.Stop()
	defer s2.Stop()

	// Allow time for goroutines to boot up
	time.Sleep(1 * time.Second)
	b.StartTimer()

	numSuccess, numFailure := 0, 0

	// Send random message from foo channel to another
	for i := 0; i < b.N; i++ {
		chID := byte(i % 4)
		successChan := s1.Broadcast(chID, "test data")
		for s := range successChan {
			if s {
				numSuccess++
			} else {
				numFailure++
			}
		}
	}

	b.Logf("success: %v, failure: %v", numSuccess, numFailure)

	// Allow everything to flush before stopping switches & closing connections.
	b.StopTimer()
}
