package peer

import (
    "sync/atomic"
    "net"
)

/* Server */

type Server struct {
    listener        Listener
    client          *Client
}

func NewServer(protocol string, laddr string, c *Client) *Server {
    l := NewListener(protocol, laddr)
    s := &Server{
        listener:   l,
        client:     c,
    }
    go s.IncomingConnectionHandler()
    return s
}

func (s *Server) LocalAddress() *NetAddress {
    return s.listener.LocalAddress()
}

// meant to run in a goroutine
func (s *Server) IncomingConnectionHandler() {
    for conn := range s.listener.Connections() {
        s.client.AddPeerWithConnection(conn, false)
    }
}

func (s *Server) Stop() {
    s.listener.Stop()
    s.client.Stop()
}
