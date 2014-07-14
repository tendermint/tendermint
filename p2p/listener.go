package p2p

import (
	"fmt"
	"net"
	"strconv"
	"sync/atomic"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/p2p/upnp"
)

/*
Listener is part of a Server.
*/
type Listener interface {
	Connections() <-chan *Connection
	ExternalAddress() *NetAddress
	Stop()
}

/*
DefaultListener is an implementation that works on the golang network stack.
*/
type DefaultListener struct {
	listener    net.Listener
	extAddr     *NetAddress
	connections chan *Connection
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

func NewDefaultListener(protocol string, lAddr string) Listener {
	// Local listen IP & port
	lAddrIP, lAddrPort := splitHostPort(lAddr)

	// Create listener
	listener, err := net.Listen(protocol, lAddr)
	if err != nil {
		panic(err)
	}
	// Actual listener local IP & port
	listenerIP, listenerPort := splitHostPort(listener.Addr().String())
	log.Infof("Local listener: %v:%v", listenerIP, listenerPort)

	// Determine external address...
	var extAddr *NetAddress
	// If the lAddrIP is INADDR_ANY, try UPnP
	if lAddrIP == "" || lAddrIP == "0.0.0.0" {
		extAddr = getUPNPExternalAddress(lAddrPort, listenerPort)
	}
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
		connections: make(chan *Connection, numBufferedConnections),
	}

	go dl.listenHandler()

	return dl
}

func (l *DefaultListener) listenHandler() {
	for {
		conn, err := l.listener.Accept()

		if atomic.LoadUint32(&l.stopped) == 1 {
			break // go to cleanup
		}

		// listener wasn't stopped,
		// yet we encountered an error.
		if err != nil {
			panic(err)
		}

		c := NewConnection(conn)
		l.connections <- c
	}

	// cleanup
	close(l.connections)
	for _ = range l.connections {
		// drain
	}
}

// A channel of inbound connections.
// It gets closed when the listener closes.
func (l *DefaultListener) Connections() <-chan *Connection {
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
	log.Infof("Getting UPNP external address")
	nat, err := upnp.Discover()
	if err != nil {
		log.Infof("Could not get UPNP extrernal address: %v", err)
		return nil
	}

	ext, err := nat.GetExternalAddress()
	if err != nil {
		log.Infof("Could not get UPNP external address: %v", err)
		return nil
	}

	// UPnP can't seem to get the external port, so let's just be explicit.
	if externalPort == 0 {
		externalPort = defaultExternalPort
	}

	externalPort, err = nat.AddPortMapping("tcp", externalPort, internalPort, "tendermint", 0)
	if err != nil {
		log.Infof("Could not get UPNP external address: %v", err)
		return nil
	}

	log.Infof("Got UPNP external address: %v", ext)
	return NewNetAddressIPPort(ext, UInt16(externalPort))
}

// TODO: use syscalls: http://pastebin.com/9exZG4rh
func getNaiveExternalAddress(port int) *NetAddress {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		Panicf("Unexpected error fetching interface addresses: %v", err)
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
		return NewNetAddressIPPort(ipnet.IP, UInt16(port))
	}
	return nil
}
