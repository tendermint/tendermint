package p2p

import (
	"encoding/hex"
	"testing"
	"time"

	. "github.com/tendermint/tendermint/binary"
)

// convenience method for creating two switches connected to each other.
func makeSwitchPair(t testing.TB, numChannels int, sendQueueCapacity int, recvBufferSize int, recvQueueCapacity int) (*Switch, *Switch, []*ChannelDescriptor) {

	// Make numChannels channels starting at byte(0x00)
	chIds := []byte{}
	for i := 0; i < numChannels; i++ {
		chIds = append(chIds, byte(i))
	}

	// Make some channel descriptors.
	chDescs := []*ChannelDescriptor{}
	for _, chId := range chIds {
		chDescs = append(chDescs, &ChannelDescriptor{
			Id:                chId,
			SendQueueCapacity: sendQueueCapacity,
			RecvBufferSize:    recvBufferSize,
			RecvQueueCapacity: recvQueueCapacity,
			DefaultPriority:   1,
		})
	}

	// Create two switches that will be interconnected.
	s1 := NewSwitch(chDescs)
	s2 := NewSwitch(chDescs)

	// Create a listener for s1
	l := NewDefaultListener("tcp", ":8001")

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

	return s1, s2, chDescs
}

func TestSwitches(t *testing.T) {
	s1, s2, _ := makeSwitchPair(t, 10, 10, 1024, 10)
	defer s1.Stop()
	defer s2.Stop()

	// Lets send a message from s1 to s2.
	if s1.Peers().Size() != 1 {
		t.Errorf("Expected exactly 1 peer in s1, got %v", s1.Peers().Size())
	}
	if s2.Peers().Size() != 1 {
		t.Errorf("Expected exactly 1 peer in s2, got %v", s2.Peers().Size())
	}

	// Broadcast a message on ch0
	s1.Broadcast(byte(0x00), String("channel zero"))
	// Broadcast a message on ch1
	s1.Broadcast(byte(0x01), String("channel one"))
	// Broadcast a message on ch2
	s1.Broadcast(byte(0x02), String("channel two"))

	// Wait for things to settle...
	time.Sleep(100 * time.Millisecond)

	// Receive message from channel 1 and check
	inMsg, ok := s2.Receive(byte(0x01))
	if !ok {
		t.Errorf("Failed to receive from channel one")
	}
	if ReadString(inMsg.Bytes.Reader()) != "channel one" {
		t.Errorf("Unexpected received message bytes:\n%v", hex.Dump(inMsg.Bytes))
	}

	// Receive message from channel 0 and check
	inMsg, ok = s2.Receive(byte(0x00))
	if !ok {
		t.Errorf("Failed to receive from channel zero")
	}
	if ReadString(inMsg.Bytes.Reader()) != "channel zero" {
		t.Errorf("Unexpected received message bytes:\n%v", hex.Dump(inMsg.Bytes))
	}
}

func BenchmarkSwitches(b *testing.B) {

	b.StopTimer()

	s1, s2, chDescs := makeSwitchPair(b, 10, 10, 1024, 10)
	defer s1.Stop()
	defer s2.Stop()

	// Create a sink on either channel to just pop off messages.
	recvRoutine := func(c *Switch, chId byte) {
		for {
			_, ok := c.Receive(chId)
			if !ok {
				break
			}
		}
	}

	// Create routines to consume from recvQueues.
	for _, chDesc := range chDescs {
		go recvRoutine(s1, chDesc.Id)
		go recvRoutine(s2, chDesc.Id)
	}

	// Allow time for goroutines to boot up
	time.Sleep(1000 * time.Millisecond)
	b.StartTimer()

	numSuccess, numFailure := 0, 0

	// Send random message from one channel to another
	for i := 0; i < b.N; i++ {
		chId := chDescs[i%len(chDescs)].Id
		nS, nF := s1.Broadcast(chId, String("test data"))
		numSuccess += nS
		numFailure += nF
	}

	log.Warning("success: %v, failure: %v", numSuccess, numFailure)

	// Allow everything to flush before stopping switches & closing connections.
	b.StopTimer()
	time.Sleep(1000 * time.Millisecond)

}
