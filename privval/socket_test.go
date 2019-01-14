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

func TestTCPListenerAcceptDeadline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	tcpLn := NewTCPListener(ln, newPrivKey())
	TCPListenerAcceptDeadline(time.Millisecond)(tcpLn)
	TCPListenerConnDeadline(time.Second)(tcpLn)

	_, err = tcpLn.Accept()
	opErr, ok := err.(*net.OpError)
	if !ok {
		t.Fatalf("have %v, want *net.OpError", err)
	}

	if have, want := opErr.Op, "accept"; have != want {
		t.Errorf("have %v, want %v", have, want)
	}
}

func TestTCPListenerConnDeadline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	tcpLn := NewTCPListener(ln, newPrivKey())
	TCPListenerAcceptDeadline(time.Second)(tcpLn)
	TCPListenerConnDeadline(time.Millisecond)(tcpLn)

	readyc := make(chan struct{})
	donec := make(chan struct{})
	go func(ln net.Listener) {
		defer close(donec)

		c, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		<-readyc

		time.Sleep(2 * time.Millisecond)

		msg := make([]byte, 200)
		_, err = c.Read(msg)
		opErr, ok := err.(*net.OpError)
		if !ok {
			t.Fatalf("have %v, want *net.OpError", err)
		}

		if have, want := opErr.Op, "read"; have != want {
			t.Errorf("have %v, want %v", have, want)
		}
	}(tcpLn)

	dialer := DialTCPFn(ln.Addr().String(), testConnDeadline, newPrivKey())
	_, err = dialer()
	if err != nil {
		t.Fatal(err)
	}
	close(readyc)
	<-donec
}

// testUnixAddr will attempt to obtain a platform-independent temporary file
// name for a Unix socket
func testUnixAddr() (string, error) {
	f, err := ioutil.TempFile("", "tendermint-privval-test")
	if err != nil {
		return "", err
	}
	addr := f.Name()
	f.Close()
	os.Remove(addr)
	return addr, nil
}

func TestUnixListenerAcceptDeadline(t *testing.T) {
	addr, err := testUnixAddr()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatal(err)
	}

	unixLn := NewUnixListener(ln)
	UnixListenerAcceptDeadline(time.Millisecond)(unixLn)
	UnixListenerConnDeadline(time.Second)(unixLn)

	_, err = unixLn.Accept()
	opErr, ok := err.(*net.OpError)
	if !ok {
		t.Fatalf("have %v, want *net.OpError", err)
	}

	if have, want := opErr.Op, "accept"; have != want {
		t.Errorf("have %v, want %v", have, want)
	}
}

func TestUnixListenerConnDeadline(t *testing.T) {
	addr, err := testUnixAddr()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatal(err)
	}

	unixLn := NewUnixListener(ln)
	UnixListenerAcceptDeadline(time.Second)(unixLn)
	UnixListenerConnDeadline(time.Millisecond)(unixLn)

	readyc := make(chan struct{})
	donec := make(chan struct{})
	go func(ln net.Listener) {
		defer close(donec)

		c, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		<-readyc

		time.Sleep(2 * time.Millisecond)

		msg := make([]byte, 200)
		_, err = c.Read(msg)
		opErr, ok := err.(*net.OpError)
		if !ok {
			t.Fatalf("have %v, want *net.OpError", err)
		}

		if have, want := opErr.Op, "read"; have != want {
			t.Errorf("have %v, want %v", have, want)
		}
	}(unixLn)

	dialer := DialUnixFn(addr)
	_, err = dialer()
	if err != nil {
		t.Fatal(err)
	}
	close(readyc)
	<-donec
}
