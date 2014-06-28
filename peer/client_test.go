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

    conn, err := s1.LocalAddress().Dial()
    if err != nil {
        t.Fatalf("Could not connect to server address %v", s1.LocalAddress())
    }

    c2.AddPeerWithConnection(conn, true)

    // lets send a message from c1 to c2.
    // XXX do we even want a broadcast function?
    //c1.Broadcast(String(""), String("message"))
    time.Sleep(500 * time.Millisecond)

    //inMsg := c2.PopMessage(String(""))

    s1.Stop()
    c2.Stop()
}
