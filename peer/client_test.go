package peer

import (
	. "github.com/tendermint/tendermint/binary"
	"testing"
	"time"
)

// convenience method for creating two clients connected to each other.
func makeClientPair(t *testing.T, bufferSize int, channels []string) (*Client, *Client) {

	peerMaker := func(conn *Connection) *Peer {
		p := NewPeer(conn)
		p.channels = map[String]*Channel{}
		for chName := range channels {
			p.channels[String(chName)] = NewChannel(String(chName), bufferSize)
		}
		return p
	}

	// Create two clients that will be interconnected.
	c1 := NewClient(peerMaker)
	c2 := NewClient(peerMaker)

	// Create a server for the listening client.
	s1 := NewServer("tcp", ":8001", c1)

	// Dial the server & add the connection to c2.
	s1laddr := s1.LocalAddress()
	conn, err := s1laddr.Dial()
	if err != nil {
		t.Fatalf("Could not connect to server address %v", s1laddr)
	} else {
		t.Logf("Created a connection to local server address %v", s1laddr)
	}

	c2.AddPeerWithConnection(conn, true)

	// Wait for things to happen, peers to get added...
	time.Sleep(100 * time.Millisecond)

	return c1, c2
}

func TestClients(t *testing.T) {

	c1, c2 := makeClientPair(t, 10, []string{"ch1", "ch2", "ch3"})

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

	s1.Stop()
	c2.Stop()
}

func BenchmarkClients(b *testing.B) {

	b.StopTimer()

	// TODO: benchmark the random functions, which is faster?

	c1, c2 := makeClientPair(t, 10, []string{"ch1", "ch2", "ch3"})

	// Create a sink on either channel to just pop off messages.
	// TODO: ensure that when clients stop, this goroutine stops.
	recvHandler := func(c *Client) {
	}

	go recvHandler(c1)
	go recvHandler(c2)

	b.StartTimer()

	// Send random message from one channel to another
	for i := 0; i < b.N; i++ {
	}

}
