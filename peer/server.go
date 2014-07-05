package peer

/* Server */

type Server struct {
	listener Listener
	client   *Client
}

func NewServer(protocol string, laddr string, c *Client) *Server {
	l := NewDefaultListener(protocol, laddr)
	s := &Server{
		listener: l,
		client:   c,
	}
	go s.IncomingConnectionHandler()
	return s
}

func (s *Server) ExternalAddress() *NetAddress {
	return s.listener.ExternalAddress()
}

// meant to run in a goroutine
func (s *Server) IncomingConnectionHandler() {
	for conn := range s.listener.Connections() {
		log.Infof("New connection found: %v", conn)
		s.client.AddPeerWithConnection(conn, false)
	}
}

// stops the server, not the client.
func (s *Server) Stop() {
	log.Infof("Stopping server")
	s.listener.Stop()
}
