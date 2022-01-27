package p2p

// BACKPORTING NOTE:
//
// This file was downloaded from Tendermint master, revision bc1a20dbb86e4fa2120b2c8a9de88814471f4a2c:
// https://raw.githubusercontent.com/tendermint/tendermint/bc1a20dbb86e4fa2120b2c8a9de88814471f4a2c/internal/p2p/address.go
// and refactored to workaround some dependencies.
// Changes:
// 1. ParseNodeAddress divided into 2 functions (added ParseNodeAddressWithoutValidation)
// 2. Added some missing types at the end of file
import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var (
	// stringHasScheme tries to detect URLs with schemes. It looks for a : before a / (if any).
	stringHasScheme = func(str string) bool {
		return strings.Contains(str, "://")
	}

	// reSchemeIsHost tries to detect URLs where the scheme part is instead a
	// hostname, i.e. of the form "host:80/path" where host: is a hostname.
	reSchemeIsHost      = regexp.MustCompile(`^[^/:]+:\d+(/|$)`)
	ErrEmptyNodeAddress = errors.New("empty node address")
)

// NodeAddress is a node address URL. It differs from a transport Endpoint in
// that it contains the node's ID, and that the address hostname may be resolved
// into multiple IP addresses (and thus multiple endpoints).
//
// If the URL is opaque, i.e. of the form "scheme:opaque", then the opaque part
// is expected to contain a node ID.
type NodeAddress struct {
	NodeID   ID
	Protocol string
	Hostname string
	Port     uint16
	Path     string
}

// ParseNodeAddress parses a node address URL into a NodeAddress, normalizing
// and validating it.
func ParseNodeAddress(urlString string) (NodeAddress, error) {
	if urlString == "" {
		return NodeAddress{}, ErrEmptyNodeAddress
	}
	address, err := ParseNodeAddressWithoutValidation(urlString)
	if err != nil {
		return address, err
	}
	return address, address.Validate()
}

// ParseNodeAddressWithoutValidation  parses a node address URL into a NodeAddress, normalizing it.
// It does NOT validate parsed address
func ParseNodeAddressWithoutValidation(urlString string) (NodeAddress, error) {
	// url.Parse requires a scheme, so if it fails to parse a scheme-less URL
	// we try to apply a default scheme.
	url, err := url.Parse(urlString)
	if (err != nil || url.Scheme == "") &&
		(!stringHasScheme(urlString) || reSchemeIsHost.MatchString(urlString)) {
		url, err = url.Parse("tcp://" + urlString)
	}
	if err != nil {
		return NodeAddress{}, fmt.Errorf("invalid node address %q: %w", urlString, err)
	}

	address := NodeAddress{
		Protocol: (strings.ToLower(url.Scheme)),
	}

	// Opaque URLs are expected to contain only a node ID.
	if url.Opaque != "" {
		address.NodeID = ID(url.Opaque)
		return address, nil
	}

	// Otherwise, just parse a normal networked URL.
	if url.User != nil {
		address.NodeID = ID(strings.ToLower(url.User.Username()))
	}

	address.Hostname = strings.ToLower(url.Hostname())

	if portString := url.Port(); portString != "" {
		port64, err := strconv.ParseUint(portString, 10, 16)
		if err != nil {
			return NodeAddress{}, fmt.Errorf("invalid port %q: %w", portString, err)
		}
		address.Port = uint16(port64)
	}

	address.Path = url.Path
	if url.RawQuery != "" {
		address.Path += "?" + url.RawQuery
	}
	if url.Fragment != "" {
		address.Path += "#" + url.Fragment
	}
	if address.Path != "" {
		switch address.Path[0] {
		case '/', '#', '?':
		default:
			address.Path = "/" + address.Path
		}
	}

	return address, nil
}

// Resolve resolves a NodeAddress into a set of Endpoints, by expanding
// out a DNS hostname to IP addresses.
func (a NodeAddress) Resolve(ctx context.Context) ([]Endpoint, error) {
	if a.Protocol == "" {
		return nil, errors.New("address has no protocol")
	}

	// If there is no hostname, this is an opaque URL in the form
	// "scheme:opaque", and the opaque part is assumed to be node ID used as
	// Path.
	if a.Hostname == "" {
		if a.NodeID == "" {
			return nil, errors.New("local address has no node ID")
		}
		return []Endpoint{{
			Protocol: a.Protocol,
			Path:     string(a.NodeID),
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

// String formats the address as a URL string.
func (a NodeAddress) String() string {
	if a.Zero() {
		return ""
	}

	u := url.URL{Scheme: a.Protocol}
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

	case a.Protocol != "" && (a.Path == "" || ID(a.Path) == a.NodeID):
		u.User = nil
		u.Opaque = string(a.NodeID) // e.g. memory:id

	case a.Path != "" && a.Path[0] != '/':
		u.Path = "/" + a.Path // e.g. some/path

	default:
		u.Path = a.Path // e.g. /some/path
	}
	return strings.TrimPrefix(u.String(), "//")
}

// Validate validates a NodeAddress.
func (a NodeAddress) Validate() error {
	if a.Protocol == "" {
		return errors.New("no protocol")
	}
	if a.NodeID == "" {
		return errors.New("no peer ID")
	}
	if a.Port > 0 && a.Hostname == "" {
		return errors.New("cannot specify port without hostname")
	}
	return nil
}

func (a NodeAddress) Zero() bool {
	return reflect.ValueOf(a).IsZero()
}

func (a NodeAddress) NetAddress() (*NetAddress, error) {
	addr, err := NewNetAddressString(a.String())
	if err != nil || addr == nil {
		return nil, err
	}
	return addr, nil
}

// Endpoint represents a transport connection endpoint, either local or remote.
//
// Endpoints are not necessarily networked (see e.g. MemoryTransport) but all
// networked endpoints must use IP as the underlying transport protocol to allow
// e.g. IP address filtering. Either IP or Path (or both) must be set.
type Endpoint struct {
	// Protocol specifies the transport protocol.
	Protocol string

	// IP is an IP address (v4 or v6) to connect to. If set, this defines the
	// endpoint as a networked endpoint.
	IP net.IP

	// Port is a network port (either TCP or UDP). If 0, a default port may be
	// used depending on the protocol.
	Port uint16

	// Path is an optional transport-specific path or identifier.
	Path string
}
