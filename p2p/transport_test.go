package p2p

import (
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/p2p/conn"
)

var defaultNodeName = "host_peer"

func emptyNodeInfo() NodeInfo {
	return DefaultNodeInfo{}
}

// newMultiplexTransport returns a tcp connected multiplexed peer
// using the default MConnConfig. It's a convenience function used
// for testing.
func newMultiplexTransport(
	nodeInfo NodeInfo,
	nodeKey NodeKey,
) *MultiplexTransport {
	return NewMultiplexTransport(
		nodeInfo, nodeKey, conn.DefaultMConnConfig(),
	)
}

func TestTransportMultiplexConnFilter(t *testing.T) {
	mt := newMultiplexTransport(
		emptyNodeInfo(),
		NodeKey{
			PrivKey: ed25519.GenPrivKey(),
		},
	)
	id := mt.nodeKey.ID()

	MultiplexTransportConnFilters(
		func(_ ConnSet, _ net.Conn, _ []net.IP) error { return nil },
		func(_ ConnSet, _ net.Conn, _ []net.IP) error { return nil },
		func(_ ConnSet, _ net.Conn, _ []net.IP) error {
			return fmt.Errorf("rejected")
		},
	)(mt)

	addr, err := NewNetAddressString(IDAddressString(id, "127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}

	if err := mt.Listen(*addr); err != nil {
		t.Fatal(err)
	}

	errc := make(chan error)

	go func() {
		addr := NewNetAddress(id, mt.listener.Addr())

		_, err := addr.Dial()
		if err != nil {
			errc <- err
			return
		}

		close(errc)
	}()

	if err := <-errc; err != nil {
		t.Errorf("connection failed: %v", err)
	}

	_, err = mt.Accept(peerConfig{})
	if err, ok := err.(ErrRejected); ok {
		if !err.IsFiltered() {
			t.Errorf("expected peer to be filtered")
		}
	} else {
		t.Errorf("expected ErrRejected")
	}
}

func TestTransportMultiplexConnFilterTimeout(t *testing.T) {
	mt := newMultiplexTransport(
		emptyNodeInfo(),
		NodeKey{
			PrivKey: ed25519.GenPrivKey(),
		},
	)
	id := mt.nodeKey.ID()

	MultiplexTransportFilterTimeout(5 * time.Millisecond)(mt)
	MultiplexTransportConnFilters(
		func(_ ConnSet, _ net.Conn, _ []net.IP) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	)(mt)

	addr, err := NewNetAddressString(IDAddressString(id, "127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}

	if err := mt.Listen(*addr); err != nil {
		t.Fatal(err)
	}

	errc := make(chan error)

	go func() {
		addr := NewNetAddress(id, mt.listener.Addr())

		_, err := addr.Dial()
		if err != nil {
			errc <- err
			return
		}

		close(errc)
	}()

	if err := <-errc; err != nil {
		t.Errorf("connection failed: %v", err)
	}

	_, err = mt.Accept(peerConfig{})
	if _, ok := err.(ErrFilterTimeout); !ok {
		t.Errorf("expected ErrFilterTimeout")
	}
}

func TestTransportMultiplexAcceptMultiple(t *testing.T) {
	mt := testSetupMultiplexTransport(t)
	laddr := NewNetAddress(mt.nodeKey.ID(), mt.listener.Addr())

	var (
		seed     = rand.New(rand.NewSource(time.Now().UnixNano()))
		nDialers = seed.Intn(64) + 64
		errc     = make(chan error, nDialers)
	)

	// Setup dialers.
	for i := 0; i < nDialers; i++ {
		go testDialer(*laddr, errc)
	}

	// Catch connection errors.
	for i := 0; i < nDialers; i++ {
		if err := <-errc; err != nil {
			t.Fatal(err)
		}
	}

	ps := []Peer{}

	// Accept all peers.
	for i := 0; i < cap(errc); i++ {
		p, err := mt.Accept(peerConfig{})
		if err != nil {
			t.Fatal(err)
		}

		if err := p.Start(); err != nil {
			t.Fatal(err)
		}

		ps = append(ps, p)
	}

	if have, want := len(ps), cap(errc); have != want {
		t.Errorf("have %v, want %v", have, want)
	}

	// Stop all peers.
	for _, p := range ps {
		if err := p.Stop(); err != nil {
			t.Fatal(err)
		}
	}

	if err := mt.Close(); err != nil {
		t.Errorf("close errored: %v", err)
	}
}

func testDialer(dialAddr NetAddress, errc chan error) {
	var (
		pv     = ed25519.GenPrivKey()
		dialer = newMultiplexTransport(
			testNodeInfo(PubKeyToID(pv.PubKey()), defaultNodeName),
			NodeKey{
				PrivKey: pv,
			},
		)
	)

	_, err := dialer.Dial(dialAddr, peerConfig{})
	if err != nil {
		errc <- err
		return
	}

	// Signal that the connection was established.
	errc <- nil
}

func TestTransportMultiplexAcceptNonBlocking(t *testing.T) {
	mt := testSetupMultiplexTransport(t)

	var (
		fastNodePV   = ed25519.GenPrivKey()
		fastNodeInfo = testNodeInfo(PubKeyToID(fastNodePV.PubKey()), "fastnode")
		errc         = make(chan error)
		fastc        = make(chan struct{})
		slowc        = make(chan struct{})
	)

	// Simulate slow Peer.
	go func() {
		addr := NewNetAddress(mt.nodeKey.ID(), mt.listener.Addr())

		c, err := addr.Dial()
		if err != nil {
			errc <- err
			return
		}

		close(slowc)

		select {
		case <-fastc:
			// Fast peer connected.
		case <-time.After(50 * time.Millisecond):
			// We error if the fast peer didn't succeed.
			errc <- fmt.Errorf("Fast peer timed out")
		}

		sc, err := upgradeSecretConn(c, 20*time.Millisecond, ed25519.GenPrivKey())
		if err != nil {
			errc <- err
			return
		}

		_, err = handshake(sc, 20*time.Millisecond,
			testNodeInfo(
				PubKeyToID(ed25519.GenPrivKey().PubKey()),
				"slow_peer",
			))
		if err != nil {
			errc <- err
			return
		}
	}()

	// Simulate fast Peer.
	go func() {
		<-slowc

		var (
			dialer = newMultiplexTransport(
				fastNodeInfo,
				NodeKey{
					PrivKey: fastNodePV,
				},
			)
		)
		addr := NewNetAddress(mt.nodeKey.ID(), mt.listener.Addr())

		_, err := dialer.Dial(*addr, peerConfig{})
		if err != nil {
			errc <- err
			return
		}

		close(errc)
		close(fastc)
	}()

	if err := <-errc; err != nil {
		t.Errorf("connection failed: %v", err)
	}

	p, err := mt.Accept(peerConfig{})
	if err != nil {
		t.Fatal(err)
	}

	if have, want := p.NodeInfo(), fastNodeInfo; !reflect.DeepEqual(have, want) {
		t.Errorf("have %v, want %v", have, want)
	}
}

func TestTransportMultiplexValidateNodeInfo(t *testing.T) {
	mt := testSetupMultiplexTransport(t)

	errc := make(chan error)

	go func() {
		var (
			pv     = ed25519.GenPrivKey()
			dialer = newMultiplexTransport(
				testNodeInfo(PubKeyToID(pv.PubKey()), ""), // Should not be empty
				NodeKey{
					PrivKey: pv,
				},
			)
		)

		addr := NewNetAddress(mt.nodeKey.ID(), mt.listener.Addr())

		_, err := dialer.Dial(*addr, peerConfig{})
		if err != nil {
			errc <- err
			return
		}

		close(errc)
	}()

	if err := <-errc; err != nil {
		t.Errorf("connection failed: %v", err)
	}

	_, err := mt.Accept(peerConfig{})
	if err, ok := err.(ErrRejected); ok {
		if !err.IsNodeInfoInvalid() {
			t.Errorf("expected NodeInfo to be invalid")
		}
	} else {
		t.Errorf("expected ErrRejected")
	}
}

func TestTransportMultiplexRejectMissmatchID(t *testing.T) {
	mt := testSetupMultiplexTransport(t)

	errc := make(chan error)

	go func() {
		dialer := newMultiplexTransport(
			testNodeInfo(
				PubKeyToID(ed25519.GenPrivKey().PubKey()), "dialer",
			),
			NodeKey{
				PrivKey: ed25519.GenPrivKey(),
			},
		)
		addr := NewNetAddress(mt.nodeKey.ID(), mt.listener.Addr())

		_, err := dialer.Dial(*addr, peerConfig{})
		if err != nil {
			errc <- err
			return
		}

		close(errc)
	}()

	if err := <-errc; err != nil {
		t.Errorf("connection failed: %v", err)
	}

	_, err := mt.Accept(peerConfig{})
	if err, ok := err.(ErrRejected); ok {
		if !err.IsAuthFailure() {
			t.Errorf("expected auth failure")
		}
	} else {
		t.Errorf("expected ErrRejected")
	}
}

func TestTransportMultiplexDialRejectWrongID(t *testing.T) {
	mt := testSetupMultiplexTransport(t)

	var (
		pv     = ed25519.GenPrivKey()
		dialer = newMultiplexTransport(
			testNodeInfo(PubKeyToID(pv.PubKey()), ""), // Should not be empty
			NodeKey{
				PrivKey: pv,
			},
		)
	)

	wrongID := PubKeyToID(ed25519.GenPrivKey().PubKey())
	addr := NewNetAddress(wrongID, mt.listener.Addr())

	_, err := dialer.Dial(*addr, peerConfig{})
	if err != nil {
		t.Logf("connection failed: %v", err)
		if err, ok := err.(ErrRejected); ok {
			if !err.IsAuthFailure() {
				t.Errorf("expected auth failure")
			}
		} else {
			t.Errorf("expected ErrRejected")
		}
	}
}

func TestTransportMultiplexRejectIncompatible(t *testing.T) {
	mt := testSetupMultiplexTransport(t)

	errc := make(chan error)

	go func() {
		var (
			pv     = ed25519.GenPrivKey()
			dialer = newMultiplexTransport(
				testNodeInfoWithNetwork(PubKeyToID(pv.PubKey()), "dialer", "incompatible-network"),
				NodeKey{
					PrivKey: pv,
				},
			)
		)
		addr := NewNetAddress(mt.nodeKey.ID(), mt.listener.Addr())

		_, err := dialer.Dial(*addr, peerConfig{})
		if err != nil {
			errc <- err
			return
		}

		close(errc)
	}()

	_, err := mt.Accept(peerConfig{})
	if err, ok := err.(ErrRejected); ok {
		if !err.IsIncompatible() {
			t.Errorf("expected to reject incompatible")
		}
	} else {
		t.Errorf("expected ErrRejected")
	}
}

func TestTransportMultiplexRejectSelf(t *testing.T) {
	mt := testSetupMultiplexTransport(t)

	errc := make(chan error)

	go func() {
		addr := NewNetAddress(mt.nodeKey.ID(), mt.listener.Addr())

		_, err := mt.Dial(*addr, peerConfig{})
		if err != nil {
			errc <- err
			return
		}

		close(errc)
	}()

	if err := <-errc; err != nil {
		if err, ok := err.(ErrRejected); ok {
			if !err.IsSelf() {
				t.Errorf("expected to reject self, got: %v", err)
			}
		} else {
			t.Errorf("expected ErrRejected")
		}
	} else {
		t.Errorf("expected connection failure")
	}

	_, err := mt.Accept(peerConfig{})
	if err, ok := err.(ErrRejected); ok {
		if !err.IsSelf() {
			t.Errorf("expected to reject self, got: %v", err)
		}
	} else {
		t.Errorf("expected ErrRejected")
	}
}

func TestTransportConnDuplicateIPFilter(t *testing.T) {
	filter := ConnDuplicateIPFilter()

	if err := filter(nil, &testTransportConn{}, nil); err != nil {
		t.Fatal(err)
	}

	var (
		c  = &testTransportConn{}
		cs = NewConnSet()
	)

	cs.Set(c, []net.IP{
		{10, 0, 10, 1},
		{10, 0, 10, 2},
		{10, 0, 10, 3},
	})

	if err := filter(cs, c, []net.IP{
		{10, 0, 10, 2},
	}); err == nil {
		t.Errorf("expected Peer to be rejected as duplicate")
	}
}

func TestTransportHandshake(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	var (
		peerPV       = ed25519.GenPrivKey()
		peerNodeInfo = testNodeInfo(PubKeyToID(peerPV.PubKey()), defaultNodeName)
	)

	go func() {
		c, err := net.Dial(ln.Addr().Network(), ln.Addr().String())
		if err != nil {
			t.Error(err)
			return
		}

		go func(c net.Conn) {
			_, err := cdc.MarshalBinaryLengthPrefixedWriter(c, peerNodeInfo.(DefaultNodeInfo))
			if err != nil {
				t.Error(err)
			}
		}(c)
		go func(c net.Conn) {
			var ni DefaultNodeInfo

			_, err := cdc.UnmarshalBinaryLengthPrefixedReader(
				c,
				&ni,
				int64(MaxNodeInfoSize()),
			)
			if err != nil {
				t.Error(err)
			}
		}(c)
	}()

	c, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}

	ni, err := handshake(c, 20*time.Millisecond, emptyNodeInfo())
	if err != nil {
		t.Fatal(err)
	}

	if have, want := ni, peerNodeInfo; !reflect.DeepEqual(have, want) {
		t.Errorf("have %v, want %v", have, want)
	}
}

// create listener
func testSetupMultiplexTransport(t *testing.T) *MultiplexTransport {
	var (
		pv = ed25519.GenPrivKey()
		id = PubKeyToID(pv.PubKey())
		mt = newMultiplexTransport(
			testNodeInfo(
				id, "transport",
			),
			NodeKey{
				PrivKey: pv,
			},
		)
	)

	addr, err := NewNetAddressString(IDAddressString(id, "127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}

	if err := mt.Listen(*addr); err != nil {
		t.Fatal(err)
	}

	return mt
}

type testTransportAddr struct{}

func (a *testTransportAddr) Network() string { return "tcp" }
func (a *testTransportAddr) String() string  { return "test.local:1234" }

type testTransportConn struct{}

func (c *testTransportConn) Close() error {
	return fmt.Errorf("Close() not implemented")
}

func (c *testTransportConn) LocalAddr() net.Addr {
	return &testTransportAddr{}
}

func (c *testTransportConn) RemoteAddr() net.Addr {
	return &testTransportAddr{}
}

func (c *testTransportConn) Read(_ []byte) (int, error) {
	return -1, fmt.Errorf("Read() not implemented")
}

func (c *testTransportConn) SetDeadline(_ time.Time) error {
	return fmt.Errorf("SetDeadline() not implemented")
}

func (c *testTransportConn) SetReadDeadline(_ time.Time) error {
	return fmt.Errorf("SetReadDeadline() not implemented")
}

func (c *testTransportConn) SetWriteDeadline(_ time.Time) error {
	return fmt.Errorf("SetWriteDeadline() not implemented")
}

func (c *testTransportConn) Write(_ []byte) (int, error) {
	return -1, fmt.Errorf("Write() not implemented")
}
