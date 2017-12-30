package p2p

import (
	"crypto/rand"
	"net"
	"time"

	crypto "github.com/libp2p/go-libp2p-crypto"
	inet "github.com/libp2p/go-libp2p-net"
	lpeer "github.com/libp2p/go-libp2p-peer"
	protocol "github.com/libp2p/go-libp2p-protocol"
	ma "github.com/multiformats/go-multiaddr"
)

// pipeStream satisfies inet.Stream and uses a pipe
type pipeStream struct {
	conn         net.Conn
	localPrivKey crypto.PrivKey
	remotePubKey crypto.PubKey
	proto        protocol.ID
}

func streamPipe() (inet.Stream, inet.Stream) {
	pa, pb := netPipe()

	p := protocol.ID("pipe")
	privKeyA, pubKeyA, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		panic(err)
	}
	privKeyB, pubKeyB, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		panic(err)
	}

	psa := &pipeStream{
		conn:         pa,
		proto:        p,
		localPrivKey: privKeyA,
		remotePubKey: pubKeyB,
	}
	psb := &pipeStream{
		conn:         pb,
		proto:        p,
		localPrivKey: privKeyB,
		remotePubKey: pubKeyA,
	}

	return psa, psb
}

// Conn is a stub to statisfy inet.Stream
func (ps *pipeStream) Conn() inet.Conn {
	return ps
}

// Close closes the stream.
func (ps *pipeStream) Close() error {
	return ps.conn.Close()
}

// Read is the basic read method.
func (ps *pipeStream) Read(p []byte) (n int, err error) {
	return ps.conn.Read(p)
}

// Write is the basic write method.
func (ps *pipeStream) Write(p []byte) (n int, err error) {
	return ps.conn.Write(p)
}

// Protocol returns the stream protocol ID.
func (ps *pipeStream) Protocol() protocol.ID {
	return ps.proto
}

// Reset closes both ends of the stream. Use this to tell the remote
// side to hang up and go away.
func (ps *pipeStream) Reset() error {
	return ps.Close()
}

func (ps *pipeStream) SetDeadline(t time.Time) error {
	return ps.conn.SetDeadline(t)
}

func (ps *pipeStream) SetReadDeadline(t time.Time) error {
	return ps.conn.SetReadDeadline(t)
}

func (ps *pipeStream) SetWriteDeadline(t time.Time) error {
	return ps.conn.SetWriteDeadline(t)
}

// SetProtocol sets the protocol ID.
func (ps *pipeStream) SetProtocol(id protocol.ID) {
	ps.proto = id
}

// NewStream constructs a new Stream over this conn.
func (ps *pipeStream) NewStream() (inet.Stream, error) {
	panic("unimplemented NewStream on pipeStream")
}

// GetStreams returns all open streams over this conn.
func (ps *pipeStream) GetStreams() ([]inet.Stream, error) {
	panic("unimplemented GetStreams on pipeStream")
}

// LocalPeer returns the local peer id
func (ps *pipeStream) LocalPeer() lpeer.ID {
	id, _ := lpeer.IDFromPrivateKey(ps.localPrivKey)
	return id
}

func (ps *pipeStream) LocalPrivateKey() crypto.PrivKey {
	return ps.localPrivKey
}

func (ps *pipeStream) LocalMultiaddr() ma.Multiaddr {
	return nil
}

// RemotePeer returns the remote peer id
func (ps *pipeStream) RemotePeer() lpeer.ID {
	id, _ := lpeer.IDFromPublicKey(ps.remotePubKey)
	return id
}

func (ps *pipeStream) RemotePublicKey() crypto.PubKey {
	return ps.remotePubKey
}

func (ps *pipeStream) RemoteMultiaddr() ma.Multiaddr {
	return nil
}
