package upnp

import (
	"net"
	"testing"
	"time"
)

/*
This is a manual test.
TODO: set up or find a service to probe open ports.
*/
func TestUPNP(t *testing.T) {
	t.Log("hello!")

	nat, err := Discover()
	if err != nil {
		t.Fatalf("NAT upnp could not be discovered: %v", err)
	}

	t.Log("ourIP: ", nat.(*upnpNAT).ourIP)

	ext, err := nat.GetExternalAddress()
	if err != nil {
		t.Fatalf("External address error: %v", err)
	}
	t.Logf("External address: %v", ext)

	port, err := nat.AddPortMapping("tcp", 8001, 8001, "testing", 0)
	if err != nil {
		t.Fatalf("Port mapping error: %v", err)
	}
	t.Logf("Port mapping mapped: %v", port)

	// also run the listener, open for all remote addresses.
	listener, err := net.Listen("tcp", ":8001")
	if err != nil {
		panic(err)
	}

	// now sleep for 10 seconds
	time.Sleep(10 * time.Second)

	err = nat.DeletePortMapping("tcp", 8001, 8001)
	if err != nil {
		t.Fatalf("Port mapping delete error: %v", err)
	}
	t.Logf("Port mapping deleted")

	listener.Close()
}
