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

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tmlibs/log"
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
	peersAdded   []*Peer
	peersRemoved []*Peer
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

func (tr *TestReactor) AddPeer(peer *Peer) {
	tr.mtx.Lock()
	defer tr.mtx.Unlock()
	tr.peersAdded = append(tr.peersAdded, peer)
}

func (tr *TestReactor) RemovePeer(peer *Peer, reason interface{}) {
	tr.mtx.Lock()
	defer tr.mtx.Unlock()
	tr.peersRemoved = append(tr.peersRemoved, peer)
}

func (tr *TestReactor) Receive(chID byte, peer *Peer, msgBytes []byte) {
	if tr.logMessages {
		tr.mtx.Lock()
		defer tr.mtx.Unlock()
		//fmt.Printf("Received: %X, %X\n", chID, msgBytes)
		tr.msgsReceived[chID] = append(tr.msgsReceived[chID], PeerMessage{peer.Key, msgBytes, tr.msgsCounter})
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
		&ChannelDescriptor{ID: byte(0x00), Priority: 10},
		&ChannelDescriptor{ID: byte(0x01), Priority: 10},
	}, true))
	sw.AddReactor("bar", NewTestReactor([]*ChannelDescriptor{
		&ChannelDescriptor{ID: byte(0x02), Priority: 10},
		&ChannelDescriptor{ID: byte(0x03), Priority: 10},
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

	// Wait for things to settle...
	time.Sleep(5000 * time.Millisecond)

	// Check message on ch0
	ch0Msgs := s2.Reactor("foo").(*TestReactor).getMsgs(byte(0x00))
	if len(ch0Msgs) != 1 {
		t.Errorf("Expected to have received 1 message in ch0")
	}
	if !bytes.Equal(ch0Msgs[0].Bytes, wire.BinaryBytes(ch0Msg)) {
		t.Errorf("Unexpected message bytes. Wanted: %X, Got: %X", wire.BinaryBytes(ch0Msg), ch0Msgs[0].Bytes)
	}

	// Check message on ch1
	ch1Msgs := s2.Reactor("foo").(*TestReactor).getMsgs(byte(0x01))
	if len(ch1Msgs) != 1 {
		t.Errorf("Expected to have received 1 message in ch1")
	}
	if !bytes.Equal(ch1Msgs[0].Bytes, wire.BinaryBytes(ch1Msg)) {
		t.Errorf("Unexpected message bytes. Wanted: %X, Got: %X", wire.BinaryBytes(ch1Msg), ch1Msgs[0].Bytes)
	}

	// Check message on ch2
	ch2Msgs := s2.Reactor("bar").(*TestReactor).getMsgs(byte(0x02))
	if len(ch2Msgs) != 1 {
		t.Errorf("Expected to have received 1 message in ch2")
	}
	if !bytes.Equal(ch2Msgs[0].Bytes, wire.BinaryBytes(ch2Msg)) {
		t.Errorf("Unexpected message bytes. Wanted: %X, Got: %X", wire.BinaryBytes(ch2Msg), ch2Msgs[0].Bytes)
	}

}

func TestConnAddrFilter(t *testing.T) {
	s1 := makeSwitch(config, 1, "testing", "123.123.123", initSwitchFunc)
	s2 := makeSwitch(config, 1, "testing", "123.123.123", initSwitchFunc)

	c1, c2 := net.Pipe()

	s1.SetAddrFilter(func(addr net.Addr) error {
		if addr.String() == c1.RemoteAddr().String() {
			return fmt.Errorf("Error: pipe is blacklisted")
		}
		return nil
	})

	// connect to good peer
	go func() {
		s1.addPeerWithConnection(c1)
	}()
	go func() {
		s2.addPeerWithConnection(c2)
	}()

	// Wait for things to happen, peers to get added...
	time.Sleep(100 * time.Millisecond * time.Duration(4))

	defer s1.Stop()
	defer s2.Stop()
	if s1.Peers().Size() != 0 {
		t.Errorf("Expected s1 not to connect to peers, got %d", s1.Peers().Size())
	}
	if s2.Peers().Size() != 0 {
		t.Errorf("Expected s2 not to connect to peers, got %d", s2.Peers().Size())
	}
}

func TestConnPubKeyFilter(t *testing.T) {
	s1 := makeSwitch(config, 1, "testing", "123.123.123", initSwitchFunc)
	s2 := makeSwitch(config, 1, "testing", "123.123.123", initSwitchFunc)

	c1, c2 := net.Pipe()

	// set pubkey filter
	s1.SetPubKeyFilter(func(pubkey crypto.PubKeyEd25519) error {
		if bytes.Equal(pubkey.Bytes(), s2.nodeInfo.PubKey.Bytes()) {
			return fmt.Errorf("Error: pipe is blacklisted")
		}
		return nil
	})

	// connect to good peer
	go func() {
		s1.addPeerWithConnection(c1)
	}()
	go func() {
		s2.addPeerWithConnection(c2)
	}()

	// Wait for things to happen, peers to get added...
	time.Sleep(100 * time.Millisecond * time.Duration(4))

	defer s1.Stop()
	defer s2.Stop()
	if s1.Peers().Size() != 0 {
		t.Errorf("Expected s1 not to connect to peers, got %d", s1.Peers().Size())
	}
	if s2.Peers().Size() != 0 {
		t.Errorf("Expected s2 not to connect to peers, got %d", s2.Peers().Size())
	}
}

func TestSwitchStopsNonPersistentPeerOnError(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	sw := makeSwitch(config, 1, "testing", "123.123.123", initSwitchFunc)
	sw.Start()
	defer sw.Stop()

	// simulate remote peer
	rp := &remotePeer{PrivKey: crypto.GenPrivKeyEd25519(), Config: DefaultPeerConfig()}
	rp.Start()
	defer rp.Stop()

	peer, err := newOutboundPeer(rp.Addr(), sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey)
	require.Nil(err)
	err = sw.AddPeer(peer)
	require.Nil(err)

	// simulate failure by closing connection
	peer.CloseConn()

	time.Sleep(100 * time.Millisecond)

	assert.Zero(sw.Peers().Size())
	assert.False(peer.IsRunning())
}

func TestSwitchReconnectsToPersistentPeer(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	sw := makeSwitch(config, 1, "testing", "123.123.123", initSwitchFunc)
	sw.Start()
	defer sw.Stop()

	// simulate remote peer
	rp := &remotePeer{PrivKey: crypto.GenPrivKeyEd25519(), Config: DefaultPeerConfig()}
	rp.Start()
	defer rp.Stop()

	peer, err := newOutboundPeer(rp.Addr(), sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey)
	peer.makePersistent()
	require.Nil(err)
	err = sw.AddPeer(peer)
	require.Nil(err)

	// simulate failure by closing connection
	peer.CloseConn()

	// TODO: actually detect the disconnection and wait for reconnect
	time.Sleep(100 * time.Millisecond)

	assert.NotZero(sw.Peers().Size())
	assert.False(peer.IsRunning())
}

func BenchmarkSwitches(b *testing.B) {
	b.StopTimer()

	s1, s2 := makeSwitchPair(b, func(i int, sw *Switch) *Switch {
		// Make bar reactors of bar channels each
		sw.AddReactor("foo", NewTestReactor([]*ChannelDescriptor{
			&ChannelDescriptor{ID: byte(0x00), Priority: 10},
			&ChannelDescriptor{ID: byte(0x01), Priority: 10},
		}, false))
		sw.AddReactor("bar", NewTestReactor([]*ChannelDescriptor{
			&ChannelDescriptor{ID: byte(0x02), Priority: 10},
			&ChannelDescriptor{ID: byte(0x03), Priority: 10},
		}, false))
		return sw
	})
	defer s1.Stop()
	defer s2.Stop()

	// Allow time for goroutines to boot up
	time.Sleep(1000 * time.Millisecond)
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
	time.Sleep(1000 * time.Millisecond)
}
