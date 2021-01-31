package p2p

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/tendermint/tendermint/crypto"
)

// NodeIDByteLength is the length of a crypto.Address. Currently only 20.
// FIXME: support other length addresses?
const NodeIDByteLength = crypto.AddressSize

// reNodeID is a regexp for valid node IDs.
var reNodeID = regexp.MustCompile(`^[0-9a-f]{40}$`)

// NodeID is a hex-encoded crypto.Address. It must be lowercased
// (for uniqueness) and of length 2*NodeIDByteLength.
type NodeID string

// NewNodeID returns a lowercased (normalized) NodeID, or errors if the
// node ID is invalid.
func NewNodeID(nodeID string) (NodeID, error) {
	n := NodeID(strings.ToLower(nodeID))
	return n, n.Validate()
}

// NodeIDFromPubKey creates a node ID from a given PubKey address.
func NodeIDFromPubKey(pubKey crypto.PubKey) NodeID {
	return NodeID(hex.EncodeToString(pubKey.Address()))
}

// Bytes converts the node ID to its binary byte representation.
func (id NodeID) Bytes() ([]byte, error) {
	bz, err := hex.DecodeString(string(id))
	if err != nil {
		return nil, fmt.Errorf("invalid node ID encoding: %w", err)
	}
	return bz, nil
}

// Validate validates the NodeID.
func (id NodeID) Validate() error {
	switch {
	case len(id) == 0:
		return errors.New("empty node ID")

	case len(id) != 2*NodeIDByteLength:
		return fmt.Errorf("invalid node ID length %d, expected %d", len(id), 2*NodeIDByteLength)

	case !reNodeID.MatchString(string(id)):
		return fmt.Errorf("node ID can only contain lowercased hex digits")

	default:
		return nil
	}
}

// PeerAddress is a peer address URL. It differs from Endpoint in that the
// address hostname may be expanded into multiple IP addresses (thus multiple
// endpoints), and that it knows the node's ID.
//
// If the URL is opaque, i.e. of the form "scheme:<opaque>", then the opaque
// part has to contain either the node ID or a node ID and path in the form
// "scheme:<nodeid>@<path>".
type PeerAddress struct {
	NodeID   NodeID
	Protocol Protocol
	Hostname string
	Port     uint16
	Path     string
}

// ParsePeerAddress parses a peer address URL into a PeerAddress,
// normalizing and validating it.
func ParsePeerAddress(urlString string) (PeerAddress, error) {
	url, err := url.Parse(urlString)
	if err != nil || url == nil {
		return PeerAddress{}, fmt.Errorf("invalid peer address %q: %w", urlString, err)
	}

	address := PeerAddress{}

	// If the URL is opaque, i.e. in the form "scheme:<opaque>", we specify the
	// opaque bit to be either a node ID or a node ID and path in the form
	// "scheme:<nodeid>@<path>".
	if url.Opaque != "" {
		parts := strings.Split(url.Opaque, "@")
		if len(parts) > 2 {
			return PeerAddress{}, fmt.Errorf("invalid address format %q, unexpected @", urlString)
		}
		address.NodeID, err = NewNodeID(parts[0])
		if err != nil {
			return PeerAddress{}, fmt.Errorf("invalid peer ID %q: %w", parts[0], err)
		}
		if len(parts) == 2 {
			address.Path = parts[1]
		}
		return address, nil
	}

	// Otherwise, just parse a normal networked URL.
	address.NodeID, err = NewNodeID(url.User.Username())
	if err != nil {
		return PeerAddress{}, fmt.Errorf("invalid peer ID %q: %w", url.User.Username(), err)
	}

	if url.Scheme != "" {
		address.Protocol = Protocol(strings.ToLower(url.Scheme))
	} else {
		address.Protocol = defaultProtocol
	}

	address.Hostname = strings.ToLower(url.Hostname())

	if portString := url.Port(); portString != "" {
		port64, err := strconv.ParseUint(portString, 10, 16)
		if err != nil {
			return PeerAddress{}, fmt.Errorf("invalid port %q: %w", portString, err)
		}
		address.Port = uint16(port64)
	}

	// NOTE: URL paths are case-sensitive, so we don't lowercase them.
	address.Path = url.Path
	if url.RawPath != "" {
		address.Path = url.RawPath
	}
	if url.RawQuery != "" {
		address.Path += "?" + url.RawQuery
	}
	if url.RawFragment != "" {
		address.Path += "#" + url.RawFragment
	}
	if address.Path != "" && address.Path[0] != '/' && address.Path[0] != '#' {
		address.Path = "/" + address.Path
	}

	return address, address.Validate()
}

// Resolve resolves a PeerAddress into a set of Endpoints, by expanding
// out a DNS hostname to IP addresses.
func (a PeerAddress) Resolve(ctx context.Context) ([]Endpoint, error) {
	// If there is no hostname, this is an opaque URL in the form
	// "scheme:<opaque>".
	if a.Hostname == "" {
		return []Endpoint{{
			Protocol: a.Protocol,
			Path:     a.Path,
		}}, nil
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", a.Hostname)
	if err != nil {
		return nil, err
	}
	endpoints := make([]Endpoint, len(ips))
	for i, ip := range ips {
		endpoints[i] = Endpoint{
			Protocol: a.Protocol,
			IP:       ip,
			Port:     a.Port,
			Path:     a.Path,
		}
	}
	return endpoints, nil
}

// Validates validates a PeerAddress.
func (a PeerAddress) Validate() error {
	if a.Protocol == "" {
		return errors.New("no protocol")
	}
	if a.NodeID == "" {
		return errors.New("no peer ID")
	} else if err := a.NodeID.Validate(); err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}
	if a.Port > 0 && a.Hostname == "" {
		return errors.New("cannot specify port without hostname")
	}
	return nil
}

// String formats the address as a URL string.
func (a PeerAddress) String() string {
	u := url.URL{Scheme: string(a.Protocol)}
	if a.NodeID != "" {
		u.User = url.User(string(a.NodeID))
	}
	switch {
	case a.Hostname != "":
		if a.Port > 0 {
			u.Host = net.JoinHostPort(a.Hostname, strconv.Itoa(int(a.Port)))
		} else {
			u.Host = a.Hostname
		}
		u.Path = a.Path
	case a.Protocol != "":
		u.Opaque = a.Path // e.g. memory:foo
	case a.Path != "" && a.Path[0] != '/':
		u.Path = "/" + a.Path // e.g. some/path
	default:
		u.Path = a.Path // e.g. /some/path
	}
	return strings.TrimPrefix(u.String(), "//")
}
