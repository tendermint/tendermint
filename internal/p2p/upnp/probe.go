package upnp

import (
	"fmt"
	"net"
	"time"

	"github.com/tendermint/tendermint/libs/log"
)

type Capabilities struct {
	PortMapping bool
	Hairpin     bool
}

func makeUPNPListener(intPort int, extPort int, logger log.Logger) (NAT, net.Listener, net.IP, error) {
	nat, err := Discover()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("nat upnp could not be discovered: %v", err)
	}
	logger.Info(fmt.Sprintf("ourIP: %v", nat.(*upnpNAT).ourIP))

	ext, err := nat.GetExternalAddress()
	if err != nil {
		return nat, nil, nil, fmt.Errorf("external address error: %v", err)
	}
	logger.Info(fmt.Sprintf("External address: %v", ext))

	port, err := nat.AddPortMapping("tcp", extPort, intPort, "Tendermint UPnP Probe", 0)
	if err != nil {
		return nat, nil, ext, fmt.Errorf("port mapping error: %v", err)
	}
	logger.Info(fmt.Sprintf("Port mapping mapped: %v", port))

	// also run the listener, open for all remote addresses.
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", intPort))
	if err != nil {
		return nat, nil, ext, fmt.Errorf("error establishing listener: %v", err)
	}
	return nat, listener, ext, nil
}

func testHairpin(listener net.Listener, extAddr string, logger log.Logger) (supportsHairpin bool) {
	// Listener
	go func() {
		inConn, err := listener.Accept()
		if err != nil {
			logger.Info(fmt.Sprintf("Listener.Accept() error: %v", err))
			return
		}
		logger.Info(fmt.Sprintf("Accepted incoming connection: %v -> %v", inConn.LocalAddr(), inConn.RemoteAddr()))
		buf := make([]byte, 1024)
		n, err := inConn.Read(buf)
		if err != nil {
			logger.Info(fmt.Sprintf("Incoming connection read error: %v", err))
			return
		}
		logger.Info(fmt.Sprintf("Incoming connection read %v bytes: %X", n, buf))
		if string(buf) == "test data" {
			supportsHairpin = true
			return
		}
	}()

	// Establish outgoing
	outConn, err := net.Dial("tcp", extAddr)
	if err != nil {
		logger.Info(fmt.Sprintf("Outgoing connection dial error: %v", err))
		return
	}

	n, err := outConn.Write([]byte("test data"))
	if err != nil {
		logger.Info(fmt.Sprintf("Outgoing connection write error: %v", err))
		return
	}
	logger.Info(fmt.Sprintf("Outgoing connection wrote %v bytes", n))

	// Wait for data receipt
	time.Sleep(1 * time.Second)
	return supportsHairpin
}

func Probe(logger log.Logger) (caps Capabilities, err error) {
	logger.Info("Probing for UPnP!")

	intPort, extPort := 8001, 8001

	nat, listener, ext, err := makeUPNPListener(intPort, extPort, logger)
	if err != nil {
		return
	}
	caps.PortMapping = true

	// Deferred cleanup
	defer func() {
		if err := nat.DeletePortMapping("tcp", intPort, extPort); err != nil {
			logger.Error(fmt.Sprintf("Port mapping delete error: %v", err))
		}
		if err := listener.Close(); err != nil {
			logger.Error(fmt.Sprintf("Listener closing error: %v", err))
		}
	}()

	supportsHairpin := testHairpin(listener, fmt.Sprintf("%v:%v", ext, extPort), logger)
	if supportsHairpin {
		caps.Hairpin = true
	}

	return
}
