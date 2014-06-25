package peer

import (
)

/* Server */

type Server struct {
    listener        Listener
    client          *Client
}

func NewServer(l Listener, c *Client) *Server {
    s := &Server{
        listener:   l,
        client:     c,
    }
    go s.IncomingConnectionsHandler()
    return s
}

// meant to run in a goroutine
func (s *Server) IncomingConnectionHandler() {
    for conn := range s.listener.Connections() {
        s.client.AddIncomingConnection(conn)
    }
}

func (s *Server) Stop() {
    s.listener.Stop()
    s.client.Stop()
}
