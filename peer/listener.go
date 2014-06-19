package peer

import (
    "sync"
    "net"
)

/* Listener */

type Listener struct {
    listener        net.Listener
    handler         func(net.Conn)
    mtx             sync.Mutex
    closed          bool
}

func NewListener(protocol string, laddr string, handler func(net.Conn)) *Listener {
    ln, err := net.Listen(protocol, laddr)
    if err != nil { panic(err) }

    s := &Listener{
        listener:   ln,
        handler:    handler,
    }

    go s.listen()

    return s
}

func (s *Listener) listen() {
    for {
        conn, err := s.listener.Accept()
        if err != nil {
            // lock & defer
            s.mtx.Lock(); defer s.mtx.Unlock()
            if s.closed {
                return
            } else {
                panic(err)
            }
            // unlock (deferred)
        }

        go s.handler(conn)
    }
}

func (s *Listener) LocalAddress() *NetAddress {
    return NewNetAddress(s.listener.Addr())
}

func (s *Listener) Close() {
    // lock
    s.mtx.Lock()
    s.closed = true
    s.mtx.Unlock()
    // unlock
    s.listener.Close()
}
