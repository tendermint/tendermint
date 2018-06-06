package privval

import (
	"net"
	"testing"
	"time"
)

func TestTCPTimeoutListenerAcceptDeadline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	ln = newTCPTimeoutListener(ln, time.Millisecond, time.Second, time.Second)

	_, err = ln.Accept()
	opErr, ok := err.(*net.OpError)
	if !ok {
		t.Fatalf("have %v, want *net.OpError", err)
	}

	if have, want := opErr.Op, "accept"; have != want {
		t.Errorf("have %v, want %v", have, want)
	}
}

func TestTCPTimeoutListenerConnDeadline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	ln = newTCPTimeoutListener(ln, time.Second, time.Millisecond, time.Second)

	donec := make(chan struct{})
	go func(ln net.Listener) {
		defer close(donec)

		c, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(2 * time.Millisecond)

		_, err = c.Write([]byte("foo"))
		opErr, ok := err.(*net.OpError)
		if !ok {
			t.Fatalf("have %v, want *net.OpError", err)
		}

		if have, want := opErr.Op, "write"; have != want {
			t.Errorf("have %v, want %v", have, want)
		}
	}(ln)

	_, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	<-donec
}
