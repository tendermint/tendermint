package p2p

import (
	"fmt"
	"net"
	"time"

	"github.com/tendermint/tendermint/p2p/conn"
)

type accept struct {
	mPeer *multiplexPeer
	err   error
}

// PeerTransport proxies incoming and outgoing peer connections.
type PeerTransport interface {
	// Accept returns a newly connected Peer.
	Accept() (Peer, error)

	// Dial connects to a Peer.
	Dial(NetAddress) (Peer, error)

	// lifecycle methods
	Close() error
	Listen(NetAddress) error
}

// multiplexTransport accepts tcp connections and upgrades to multiplexted
// peers.
type multiplexTransport struct {
	listener net.Listener

	acceptc chan accept
	closec  <-chan struct{}

	dialTimeout      time.Duration
	handshakeTimeout time.Duration
	nodeInfo         NodeInfo
}

var _ PeerTransport = (*multiplexTransport)(nil)

// NewMTransport returns network connected multiplexed peers.
func NewMTransport(nodeInfo NodeInfo) PeerTransport {
	return &multiplexTransport{
		nodeInfo: nodeInfo,
	}
}

func (mt *multiplexTransport) Accept() (Peer, error) {
	a := <-mt.acceptc
	return a.mPeer, a.err
}

func (mt *multiplexTransport) Dial(addr NetAddress) (Peer, error) {
	c, err := addr.DialTimeout(mt.dialTimeout)
	if err != nil {
		return nil, err
	}

	c, err = mt.secretConn(c)
	if err != nil {
		return nil, err
	}

	peerNodeInfo, err := mt.handshake(c)
	if err != nil {
		return nil, err
	}

	return newMultiplexPeer(c.(*conn.SecretConnection), addr, peerNodeInfo)
}

func (mt *multiplexTransport) Close() error {
	return fmt.Errorf("not implemented")
}

func (mt *multiplexTransport) Listen(addr NetAddress) error {
	ln, err := net.Listen("tcp", addr.DialString())
	if err != nil {
		return err
	}

	mt.listener = ln

	go mt.acceptPeers()

	return fmt.Errorf("not implemented")
}

func (mt *multiplexTransport) acceptPeers() {
	for {
		c, err := mt.listener.Accept()
		if err != nil {
			// Close has been called so we silently exit.
			if _, ok := <-mt.closec; !ok {
				return
			}

			mt.acceptc <- accept{nil, err}
			return
		}

		mp, err := mt.acceptPeer(c)

		mt.acceptc <- accept{mp, err}
	}
}

func (mt *multiplexTransport) acceptPeer(c net.Conn) (*multiplexPeer, error) {
	c, err := mt.secretConn(c)
	if err != nil {
		return nil, err
	}

	peerNodeInfo, err := mt.handshake(c)
	if err != nil {
		return nil, err
	}

	addr := NewNetAddress(peerNodeInfo.ID, c.RemoteAddr())

	return newMultiplexPeer(c.(*conn.SecretConnection), *addr, peerNodeInfo)

}

func (mt *multiplexTransport) handshake(c net.Conn) (NodeInfo, error) {
	if err := c.SetDeadline(time.Now().Add(mt.handshakeTimeout)); err != nil {
		return NodeInfo{}, err
	}

	var (
		errc         = make(chan error, 2)
		peerNodeInfo NodeInfo
	)
	go func(errc chan<- error, c net.Conn) {
		_, err := cdc.MarshalBinaryWriter(c, mt.nodeInfo)
		errc <- err
	}(errc, c)
	go func(errc chan<- error, c net.Conn) {
		_, err := cdc.UnmarshalBinaryReader(
			c,
			&peerNodeInfo,
			int64(MaxNodeInfoSize()),
		)
		errc <- err
	}(errc, c)

	for i := 0; i < cap(errc); i++ {
		err := <-errc
		if err != nil {
			return NodeInfo{}, err
		}
	}

	return c.SetDeadline(time.Time{})
}

func (mt *multiplexTransport) secretConn(c) (net.Conn, error) {
	if err := c.SetDeadline(time.Now().Add(mt.handshakeTimeout)); err != nil {
		return nil, err
	}

	c, err = conn.MakeSecretConnection(c, mt.privKey)
	if err != nil {
		return nil, err
	}

	return c.SetDeadline(time.Time{})
}
