package peer

import (
	"testing"
	"time"

	. "github.com/tendermint/tendermint/binary"
)

// convenience method for creating two clients connected to each other.
func makeClientPair(t testing.TB, bufferSize int, chNames []String) (*Client, *Client) {

	peerMaker := func(conn *Connection) *Peer {
		channels := map[String]*Channel{}
		for _, chName := range chNames {
			channels[chName] = NewChannel(chName, bufferSize)
		}
		return NewPeer(conn, channels)
	}

	// Create two clients that will be interconnected.
	c1 := NewClient(peerMaker)
	c2 := NewClient(peerMaker)

	// Create a server for the listening client.
	s1 := NewServer("tcp", ":8001", c1)

	// Dial the server & add the connection to c2.
	s1laddr := s1.ExternalAddress()
	conn, err := s1laddr.Dial()
	if err != nil {
		t.Fatalf("Could not connect to server address %v", s1laddr)
	} else {
		t.Logf("Created a connection to local server address %v", s1laddr)
	}

	c2.AddPeerWithConnection(conn, true)

	// Wait for things to happen, peers to get added...
	time.Sleep(100 * time.Millisecond)

	// Close the server, no longer needed.
	s1.Stop()

	return c1, c2
}

func TestClients(t *testing.T) {

	channels := []String{"ch1", "ch2", "ch3", "ch4", "ch5", "ch6", "ch7", "ch8", "ch9", "ch0"}
	c1, c2 := makeClientPair(t, 10, channels)
	defer c1.Stop()
	defer c2.Stop()

	// Lets send a message from c1 to c2.
	if c1.Peers().Size() != 1 {
		t.Errorf("Expected exactly 1 peer in c1, got %v", c1.Peers().Size())
	}
	if c2.Peers().Size() != 1 {
		t.Errorf("Expected exactly 1 peer in c2, got %v", c2.Peers().Size())
	}

	// Broadcast a message on ch1
	c1.Broadcast(NewPacket("ch1", ByteSlice("channel one")))
	// Broadcast a message on ch2
	c1.Broadcast(NewPacket("ch2", ByteSlice("channel two")))
	// Broadcast a message on ch3
	c1.Broadcast(NewPacket("ch3", ByteSlice("channel three")))

	// Wait for things to settle...
	time.Sleep(100 * time.Millisecond)

	// Receive message from channel 2 and check
	inMsg := c2.Receive("ch2")
	if string(inMsg.Bytes) != "channel two" {
		t.Errorf("Unexpected received message bytes: %v", string(inMsg.Bytes))
	}

	// Receive message from channel 1 and check
	inMsg = c2.Receive("ch1")
	if string(inMsg.Bytes) != "channel one" {
		t.Errorf("Unexpected received message bytes: %v", string(inMsg.Bytes))
	}
}

func BenchmarkClients(b *testing.B) {

	b.StopTimer()

	channels := []String{"ch1", "ch2", "ch3", "ch4", "ch5", "ch6", "ch7", "ch8", "ch9", "ch0"}
	c1, c2 := makeClientPair(b, 10, channels)
	defer c1.Stop()
	defer c2.Stop()

	// Create a sink on either channel to just pop off messages.
	recvHandler := func(c *Client, chName String) {
		for {
			it := c.Receive(chName)
			if it == nil {
				break
			}
		}
	}

	for _, chName := range channels {
		go recvHandler(c1, chName)
		go recvHandler(c2, chName)
	}

	// Allow time for goroutines to boot up
	time.Sleep(1000 * time.Millisecond)
	b.StartTimer()

	numSuccess, numFailure := 0, 0

	// Send random message from one channel to another
	for i := 0; i < b.N; i++ {
		chName := channels[i%len(channels)]
		pkt := NewPacket(chName, ByteSlice("test data"))
		nS, nF := c1.Broadcast(pkt)
		numSuccess += nS
		numFailure += nF
	}

	log.Warnf("success: %v, failure: %v", numSuccess, numFailure)

	// Allow everything to flush before stopping clients & closing connections.
	b.StopTimer()
	time.Sleep(1000 * time.Millisecond)

}
