package privval

import (
	"net"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	p2pconn "github.com/tendermint/tendermint/p2p/conn"
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

	donec := make(chan struct{})
	go func(ln net.Listener) {
		defer close(donec)

		c, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}

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

	// connect and start secret conn
	conn, err := cmn.Connect(ln.Addr().String())
	if err == nil {
		err = conn.SetDeadline(time.Now().Add(connTimeout))
	}
	if err == nil {
		_, err = p2pconn.MakeSecretConnection(conn, newPrivKey())
	}
	if err != nil {
		t.Fatal(err)
	}

	<-donec
}
