package p2p

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p/upnp"
)

// Listener is a network listener for stream-oriented protocols, providing
// convenient methods to get listener's internal and external addresses.
// Clients are supposed to read incoming connections from a channel, returned
// by Connections() method.
type Listener interface {
	Connections() <-chan net.Conn
	InternalAddress() *NetAddress
	ExternalAddress() *NetAddress
	ExternalAddressHost() string
	String() string
	Stop() error
}

// DefaultListener is a cmn.Service, running net.Listener underneath.
// Optionally, UPnP is used upon calling NewDefaultListener to resolve external
// address.
type DefaultListener struct {
	cmn.BaseService

	listener    net.Listener
	intAddr     *NetAddress
	extAddr     *NetAddress
	connections chan net.Conn
}

var _ Listener = (*DefaultListener)(nil)

const (
	numBufferedConnections = 10
	defaultExternalPort    = 8770
	tryListenSeconds       = 5
)

func splitHostPort(addr string) (host string, port int) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	port, err = strconv.Atoi(portStr)
	if err != nil {
		panic(err)
	}
	return host, port
}

// NewDefaultListener creates a new DefaultListener on lAddr, optionally trying
// to determine external address using UPnP.
func NewDefaultListener(
	fullListenAddrString string,
	externalAddrString string,
	useUPnP bool,
	logger log.Logger) Listener {

	// Split protocol, address, and port.
	protocol, lAddr := cmn.ProtocolAndAddress(fullListenAddrString)
	lAddrIP, lAddrPort := splitHostPort(lAddr)

	// Create listener
	var listener net.Listener
	var err error
	for i := 0; i < tryListenSeconds; i++ {
		listener, err = net.Listen(protocol, lAddr)
		if err == nil {
			break
		} else if i < tryListenSeconds-1 {
			time.Sleep(time.Second * 1)
		}
	}
	if err != nil {
		panic(err)
	}
	// Actual listener local IP & port
	listenerIP, listenerPort := splitHostPort(listener.Addr().String())
	logger.Info("Local listener", "ip", listenerIP, "port", listenerPort)

	// Determine internal address...
	var intAddr *NetAddress
	intAddr, err = NewNetAddressStringWithOptionalID(lAddr)
	if err != nil {
		panic(err)
	}

	inAddrAny := lAddrIP == "" || lAddrIP == "0.0.0.0"

	// Determine external address.
	var extAddr *NetAddress

	if externalAddrString != "" {
		var err error
		extAddr, err = NewNetAddressStringWithOptionalID(externalAddrString)
		if err != nil {
			panic(fmt.Sprintf("Error in ExternalAddress: %v", err))
		}
	}

	// If the lAddrIP is INADDR_ANY, try UPnP.
	if extAddr == nil && useUPnP && inAddrAny {
		extAddr = getUPNPExternalAddress(lAddrPort, listenerPort, logger)
	}

	// Otherwise just use the local address.
	if extAddr == nil {
		defaultToIPv4 := inAddrAny
		extAddr = getNaiveExternalAddress(defaultToIPv4, listenerPort, false, logger)
	}
	if extAddr == nil {
		panic("Could not determine external address!")
	}

	dl := &DefaultListener{
		listener:    listener,
		intAddr:     intAddr,
		extAddr:     extAddr,
		connections: make(chan net.Conn, numBufferedConnections),
	}
	dl.BaseService = *cmn.NewBaseService(logger, "DefaultListener", dl)
	err = dl.Start() // Started upon construction
	if err != nil {
		logger.Error("Error starting base service", "err", err)
	}
	return dl
}

// OnStart implements cmn.Service by spinning a goroutine, listening for new
// connections.
func (l *DefaultListener) OnStart() error {
	if err := l.BaseService.OnStart(); err != nil {
		return err
	}
	go l.listenRoutine()
	return nil
}

// OnStop implements cmn.Service by closing the listener.
func (l *DefaultListener) OnStop() {
	l.BaseService.OnStop()
	l.listener.Close() // nolint: errcheck
}

// Accept connections and pass on the channel.
func (l *DefaultListener) listenRoutine() {
	for {
		conn, err := l.listener.Accept()

		if !l.IsRunning() {
			break // Go to cleanup
		}

		// listener wasn't stopped,
		// yet we encountered an error.
		if err != nil {
			panic(err)
		}

		l.connections <- conn
	}

	// Cleanup
	close(l.connections)
	for range l.connections {
		// Drain
	}
}

// Connections returns a channel of inbound connections.
// It gets closed when the listener closes.
// It is the callers responsibility to close any connections received
// over this channel.
func (l *DefaultListener) Connections() <-chan net.Conn {
	return l.connections
}

// InternalAddress returns the internal NetAddress (address used for
// listening).
func (l *DefaultListener) InternalAddress() *NetAddress {
	return l.intAddr
}

// ExternalAddress returns the external NetAddress (publicly available,
// determined using either UPnP or local resolver).
func (l *DefaultListener) ExternalAddress() *NetAddress {
	return l.extAddr
}

// ExternalAddressHost returns the external NetAddress IP string. If an IP is
// IPv6, it's wrapped in brackets ("[2001:db8:1f70::999:de8:7648:6e8]").
func (l *DefaultListener) ExternalAddressHost() string {
	ip := l.ExternalAddress().IP
	if isIpv6(ip) {
		// Means it's ipv6, so format it with brackets
		return "[" + ip.String() + "]"
	}
	return ip.String()
}

func (l *DefaultListener) String() string {
	return fmt.Sprintf("Listener(@%v)", l.extAddr)
}

/* external address helpers */

// UPNP external address discovery & port mapping
func getUPNPExternalAddress(externalPort, internalPort int, logger log.Logger) *NetAddress {
	logger.Info("Getting UPNP external address")
	nat, err := upnp.Discover()
	if err != nil {
		logger.Info("Could not perform UPNP discover", "err", err)
		return nil
	}

	ext, err := nat.GetExternalAddress()
	if err != nil {
		logger.Info("Could not get UPNP external address", "err", err)
		return nil
	}

	// UPnP can't seem to get the external port, so let's just be explicit.
	if externalPort == 0 {
		externalPort = defaultExternalPort
	}

	externalPort, err = nat.AddPortMapping("tcp", externalPort, internalPort, "tendermint", 0)
	if err != nil {
		logger.Info("Could not add UPNP port mapping", "err", err)
		return nil
	}

	logger.Info("Got UPNP external address", "address", ext)
	return NewNetAddressIPPort(ext, uint16(externalPort))
}

func isIpv6(ip net.IP) bool {
	v4 := ip.To4()
	if v4 != nil {
		return false
	}

	ipString := ip.String()

	// Extra check just to be sure it's IPv6
	return (strings.Contains(ipString, ":") && !strings.Contains(ipString, "."))
}

// TODO: use syscalls: see issue #712
func getNaiveExternalAddress(defaultToIPv4 bool, port int, settleForLocal bool, logger log.Logger) *NetAddress {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(cmn.Fmt("Could not fetch interface addresses: %v", err))
	}

	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if defaultToIPv4 || !isIpv6(ipnet.IP) {
			v4 := ipnet.IP.To4()
			if v4 == nil || (!settleForLocal && v4[0] == 127) {
				// loopback
				continue
			}
		} else if !settleForLocal && ipnet.IP.IsLoopback() {
			// IPv6, check for loopback
			continue
		}
		return NewNetAddressIPPort(ipnet.IP, uint16(port))
	}

	// try again, but settle for local
	logger.Info("Node may not be connected to internet. Settling for local address")
	return getNaiveExternalAddress(defaultToIPv4, port, true, logger)
}
