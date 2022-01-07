package types

import (
	"fmt"
	"net"
	"time"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
)

const (
	// DefaultDialTimeout when resolving node id using TCP connection
	DefaultDialTimeout = 1000 * time.Millisecond
	// DefaultConnectionTimeout is a connection timeout when resolving node id using TCP connection
	DefaultConnectionTimeout = 1 * time.Second
)

// NodeIDResolver determines a node ID based on validator address
type NodeIDResolver interface {
	// Resolve retrieves a node ID from remote node.
	Resolve(ValidatorAddress) (p2p.ID, error)
}

type tcpNodeIDResolver struct {
	DialerTimeout     time.Duration
	ConnectionTimeout time.Duration
	// other dependencies
}

// NewTCPNodeIDResolver creates new NodeIDResolver that connects to remote host with p2p protocol and
// derives node ID from remote p2p public key.
func NewTCPNodeIDResolver() NodeIDResolver {
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
// Resolve retrieves a node ID from remote node.
// Note that it is quite expensive, as it establishes secure connection to the other node
// which is dropped afterwards.
func (resolver tcpNodeIDResolver) Resolve(va ValidatorAddress) (p2p.ID, error) {
	connection, err := resolver.connect(va.Hostname, va.Port)
	if err != nil {
		return "", err
	}
	defer connection.Close()

	sc, err := conn.MakeSecretConnection(connection, nil)
	if err != nil {
		return "", err
	}
	return p2p.PubKeyToID(sc.RemotePubKey()), nil
}

type addrbookNodeIDResolver struct {
	addrBook p2p.AddrBook
}

// NewAddrbookNodeIDResolver creates new node ID resolver.
// It looks up for the node ID based on IP address, using the p2p addressbook.
func NewAddrbookNodeIDResolver(addrBook p2p.AddrBook) NodeIDResolver {
	return addrbookNodeIDResolver{addrBook: addrBook}
}

// Resolve implements NodeIDResolver
// Resolve retrieves a node ID from the address book.
func (resolver addrbookNodeIDResolver) Resolve(va ValidatorAddress) (p2p.ID, error) {
	ip := net.ParseIP(va.Hostname)
	if ip == nil {
		ips, err := net.LookupIP(va.Hostname)
		if err != nil {
			return "", p2p.ErrNetAddressLookup{Addr: va.Hostname, Err: err}
		}
		ip = ips[0]
	}

	id := resolver.addrBook.FindIP(ip, va.Port)
	if id == "" {
		return "", ErrNoNodeID
	}

	return id, nil
}
