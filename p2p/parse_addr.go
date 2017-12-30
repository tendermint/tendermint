package p2p

import (
	"errors"
	"fmt"
	"net"
	"strings"

	ma "github.com/multiformats/go-multiaddr"
	cmn "github.com/tendermint/tmlibs/common"
)

// ParseMultiaddrFlexible parses a Multiaddr or a normal net.Addr to a Multiaddr.
// Will return nil if the string is empty.
func ParseMultiaddrFlexible(addr string) (maddr ma.Multiaddr, err error) {
	addr = strings.TrimSpace(addr)
	if len(addr) == 0 {
		return
	}

	// if the address starts with / it is a multiaddr.
	if addr[0] == '/' {
		return ma.NewMultiaddr(addr)
	}

	// It must be a net.Addr
	proto, adr := cmn.ProtocolAndAddress(addr)

	switch proto {
	case "tcp":
		fallthrough
	case "tcp6":
		fallthrough
	case "tcp4":
		break
	case "udp":
		fallthrough
	case "udp4":
		fallthrough
	case "udp6":
		err = errors.New("udp is not supported")
		return
	default:
		err = fmt.Errorf("protocol is not supported: %s", proto)
	}

	// proto contains tcp, udp, tcp4, udp4, tcp6, udp6
	// adr contains 192.168.1.1:5000
	var host, port string
	host, port, err = net.SplitHostPort(adr)
	if err != nil {
		return
	}

	if len(port) == 0 {
		err = fmt.Errorf("address must specify port: %s", addr)
		return
	}

	if len(host) == 0 {
		err = fmt.Errorf("address must specify host: %s", addr)
		return
	}

	// TODO: allow DNS lookup here
	adrIP := net.ParseIP(host)
	if adrIP == nil {
		err = fmt.Errorf("invalid ip address (dns not supported): %s (from %s)", host, addr)
		return
	}

	ipProto := "ip6"
	if p4 := adrIP.To4(); len(p4) == net.IPv4len {
		ipProto = "ip4"
	}

	maddr, err = ma.NewMultiaddr(fmt.Sprintf("/%s/%s/tcp/%s", ipProto, adrIP.String(), port))
	return
}
