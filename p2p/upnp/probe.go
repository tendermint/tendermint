package upnp

import (
	"fmt"
	"net"
	"time"

	"github.com/tendermint/tendermint/libs/log"
)

type UPNPCapabilities struct {
	PortMapping bool
	Hairpin     bool
}

func makeUPNPListener(intPort int, extPort int, logger log.Logger) (NAT, net.Listener, net.IP, error) {
	nat, err := Discover()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("NAT upnp could not be discovered: %v", err)
	}
	logger.Info(fmt.Sprintf("ourIP: %v", nat.(*upnpNAT).ourIP))

	ext, err := nat.GetExternalAddress()
	if err != nil {
		return nat, nil, nil, fmt.Errorf("External address error: %v", err)
	}
	logger.Info(fmt.Sprintf("External address: %v", ext))

	port, err := nat.AddPortMapping("tcp", extPort, intPort, "Tendermint UPnP Probe", 0)
	if err != nil {
		return nat, nil, ext, fmt.Errorf("Port mapping error: %v", err)
	}
	logger.Info(fmt.Sprintf("Port mapping mapped: %v", port))

	// also run the listener, open for all remote addresses.
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", intPort))
	if err != nil {
		return nat, nil, ext, fmt.Errorf("Error establishing listener: %v", err)
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
	return
}

func Probe(logger log.Logger) (caps UPNPCapabilities, err error) {
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
