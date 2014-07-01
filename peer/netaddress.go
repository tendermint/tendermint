// Modified for Tendermint
// Originally Copyright (c) 2013-2014 Conformal Systems LLC.
// https://github.com/conformal/btcd/blob/master/LICENSE

package peer

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"io"
	"net"
	"strconv"
)

/* NetAddress */

type NetAddress struct {
	IP   net.IP
	Port UInt16
}

// TODO: socks proxies?
func NewNetAddress(addr net.Addr) *NetAddress {
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		Panicf("Only TCPAddrs are supported. Got: %v", addr)
	}
	ip := tcpAddr.IP
	port := UInt16(tcpAddr.Port)
	return NewNetAddressIPPort(ip, port)
}

func NewNetAddressString(addr string) *NetAddress {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	ip := net.ParseIP(host)
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		panic(err)
	}
	na := NewNetAddressIPPort(ip, UInt16(port))
	return na
}

func NewNetAddressIPPort(ip net.IP, port UInt16) *NetAddress {
	na := NetAddress{
		IP:   ip,
		Port: port,
	}
	return &na
}

func ReadNetAddress(r io.Reader) *NetAddress {
	return &NetAddress{
		IP:   net.IP(ReadByteSlice(r)),
		Port: ReadUInt16(r),
	}
}

func (na *NetAddress) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(ByteSlice(na.IP.To16()), w, n, err)
	n, err = WriteOnto(na.Port, w, n, err)
	return
}

func (na *NetAddress) Equals(other Binary) bool {
	if o, ok := other.(*NetAddress); ok {
		return na.String() == o.String()
	} else {
		return false
	}
}

func (na *NetAddress) Less(other Binary) bool {
	if o, ok := other.(*NetAddress); ok {
		return na.String() < o.String()
	} else {
		panic("Cannot compare unequal types")
	}
}

func (na *NetAddress) String() string {
	port := strconv.FormatUint(uint64(na.Port), 10)
	addr := net.JoinHostPort(na.IP.String(), port)
	return addr
}

func (na *NetAddress) Dial() (*Connection, error) {
	conn, err := net.Dial("tcp", na.String())
	if err != nil {
		return nil, err
	}
	return NewConnection(conn), nil
}

func (na *NetAddress) Routable() bool {
	// TODO(oga) bitcoind doesn't include RFC3849 here, but should we?
	return na.Valid() && !(na.RFC1918() || na.RFC3927() || na.RFC4862() ||
		na.RFC4193() || na.RFC4843() || na.Local())
}

// For IPv4 these are either a 0 or all bits set address. For IPv6 a zero
// address or one that matches the RFC3849 documentation address format.
func (na *NetAddress) Valid() bool {
	return na.IP != nil && !(na.IP.IsUnspecified() || na.RFC3849() ||
		na.IP.Equal(net.IPv4bcast))
}

func (na *NetAddress) Local() bool {
	return na.IP.IsLoopback() || zero4.Contains(na.IP)
}

func (na *NetAddress) ReachabilityTo(o *NetAddress) int {
	const (
		Unreachable = 0
		Default     = iota
		Teredo
		Ipv6_weak
		Ipv4
		Ipv6_strong
		Private
	)
	if !na.Routable() {
		return Unreachable
	} else if na.RFC4380() {
		if !o.Routable() {
			return Default
		} else if o.RFC4380() {
			return Teredo
		} else if o.IP.To4() != nil {
			return Ipv4
		} else { // ipv6
			return Ipv6_weak
		}
	} else if na.IP.To4() != nil {
		if o.Routable() && o.IP.To4() != nil {
			return Ipv4
		}
		return Default
	} else /* ipv6 */ {
		var tunnelled bool
		// Is our v6 is tunnelled?
		if o.RFC3964() || o.RFC6052() || o.RFC6145() {
			tunnelled = true
		}
		if !o.Routable() {
			return Default
		} else if o.RFC4380() {
			return Teredo
		} else if o.IP.To4() != nil {
			return Ipv4
		} else if tunnelled {
			// only prioritise ipv6 if we aren't tunnelling it.
			return Ipv6_weak
		}
		return Ipv6_strong
	}
}

// RFC1918: IPv4 Private networks (10.0.0.0/8, 192.168.0.0/16, 172.16.0.0/12)
// RFC3849: IPv6 Documentation address  (2001:0DB8::/32)
// RFC3927: IPv4 Autoconfig (169.254.0.0/16)
// RFC3964: IPv6 6to4 (2002::/16)
// RFC4193: IPv6 unique local (FC00::/7)
// RFC4380: IPv6 Teredo tunneling (2001::/32)
// RFC4843: IPv6 ORCHID: (2001:10::/28)
// RFC4862: IPv6 Autoconfig (FE80::/64)
// RFC6052: IPv6 well known prefix (64:FF9B::/96)
// RFC6145: IPv6 IPv4 translated address ::FFFF:0:0:0/96
var rfc1918_10 = net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(8, 32)}
var rfc1918_192 = net.IPNet{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)}
var rfc1918_172 = net.IPNet{IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(12, 32)}
var rfc3849 = net.IPNet{IP: net.ParseIP("2001:0DB8::"), Mask: net.CIDRMask(32, 128)}
var rfc3927 = net.IPNet{IP: net.ParseIP("169.254.0.0"), Mask: net.CIDRMask(16, 32)}
var rfc3964 = net.IPNet{IP: net.ParseIP("2002::"), Mask: net.CIDRMask(16, 128)}
var rfc4193 = net.IPNet{IP: net.ParseIP("FC00::"), Mask: net.CIDRMask(7, 128)}
var rfc4380 = net.IPNet{IP: net.ParseIP("2001::"), Mask: net.CIDRMask(32, 128)}
var rfc4843 = net.IPNet{IP: net.ParseIP("2001:10::"), Mask: net.CIDRMask(28, 128)}
var rfc4862 = net.IPNet{IP: net.ParseIP("FE80::"), Mask: net.CIDRMask(64, 128)}
var rfc6052 = net.IPNet{IP: net.ParseIP("64:FF9B::"), Mask: net.CIDRMask(96, 128)}
var rfc6145 = net.IPNet{IP: net.ParseIP("::FFFF:0:0:0"), Mask: net.CIDRMask(96, 128)}
var zero4 = net.IPNet{IP: net.ParseIP("0.0.0.0"), Mask: net.CIDRMask(8, 32)}

func (na *NetAddress) RFC1918() bool {
	return rfc1918_10.Contains(na.IP) ||
		rfc1918_192.Contains(na.IP) ||
		rfc1918_172.Contains(na.IP)
}
func (na *NetAddress) RFC3849() bool { return rfc3849.Contains(na.IP) }
func (na *NetAddress) RFC3927() bool { return rfc3927.Contains(na.IP) }
func (na *NetAddress) RFC3964() bool { return rfc3964.Contains(na.IP) }
func (na *NetAddress) RFC4193() bool { return rfc4193.Contains(na.IP) }
func (na *NetAddress) RFC4380() bool { return rfc4380.Contains(na.IP) }
func (na *NetAddress) RFC4843() bool { return rfc4843.Contains(na.IP) }
func (na *NetAddress) RFC4862() bool { return rfc4862.Contains(na.IP) }
func (na *NetAddress) RFC6052() bool { return rfc6052.Contains(na.IP) }
func (na *NetAddress) RFC6145() bool { return rfc6145.Contains(na.IP) }
