package peer

import (
    "testing"
    "time"
)

func TestLocalConnection(t *testing.T) {

    c1 := NewClient(func(conn *Connection) *Peer {
        p := &Peer{conn: conn}

        ch1 := NewChannel(String("ch1"),
                nil,
                // XXX these channels should be buffered.
                make(chan ByteSlice),
                make(chan ByteSlice),
        )

        ch2 := NewChannel(String("ch2"),
                nil,
                make(chan ByteSlice),
                make(chan ByteSlice),
        )

        channels := make(map[String]*Channel)
        channels[ch1.Name] = ch1
        channels[ch2.Name] = ch2
        p.channels = channels

        return p
    })

    // XXX make c2 like c1.

    c2 := NewClient(func(conn *Connection) *Peer {
        return nil
    })

    // XXX clients don't have "local addresses"
    c1.ConnectTo(c2.LocalAddress())

    // lets send a message from c1 to c2.
    c1.Broadcast(String(""), String("message"))
    time.Sleep(500 * time.Millisecond)

    inMsg := c2.PopMessage()

    c1.Stop()
    c2.Stop()
}
