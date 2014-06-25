package peer

import (
    "atomic"
    "net"
)

/* Listener */

type Listener interface {
    Connections()   <-chan *Connection
    LocalAddress()  *NetAddress
    Stop()
}


/* DefaultListener */

type DefaultListener struct {
    listener        net.Listener
    connections     chan *Connection
    stopped         uint32
}

const (
    DEFAULT_BUFFERED_CONNECTIONS = 10
)

func NewListener(protocol string, laddr string) *Listener {
    ln, err := net.Listen(protocol, laddr)
    if err != nil { panic(err) }

    s := &Listener{
        listener:       ln,
        connections:    make(chan *Connection, DEFAULT_BUFFERED_CONNECTIONS),
    }

    go l.listenHandler()

    return s
}

func (l *Listener) listenHandler() {
    for {
        conn, err := l.listener.Accept()

        if atomic.LoadUint32(&l.stopped) == 1 { return }

        // listener wasn't stopped,
        // yet we encountered an error.
        if err != nil { panic(err) }

        c := NewConnection(con)
        l.connections <- c
    }

    // cleanup
    close(l.connections)
    for _ = range l.connections {
        // drain
    }
}

func (l *Listener) Connections() <-chan *Connection {
    return l.connections
}

func (l *Listener) LocalAddress() *NetAddress {
    return NewNetAddress(l.listener.Addr())
}

func (l *Listener) Stop() {
    if atomic.CompareAndSwapUint32(&l.stopped, 0, 1) {
        l.listener.Close()
    }
}
