package peer

import (
    . "github.com/tendermint/tendermint/binary"
    "testing"
    "time"
)

func TestConnection(t *testing.T) {

    peerMaker := func(conn *Connection) *Peer {
        bufferSize := 10
        p := NewPeer(conn)
        p.channels = map[String]*Channel{}
        p.channels["ch1"] = NewChannel("ch1", bufferSize)
        p.channels["ch2"] = NewChannel("ch2", bufferSize)
        return p
    }

    c1 := NewClient(peerMaker)
    c2 := NewClient(peerMaker)

    s1 := NewServer("tcp", ":8001", c1)
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

    // lets send a message from c1 to c2.
    if c1.Peers().Size() != 1 {
        t.Errorf("Expected exactly 1 peer in c1, got %v", c1.Peers().Size())
    }
    if c2.Peers().Size() != 1 {
        t.Errorf("Expected exactly 1 peer in c2, got %v", c2.Peers().Size())
    }

    // TODO: test the transmission of information on channels.
    c1.Broadcast(NewPacket("ch1", ByteSlice("test data")))
    time.Sleep(100 * time.Millisecond)
    inMsg := c2.Receive("ch1")

    t.Logf("c2 popped message: %v", inMsg)

    s1.Stop()
    c2.Stop()
}
