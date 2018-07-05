package p2p

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

func TestListener(t *testing.T) {
	// Create a listener
	l := NewDefaultListener("tcp://:8001", "", false, log.TestingLogger())

	// Dial the listener
	lAddr := l.ExternalAddress()
	connOut, err := lAddr.Dial()
	if err != nil {
		t.Fatalf("Could not connect to listener address %v", lAddr)
	} else {
		t.Logf("Created a connection to listener address %v", lAddr)
	}
	connIn, ok := <-l.Connections()
	if !ok {
		t.Fatalf("Could not get inbound connection from listener")
	}

	msg := []byte("hi!")
	go func() {
		_, err := connIn.Write(msg)
		if err != nil {
			t.Error(err)
		}
	}()
	b := make([]byte, 32)
	n, err := connOut.Read(b)
	if err != nil {
		t.Fatalf("Error reading off connection: %v", err)
	}

	b = b[:n]
	if !bytes.Equal(msg, b) {
		t.Fatalf("Got %s, expected %s", b, msg)
	}

	// Close the server, no longer needed.
	l.Stop()
}

func TestExternalAddress(t *testing.T) {
	{
		// Create a listener with no external addr. Should default
		// to local ipv4.
		l := NewDefaultListener("tcp://:8001", "", false, log.TestingLogger())
		lAddr := l.ExternalAddress().String()
		_, _, err := net.SplitHostPort(lAddr)
		require.Nil(t, err)
		spl := strings.Split(lAddr, ".")
		require.Equal(t, len(spl), 4)
		l.Stop()
	}

	{
		// Create a listener with set external ipv4 addr.
		setExAddr := "8.8.8.8:8080"
		l := NewDefaultListener("tcp://:8001", setExAddr, false, log.TestingLogger())
		lAddr := l.ExternalAddress().String()
		require.Equal(t, lAddr, setExAddr)
		l.Stop()
	}

	{
		// Invalid external addr causes panic
		setExAddr := "awrlsckjnal:8080"
		require.Panics(t, func() { NewDefaultListener("tcp://:8001", setExAddr, false, log.TestingLogger()) })
	}
}
