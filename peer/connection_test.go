package peer

import (
    . "github.com/tendermint/tendermint/binary"
    "testing"
    "time"
)

func TestLocalConnection(t *testing.T) {

    makePeer := func(conn *Connection) *Peer {
        bufferSize := 10
        p := &Peer{conn: conn}
        p.channels := map[String]*Channel{}
        p.channels["ch1"] = NewChannel("ch1", bufferSize)
        p.channels["ch2"] = NewChannel("ch2", bufferSize)
        return p
    }

    c1 := NewClient(makePeer)
    c2 := NewClient(makePeer)

    s1 := NewServer("tcp", "127.0.0.1:8001", c1)

    c2.ConnectTo(c1.LocalAddress())

    // lets send a message from c1 to c2.
    // XXX do we even want a broadcast function?
    c1.Broadcast(String(""), String("message"))
    time.Sleep(500 * time.Millisecond)

    inMsg := c2.PopMessage(String(""))

    c1.Stop()
    c2.Stop()
}
