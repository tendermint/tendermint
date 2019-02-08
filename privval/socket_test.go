package privval

import (
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
)

//-------------------------------------------
// helper funcs

func newPrivKey() ed25519.PrivKeyEd25519 {
	return ed25519.GenPrivKey()
}

//-------------------------------------------
// tests

type listenerTestCase struct {
	description string // For test reporting purposes.
	listener    net.Listener
	dialer      Dialer
}

// testUnixAddr will attempt to obtain a platform-independent temporary file
// name for a Unix socket
func testUnixAddr() (string, error) {
	f, err := ioutil.TempFile("", "tendermint-privval-test-*")
	if err != nil {
		return "", err
	}
	addr := f.Name()
	f.Close()
	os.Remove(addr)
	return addr, nil
}

func tcpListenerTestCase(t *testing.T, acceptDeadline, connectDeadline time.Duration) listenerTestCase {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	tcpLn := NewTCPListener(ln, newPrivKey())
	TCPListenerAcceptDeadline(acceptDeadline)(tcpLn)
	TCPListenerConnDeadline(connectDeadline)(tcpLn)
	return listenerTestCase{
		description: "TCP",
		listener:    tcpLn,
		dialer:      DialTCPFn(ln.Addr().String(), testConnDeadline, newPrivKey()),
	}
}

func unixListenerTestCase(t *testing.T, acceptDeadline, connectDeadline time.Duration) listenerTestCase {
	addr, err := testUnixAddr()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatal(err)
	}

	unixLn := NewUnixListener(ln)
	UnixListenerAcceptDeadline(acceptDeadline)(unixLn)
	UnixListenerConnDeadline(connectDeadline)(unixLn)
	return listenerTestCase{
		description: "Unix",
		listener:    unixLn,
		dialer:      DialUnixFn(addr),
	}
}

func listenerTestCases(t *testing.T, acceptDeadline, connectDeadline time.Duration) []listenerTestCase {
	return []listenerTestCase{
		tcpListenerTestCase(t, acceptDeadline, connectDeadline),
		unixListenerTestCase(t, acceptDeadline, connectDeadline),
	}
}

func TestListenerAcceptDeadlines(t *testing.T) {
	for _, tc := range listenerTestCases(t, time.Millisecond, time.Second) {
		_, err := tc.listener.Accept()
		opErr, ok := err.(*net.OpError)
		if !ok {
			t.Fatalf("for %s listener, have %v, want *net.OpError", tc.description, err)
		}

		if have, want := opErr.Op, "accept"; have != want {
			t.Errorf("for %s listener,  have %v, want %v", tc.description, have, want)
		}
	}
}

func TestListenerConnectDeadlines(t *testing.T) {
	for _, tc := range listenerTestCases(t, time.Second, time.Millisecond) {
		go func(dialer Dialer) {
			_, err := dialer()
			if err != nil {
				panic(err)
			}
		}(tc.dialer)

		c, err := tc.listener.Accept()
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(2 * time.Millisecond)

		msg := make([]byte, 200)
		_, err = c.Read(msg)
		opErr, ok := err.(*net.OpError)
		if !ok {
			t.Fatalf("for %s listener, have %v, want *net.OpError", tc.description, err)
		}

		if have, want := opErr.Op, "read"; have != want {
			t.Errorf("for %s listener, have %v, want %v", tc.description, have, want)
		}
	}
}
