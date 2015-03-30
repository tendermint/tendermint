package p2p

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

type PeerMessage struct {
	PeerKey string
	Bytes   []byte
	Counter int
}

type TestReactor struct {
	mtx          sync.Mutex
	channels     []*ChannelDescriptor
	peersAdded   []*Peer
	peersRemoved []*Peer
	logMessages  bool
	msgsCounter  int
	msgsReceived map[byte][]PeerMessage
}

func NewTestReactor(channels []*ChannelDescriptor, logMessages bool) *TestReactor {
	return &TestReactor{
		channels:     channels,
		logMessages:  logMessages,
		msgsReceived: make(map[byte][]PeerMessage),
	}
}

func (tr *TestReactor) Start(sw *Switch) {
}

func (tr *TestReactor) Stop() {
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

func (tr *TestReactor) Receive(chId byte, peer *Peer, msgBytes []byte) {
	if tr.logMessages {
		tr.mtx.Lock()
		defer tr.mtx.Unlock()
		//fmt.Printf("Received: %X, %X\n", chId, msgBytes)
		tr.msgsReceived[chId] = append(tr.msgsReceived[chId], PeerMessage{peer.Key, msgBytes, tr.msgsCounter})
		tr.msgsCounter++
	}
}

//-----------------------------------------------------------------------------

// convenience method for creating bar switches connected to each other.
func makeSwitchPair(t testing.TB, initSwitch func(*Switch) *Switch) (*Switch, *Switch) {

	// Create bar switches that will be interconnected.
	s1 := initSwitch(NewSwitch())
	s2 := initSwitch(NewSwitch())

	// Create a listener for s1
	l := NewDefaultListener("tcp", ":8001", true)

	// Dial the listener & add the connection to s2.
	lAddr := l.ExternalAddress()
	connOut, err := lAddr.Dial()
	if err != nil {
		t.Fatalf("Could not connect to listener address %v", lAddr)
	} else {
		t.Logf("Created a connection to listener address %v", lAddr)
	}
	connIn, ok := <-l.Connections()
	if !ok {
		t.Fatalf("Could not get inbound connection from listener")
	}

	s1.AddPeerWithConnection(connIn, false)
	s2.AddPeerWithConnection(connOut, true)

	// Wait for things to happen, peers to get added...
	time.Sleep(100 * time.Millisecond)

	// Close the server, no longer needed.
	l.Stop()

	return s1, s2
}

func TestSwitches(t *testing.T) {
	s1, s2 := makeSwitchPair(t, func(sw *Switch) *Switch {
		// Make bar reactors of bar channels each
		sw.AddReactor("foo", NewTestReactor([]*ChannelDescriptor{
			&ChannelDescriptor{Id: byte(0x00), Priority: 10},
			&ChannelDescriptor{Id: byte(0x01), Priority: 10},
		}, true)).Start(sw) // Start the reactor
		sw.AddReactor("bar", NewTestReactor([]*ChannelDescriptor{
			&ChannelDescriptor{Id: byte(0x02), Priority: 10},
			&ChannelDescriptor{Id: byte(0x03), Priority: 10},
		}, true)).Start(sw) // Start the reactor
		return sw
	})
	defer s1.Stop()
	defer s2.Stop()

	// Lets send a message from s1 to s2.
	if s1.Peers().Size() != 1 {
		t.Errorf("Expected exactly 1 peer in s1, got %v", s1.Peers().Size())
	}
	if s2.Peers().Size() != 1 {
		t.Errorf("Expected exactly 1 peer in s2, got %v", s2.Peers().Size())
	}

	ch0Msg := "channel zero"
	ch1Msg := "channel foo"
	ch2Msg := "channel bar"

	s1.Broadcast(byte(0x00), ch0Msg)
	s1.Broadcast(byte(0x01), ch1Msg)
	s1.Broadcast(byte(0x02), ch2Msg)

	// Wait for things to settle...
	time.Sleep(5000 * time.Millisecond)

	// Check message on ch0
	ch0Msgs := s2.Reactor("foo").(*TestReactor).msgsReceived[byte(0x00)]
	if len(ch0Msgs) != 2 {
		t.Errorf("Expected to have received 1 message in ch0")
	}
	if !bytes.Equal(ch0Msgs[1].Bytes, binary.BinaryBytes(ch0Msg)) {
		t.Errorf("Unexpected message bytes. Wanted: %X, Got: %X", binary.BinaryBytes(ch0Msg), ch0Msgs[0].Bytes)
	}

	// Check message on ch1
	ch1Msgs := s2.Reactor("foo").(*TestReactor).msgsReceived[byte(0x01)]
	if len(ch1Msgs) != 1 {
		t.Errorf("Expected to have received 1 message in ch1")
	}
	if !bytes.Equal(ch1Msgs[0].Bytes, binary.BinaryBytes(ch1Msg)) {
		t.Errorf("Unexpected message bytes. Wanted: %X, Got: %X", binary.BinaryBytes(ch1Msg), ch1Msgs[0].Bytes)
	}

	// Check message on ch2
	ch2Msgs := s2.Reactor("bar").(*TestReactor).msgsReceived[byte(0x02)]
	if len(ch2Msgs) != 1 {
		t.Errorf("Expected to have received 1 message in ch2")
	}
	if !bytes.Equal(ch2Msgs[0].Bytes, binary.BinaryBytes(ch2Msg)) {
		t.Errorf("Unexpected message bytes. Wanted: %X, Got: %X", binary.BinaryBytes(ch2Msg), ch2Msgs[0].Bytes)
	}

}

func BenchmarkSwitches(b *testing.B) {

	b.StopTimer()

	s1, s2 := makeSwitchPair(b, func(sw *Switch) *Switch {
		// Make bar reactors of bar channels each
		sw.AddReactor("foo", NewTestReactor([]*ChannelDescriptor{
			&ChannelDescriptor{Id: byte(0x00), Priority: 10},
			&ChannelDescriptor{Id: byte(0x01), Priority: 10},
		}, false))
		sw.AddReactor("bar", NewTestReactor([]*ChannelDescriptor{
			&ChannelDescriptor{Id: byte(0x02), Priority: 10},
			&ChannelDescriptor{Id: byte(0x03), Priority: 10},
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
		chId := byte(i % 4)
		successChan := s1.Broadcast(chId, "test data")
		for s := range successChan {
			if s {
				numSuccess += 1
			} else {
				numFailure += 1
			}
		}
	}

	log.Warn(Fmt("success: %v, failure: %v", numSuccess, numFailure))

	// Allow everything to flush before stopping switches & closing connections.
	b.StopTimer()
	time.Sleep(1000 * time.Millisecond)

}
