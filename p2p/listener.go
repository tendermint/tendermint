package p2p

import (
	"fmt"
	"net"
	"strconv"

	inet "github.com/libp2p/go-libp2p-net"
	swarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	ma "github.com/multiformats/go-multiaddr"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
)

type Listener interface {
	Connections() <-chan inet.Stream
	ExternalAddress() ma.Multiaddr
	String() string
	Stop() error
}

// Implements Listener
type DefaultListener struct {
	cmn.BaseService

	host        *bhost.BasicHost
	connections chan inet.Stream
}

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

// skipUPNP: If true, does not try getUPNPExternalAddress()
func NewDefaultListener(
	localMa []ma.Multiaddr,
	skipUPNP bool,
	logger log.Logger,
	snet *swarm.Network,
	bh *bhost.BasicHost,
) (Listener, error) {
	logger.Info("Building default swarm handler")
	for _, ma := range localMa {
		logger.Info("Adding local swarm listener", "addr", ma.String())
	}
	if err := snet.Listen(localMa...); err != nil {
		return nil, err
	}

	dl := &DefaultListener{
		host:        bh,
		connections: make(chan inet.Stream, numBufferedConnections),
	}

	dl.BaseService = *cmn.NewBaseService(logger, "DefaultListener", dl)
	// Started upon construction
	if err := dl.Start(); err != nil {
		logger.Error("Error starting base service", "err", err)
		return dl, err
	}

	return dl, nil
}

func (l *DefaultListener) OnStart() error {
	if err := l.BaseService.OnStart(); err != nil {
		return err
	}
	l.host.SetStreamHandler(Protocol, l.streamHandler)
	return nil
}

// streamHandler handles incoming streams on the protocol.
func (l *DefaultListener) streamHandler(stream inet.Stream) {
	if !l.IsRunning() {
		stream.Close()
		return
	}

	l.connections <- stream
}

func (l *DefaultListener) OnStop() {
	l.BaseService.OnStop()
	l.host.Close()
}

// A channel of inbound connections.
// It gets closed when the listener closes.
func (l *DefaultListener) Connections() <-chan inet.Stream {
	return l.connections
}

func (l *DefaultListener) ExternalAddress() ma.Multiaddr {
	return l.host.Addrs()[0]
}

func (l *DefaultListener) String() string {
	return fmt.Sprintf("Listener(@%v)", l.ExternalAddress().String())
}

/* external address helpers */
func getNaiveExternalAddress(port int, settleForLocal bool, logger log.Logger) ma.Multiaddr {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(cmn.Fmt("Could not fetch interface addresses: %v", err))
	}

	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		v4 := ipnet.IP.To4()
		if v4 == nil || (!settleForLocal && v4[0] == 127) {
			continue
		} // loopback
		maddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", ipnet.IP.String(), port))
		if err != nil {
			cmn.PanicCrisis(err)
		}
		return maddr
	}

	// try again, but settle for local
	logger.Info("Node may not be connected to internet. Settling for local address")
	return getNaiveExternalAddress(port, true, logger)
}
