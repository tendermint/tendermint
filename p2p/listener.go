package p2p

import (
	"fmt"
	"net"
	"strconv"
	"sync/atomic"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/p2p/upnp"
)

type Listener interface {
	Connections() <-chan net.Conn
	ExternalAddress() *NetAddress
	Stop()
}

// Implements Listener
type DefaultListener struct {
	listener    net.Listener
	extAddr     *NetAddress
	connections chan net.Conn
	stopped     uint32
}

const (
	numBufferedConnections = 10
	defaultExternalPort    = 8770
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

func NewDefaultListener(protocol string, lAddr string, requireUPNPHairpin bool) Listener {
	// Local listen IP & port
	lAddrIP, lAddrPort := splitHostPort(lAddr)

	// Create listener
	listener, err := net.Listen(protocol, lAddr)
	if err != nil {
		panic(err)
	}
	// Actual listener local IP & port
	listenerIP, listenerPort := splitHostPort(listener.Addr().String())
	log.Debug("Local listener", "ip", listenerIP, "port", listenerPort)

	// Determine external address...
	var extAddr *NetAddress

	// If the lAddrIP is INADDR_ANY, try UPnP
	if lAddrIP == "" || lAddrIP == "0.0.0.0" {
		if requireUPNPHairpin {
			upnpCapabilities, err := upnp.Probe()
			if err != nil {
				log.Warn("Failed to probe UPNP", "error", err)
				goto SKIP_UPNP
			}
			if !upnpCapabilities.Hairpin {
				goto SKIP_UPNP
			}
		}
		extAddr = getUPNPExternalAddress(lAddrPort, listenerPort)
	}
SKIP_UPNP:

	// Otherwise just use the local address...
	if extAddr == nil {
		extAddr = getNaiveExternalAddress(listenerPort)
	}
	if extAddr == nil {
		panic("Could not determine external address!")
	}

	dl := &DefaultListener{
		listener:    listener,
		extAddr:     extAddr,
		connections: make(chan net.Conn, numBufferedConnections),
	}

	go dl.listenRoutine()

	return dl
}

// TODO: prevent abuse, esp a bunch of connections coming from the same IP range.
func (l *DefaultListener) listenRoutine() {
	for {
		conn, err := l.listener.Accept()

		if atomic.LoadUint32(&l.stopped) == 1 {
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
	for _ = range l.connections {
		// Drain
	}
}

// A channel of inbound connections.
// It gets closed when the listener closes.
func (l *DefaultListener) Connections() <-chan net.Conn {
	return l.connections
}

func (l *DefaultListener) ExternalAddress() *NetAddress {
	return l.extAddr
}

func (l *DefaultListener) Stop() {
	if atomic.CompareAndSwapUint32(&l.stopped, 0, 1) {
		l.listener.Close()
	}
}

func (l *DefaultListener) String() string {
	return fmt.Sprintf("Listener(@%v)", l.extAddr)
}

/* external address helpers */

// UPNP external address discovery & port mapping
func getUPNPExternalAddress(externalPort, internalPort int) *NetAddress {
	log.Debug("Getting UPNP external address")
	nat, err := upnp.Discover()
	if err != nil {
		log.Debug("Could not get UPNP extrernal address", "error", err)
		return nil
	}

	ext, err := nat.GetExternalAddress()
	if err != nil {
		log.Debug("Could not get UPNP external address", "error", err)
		return nil
	}

	// UPnP can't seem to get the external port, so let's just be explicit.
	if externalPort == 0 {
		externalPort = defaultExternalPort
	}

	externalPort, err = nat.AddPortMapping("tcp", externalPort, internalPort, "tendermint", 0)
	if err != nil {
		log.Debug("Could not get UPNP external address", "error", err)
		return nil
	}

	log.Debug("Got UPNP external address", "address", ext)
	return NewNetAddressIPPort(ext, uint16(externalPort))
}

// TODO: use syscalls: http://pastebin.com/9exZG4rh
func getNaiveExternalAddress(port int) *NetAddress {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(Fmt("Could not fetch interface addresses: %v", err))
	}

	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		v4 := ipnet.IP.To4()
		if v4 == nil || v4[0] == 127 {
			continue
		} // loopback
		return NewNetAddressIPPort(ipnet.IP, uint16(port))
	}
	return nil
}
