package privval

import (
	"net"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
)

func TestTCPListenerAcceptDeadline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	ln = NewTCPListener(ln, time.Millisecond, time.Second, newPrivKey())

	_, err = ln.Accept()
	opErr, ok := err.(*net.OpError)
	if !ok {
		t.Fatalf("have %v, want *net.OpError", err)
	}

	if have, want := opErr.Op, "accept"; have != want {
		t.Errorf("have %v, want %v", have, want)
	}
}

func newPrivKey() ed25519.PrivKeyEd25519 {
	return ed25519.GenPrivKey()
}

func TestTCPListenerConnDeadline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	ln = NewTCPListener(ln, time.Second, time.Millisecond, newPrivKey())

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
	}(ln)

	dialer := dialTCPFn(ln.Addr().String(), connTimeout, newPrivKey())
	_, err = dialer()
	if err != nil {
		t.Fatal(err)
	}
	close(readyc)
	<-donec
}
