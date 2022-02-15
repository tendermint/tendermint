package quorum

import (
	"fmt"
	"net"
	"time"

	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/conn"
	"github.com/tendermint/tendermint/types"
)

const (
	// DefaultDialTimeout when resolving node id using TCP connection
	DefaultDialTimeout = 1000 * time.Millisecond
	// DefaultConnectionTimeout is a connection timeout when resolving node id using TCP connection
	DefaultConnectionTimeout = 1 * time.Second
)

type tcpNodeIDResolver struct {
	DialerTimeout     time.Duration
	ConnectionTimeout time.Duration
}

// NewTCPNodeIDResolver creates new NodeIDResolver that connects to remote host with p2p protocol and
// derives node ID from remote p2p public key.
func NewTCPNodeIDResolver() p2p.NodeIDResolver {
	return &tcpNodeIDResolver{
		DialerTimeout:     DefaultDialTimeout,
		ConnectionTimeout: DefaultConnectionTimeout,
	}
}

// connect establishes a TCP connection to remote host.
// When err == nil, caller is responsible for closing of the connection
func (resolver tcpNodeIDResolver) connect(host string, port uint16) (net.Conn, error) {
	dialer := net.Dialer{
		Timeout: resolver.DialerTimeout,
	}
	connection, err := dialer.Dial("tcp4", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, fmt.Errorf("cannot lookup node ID: %w", err)
	}
	if err := connection.SetDeadline(time.Now().Add(resolver.ConnectionTimeout)); err != nil {
		connection.Close()
		return nil, err
	}

	return connection, nil
}

// Resolve implements NodeIDResolver
// Resolve retrieves a node ID from remote validator and generates a correct node address.
// Note that it is quite expensive, as it establishes secure connection to the other node
// which is dropped afterwards.
func (resolver tcpNodeIDResolver) Resolve(va types.ValidatorAddress) (p2p.NodeAddress, error) {
	connection, err := resolver.connect(va.Hostname, va.Port)
	if err != nil {
		return p2p.NodeAddress{}, err
	}
	defer connection.Close()

	sc, err := conn.MakeSecretConnection(connection, nil)
	if err != nil {
		return p2p.NodeAddress{}, err
	}
	va.NodeID = types.NodeIDFromPubKey(sc.RemotePubKey())
	return nodeAddress(va), nil
}
