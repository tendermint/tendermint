package upnp

import (
	"errors"
	"fmt"
	"net"
	"time"

	. "github.com/tendermint/go-common"
)

type UPNPCapabilities struct {
	PortMapping bool
	Hairpin     bool
}

func makeUPNPListener(intPort int, extPort int) (NAT, net.Listener, net.IP, error) {
	nat, err := Discover()
	if err != nil {
		return nil, nil, nil, errors.New(fmt.Sprintf("NAT upnp could not be discovered: %v", err))
	}
	log.Info(Fmt("ourIP: %v", nat.(*upnpNAT).ourIP))

	ext, err := nat.GetExternalAddress()
	if err != nil {
		return nat, nil, nil, errors.New(fmt.Sprintf("External address error: %v", err))
	}
	log.Info(Fmt("External address: %v", ext))

	port, err := nat.AddPortMapping("tcp", extPort, intPort, "Tendermint UPnP Probe", 0)
	if err != nil {
		return nat, nil, ext, errors.New(fmt.Sprintf("Port mapping error: %v", err))
	}
	log.Info(Fmt("Port mapping mapped: %v", port))

	// also run the listener, open for all remote addresses.
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", intPort))
	if err != nil {
		return nat, nil, ext, errors.New(fmt.Sprintf("Error establishing listener: %v", err))
	}
	return nat, listener, ext, nil
}

func testHairpin(listener net.Listener, extAddr string) (supportsHairpin bool) {
	// Listener
	go func() {
		inConn, err := listener.Accept()
		if err != nil {
			log.Notice(Fmt("Listener.Accept() error: %v", err))
			return
		}
		log.Info(Fmt("Accepted incoming connection: %v -> %v", inConn.LocalAddr(), inConn.RemoteAddr()))
		buf := make([]byte, 1024)
		n, err := inConn.Read(buf)
		if err != nil {
			log.Notice(Fmt("Incoming connection read error: %v", err))
			return
		}
		log.Info(Fmt("Incoming connection read %v bytes: %X", n, buf))
		if string(buf) == "test data" {
			supportsHairpin = true
			return
		}
	}()

	// Establish outgoing
	outConn, err := net.Dial("tcp", extAddr)
	if err != nil {
		log.Notice(Fmt("Outgoing connection dial error: %v", err))
		return
	}

	n, err := outConn.Write([]byte("test data"))
	if err != nil {
		log.Notice(Fmt("Outgoing connection write error: %v", err))
		return
	}
	log.Info(Fmt("Outgoing connection wrote %v bytes", n))

	// Wait for data receipt
	time.Sleep(1 * time.Second)
	return
}

func Probe() (caps UPNPCapabilities, err error) {
	log.Info("Probing for UPnP!")

	intPort, extPort := 8001, 8001

	nat, listener, ext, err := makeUPNPListener(intPort, extPort)
	if err != nil {
		return
	}
	caps.PortMapping = true

	// Deferred cleanup
	defer func() {
		err = nat.DeletePortMapping("tcp", intPort, extPort)
		if err != nil {
			log.Warn(Fmt("Port mapping delete error: %v", err))
		}
		listener.Close()
	}()

	supportsHairpin := testHairpin(listener, fmt.Sprintf("%v:%v", ext, extPort))
	if supportsHairpin {
		caps.Hairpin = true
	}

	return
}
