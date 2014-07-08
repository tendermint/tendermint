package p2p

import (
	"testing"
	"time"

	. "github.com/tendermint/tendermint/binary"
)

// convenience method for creating two switches connected to each other.
func makeSwitchPair(t testing.TB, bufferSize int, chNames []string) (*Switch, *Switch) {

	chDescs := []ChannelDescriptor{}
	for _, chName := range chNames {
		chDescs = append(chDescs, ChannelDescriptor{
			Name:           chName,
			SendBufferSize: bufferSize,
			RecvBufferSize: bufferSize,
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
		t.Fatalf("Could not get incoming connection from listener")
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

	channels := []string{"ch1", "ch2", "ch3", "ch4", "ch5", "ch6", "ch7", "ch8", "ch9", "ch0"}
	s1, s2 := makeSwitchPair(t, 10, channels)
	defer s1.Stop()
	defer s2.Stop()

	// Lets send a message from s1 to s2.
	if s1.Peers().Size() != 1 {
		t.Errorf("Expected exactly 1 peer in s1, got %v", s1.Peers().Size())
	}
	if s2.Peers().Size() != 1 {
		t.Errorf("Expected exactly 1 peer in s2, got %v", s2.Peers().Size())
	}

	// Broadcast a message on ch1
	s1.Broadcast(NewPacket("ch1", ByteSlice("channel one")))
	// Broadcast a message on ch2
	s1.Broadcast(NewPacket("ch2", ByteSlice("channel two")))
	// Broadcast a message on ch3
	s1.Broadcast(NewPacket("ch3", ByteSlice("channel three")))

	// Wait for things to settle...
	time.Sleep(100 * time.Millisecond)

	// Receive message from channel 2 and check
	inMsg := s2.Receive("ch2")
	if string(inMsg.Bytes) != "channel two" {
		t.Errorf("Unexpected received message bytes: %v", string(inMsg.Bytes))
	}

	// Receive message from channel 1 and check
	inMsg = s2.Receive("ch1")
	if string(inMsg.Bytes) != "channel one" {
		t.Errorf("Unexpected received message bytes: %v", string(inMsg.Bytes))
	}
}

func BenchmarkSwitches(b *testing.B) {

	b.StopTimer()

	channels := []string{"ch1", "ch2", "ch3", "ch4", "ch5", "ch6", "ch7", "ch8", "ch9", "ch0"}
	s1, s2 := makeSwitchPair(b, 10, channels)
	defer s1.Stop()
	defer s2.Stop()

	// Create a sink on either channel to just pop off messages.
	recvHandler := func(c *Switch, chName string) {
		for {
			it := c.Receive(chName)
			if it == nil {
				break
			}
		}
	}

	for _, chName := range channels {
		go recvHandler(s1, chName)
		go recvHandler(s2, chName)
	}

	// Allow time for goroutines to boot up
	time.Sleep(1000 * time.Millisecond)
	b.StartTimer()

	numSuccess, numFailure := 0, 0

	// Send random message from one channel to another
	for i := 0; i < b.N; i++ {
		chName := channels[i%len(channels)]
		pkt := NewPacket(String(chName), ByteSlice("test data"))
		nS, nF := s1.Broadcast(pkt)
		numSuccess += nS
		numFailure += nF
	}

	log.Warnf("success: %v, failure: %v", numSuccess, numFailure)

	// Allow everything to flush before stopping switches & closing connections.
	b.StopTimer()
	time.Sleep(1000 * time.Millisecond)

}
