package p2p

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/tendermint/tendermint/types"
)

var (
	// stringHasScheme tries to detect URLs with schemes. It looks for a : before a / (if any).
	stringHasScheme = func(str string) bool {
		return strings.Contains(str, "://")
	}

	// reSchemeIsHost tries to detect URLs where the scheme part is instead a
	// hostname, i.e. of the form "host:80/path" where host: is a hostname.
	reSchemeIsHost = regexp.MustCompile(`^[^/:]+:\d+(/|$)`)
)

// NodeAddress is a node address URL. It differs from a transport Endpoint in
// that it contains the node's ID, and that the address hostname may be resolved
// into multiple IP addresses (and thus multiple endpoints).
//
// If the URL is opaque, i.e. of the form "scheme:opaque", then the opaque part
// is expected to contain a node ID.
type NodeAddress struct {
	NodeID   types.NodeID
	Protocol Protocol
	Hostname string
	Port     uint16
	Path     string
}

// ParseNodeAddress parses a node address URL into a NodeAddress, normalizing
// and validating it.
func ParseNodeAddress(urlString string) (NodeAddress, error) {
	// url.Parse requires a scheme, so if it fails to parse a scheme-less URL
	// we try to apply a default scheme.
	url, err := url.Parse(urlString)
	if (err != nil || url.Scheme == "") &&
		(!stringHasScheme(urlString) || reSchemeIsHost.MatchString(urlString)) {
		url, err = url.Parse(string(defaultProtocol) + "://" + urlString)
	}
	if err != nil {
		return NodeAddress{}, fmt.Errorf("invalid node address %q: %w", urlString, err)
	}

	address := NodeAddress{
		Protocol: Protocol(strings.ToLower(url.Scheme)),
	}

	// Opaque URLs are expected to contain only a node ID.
	if url.Opaque != "" {
		address.NodeID = types.NodeID(url.Opaque)
		return address, address.Validate()
	}

	// Otherwise, just parse a normal networked URL.
	if url.User != nil {
		address.NodeID = types.NodeID(strings.ToLower(url.User.Username()))
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

	return address, address.Validate()
}

// Resolve resolves a NodeAddress into a set of Endpoints, by expanding
// out a DNS hostname to IP addresses.
func (a NodeAddress) Resolve(ctx context.Context) ([]*Endpoint, error) {
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
		return []*Endpoint{{
			Protocol: a.Protocol,
			Path:     string(a.NodeID),
		}}, nil
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", a.Hostname)
	if err != nil {
		return nil, err
	}
	endpoints := make([]*Endpoint, len(ips))
	for i, ip := range ips {
		endpoints[i] = &Endpoint{
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

	case a.Protocol != "" && (a.Path == "" || a.Path == string(a.NodeID)):
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
	} else if err := a.NodeID.Validate(); err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}
	if a.Port > 0 && a.Hostname == "" {
		return errors.New("cannot specify port without hostname")
	}
	return nil
}
