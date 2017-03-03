package p2p

import (
	"fmt"
	"net"
	"strconv"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-p2p/upnp"
)

type Listener interface {
	Connections() <-chan net.Conn
	InternalAddress() *NetAddress
	ExternalAddress() *NetAddress
	String() string
	Stop() bool
}

// Implements Listener
type DefaultListener struct {
	BaseService

	listener    net.Listener
	intAddr     *NetAddress
	extAddr     *NetAddress
	connections chan net.Conn
}

const (
	numBufferedConnections = 10
	defaultExternalPort    = 8770
	tryListenSeconds       = 5
)

func splitHostPort(addr string) (host string, port int) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		PanicSanity(err)
	}
	port, err = strconv.Atoi(portStr)
	if err != nil {
		PanicSanity(err)
	}
	return host, port
}

// skipUPNP: If true, does not try getUPNPExternalAddress()
func NewDefaultListener(protocol string, lAddr string, skipUPNP bool) Listener {
	// Local listen IP & port
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
		PanicCrisis(err)
	}
	// Actual listener local IP & port
	listenerIP, listenerPort := splitHostPort(listener.Addr().String())
	log.Info("Local listener", "ip", listenerIP, "port", listenerPort)

	// Determine internal address...
	var intAddr *NetAddress
	intAddr, err = NewNetAddressString(lAddr)
	if err != nil {
		PanicCrisis(err)
	}

	// Determine external address...
	var extAddr *NetAddress
	if !skipUPNP {
		// If the lAddrIP is INADDR_ANY, try UPnP
		if lAddrIP == "" || lAddrIP == "0.0.0.0" {
			extAddr = getUPNPExternalAddress(lAddrPort, listenerPort)
		}
	}
	// Otherwise just use the local address...
	if extAddr == nil {
		extAddr = getNaiveExternalAddress(listenerPort)
	}
	if extAddr == nil {
		PanicCrisis("Could not determine external address!")
	}

	dl := &DefaultListener{
		listener:    listener,
		intAddr:     intAddr,
		extAddr:     extAddr,
		connections: make(chan net.Conn, numBufferedConnections),
	}
	dl.BaseService = *NewBaseService(log, "DefaultListener", dl)
	dl.Start() // Started upon construction
	return dl
}

func (l *DefaultListener) OnStart() error {
	l.BaseService.OnStart()
	go l.listenRoutine()
	return nil
}

func (l *DefaultListener) OnStop() {
	l.BaseService.OnStop()
	l.listener.Close()
}

// Accept connections and pass on the channel
func (l *DefaultListener) listenRoutine() {
	for {
		conn, err := l.listener.Accept()

		if !l.IsRunning() {
			break // Go to cleanup
		}

		// listener wasn't stopped,
		// yet we encountered an error.
		if err != nil {
			PanicCrisis(err)
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

func (l *DefaultListener) InternalAddress() *NetAddress {
	return l.intAddr
}

func (l *DefaultListener) ExternalAddress() *NetAddress {
	return l.extAddr
}

// NOTE: The returned listener is already Accept()'ing.
// So it's not suitable to pass into http.Serve().
func (l *DefaultListener) NetListener() net.Listener {
	return l.listener
}

func (l *DefaultListener) String() string {
	return fmt.Sprintf("Listener(@%v)", l.extAddr)
}

/* external address helpers */

// UPNP external address discovery & port mapping
func getUPNPExternalAddress(externalPort, internalPort int) *NetAddress {
	log.Info("Getting UPNP external address")
	nat, err := upnp.Discover()
	if err != nil {
		log.Info("Could not perform UPNP discover", "error", err)
		return nil
	}

	ext, err := nat.GetExternalAddress()
	if err != nil {
		log.Info("Could not get UPNP external address", "error", err)
		return nil
	}

	// UPnP can't seem to get the external port, so let's just be explicit.
	if externalPort == 0 {
		externalPort = defaultExternalPort
	}

	externalPort, err = nat.AddPortMapping("tcp", externalPort, internalPort, "tendermint", 0)
	if err != nil {
		log.Info("Could not add UPNP port mapping", "error", err)
		return nil
	}

	log.Info("Got UPNP external address", "address", ext)
	return NewNetAddressIPPort(ext, uint16(externalPort))
}

// TODO: use syscalls: http://pastebin.com/9exZG4rh
func getNaiveExternalAddress(port int) *NetAddress {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		PanicCrisis(Fmt("Could not fetch interface addresses: %v", err))
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
