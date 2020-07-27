package p2p

import (
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/protoio"
	"github.com/tendermint/tendermint/p2p/conn"
	tmp2p "github.com/tendermint/tendermint/proto/tendermint/p2p"
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
			t.Errorf("expected peer to be filtered, got %v", err)
		}
	} else {
		t.Errorf("expected ErrRejected, got %v", err)
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
			time.Sleep(1 * time.Second)
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
		t.Errorf("expected ErrFilterTimeout, got %v", err)
	}
}

func TestTransportMultiplexMaxIncomingConnections(t *testing.T) {
	pv := ed25519.GenPrivKey()
	id := PubKeyToID(pv.PubKey())
	mt := newMultiplexTransport(
		testNodeInfo(
			id, "transport",
		),
		NodeKey{
			PrivKey: pv,
		},
	)

	MultiplexTransportMaxIncomingConnections(0)(mt)

	addr, err := NewNetAddressString(IDAddressString(id, "127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}
	const maxIncomingConns = 2
	MultiplexTransportMaxIncomingConnections(maxIncomingConns)(mt)
	if err := mt.Listen(*addr); err != nil {
		t.Fatal(err)
	}

	laddr := NewNetAddress(mt.nodeKey.ID(), mt.listener.Addr())

	// Connect more peers than max
	for i := 0; i <= maxIncomingConns; i++ {
		errc := make(chan error)
		go testDialer(*laddr, errc)

		err = <-errc
		if i < maxIncomingConns {
			if err != nil {
				t.Errorf("dialer connection failed: %v", err)
			}
			_, err = mt.Accept(peerConfig{})
			if err != nil {
				t.Errorf("connection failed: %v", err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "i/o timeout") {
			// mt actually blocks forever on trying to accept a new peer into a full channel so
			// expect the dialer to encounter a timeout error. Calling mt.Accept will block until
			// mt is closed.
			t.Errorf("expected i/o timeout error, got %v", err)
		}
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
		slowdonec    = make(chan struct{})
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
		defer func() {
			close(slowdonec)
		}()

		// Make sure we switch to fast peer goroutine.
		runtime.Gosched()

		select {
		case <-fastc:
			// Fast peer connected.
		case <-time.After(200 * time.Millisecond):
			// We error if the fast peer didn't succeed.
			errc <- fmt.Errorf("fast peer timed out")
		}

		sc, err := upgradeSecretConn(c, 200*time.Millisecond, ed25519.GenPrivKey())
		if err != nil {
			errc <- err
			return
		}

		_, err = handshake(sc, 200*time.Millisecond,
			testNodeInfo(
				PubKeyToID(ed25519.GenPrivKey().PubKey()),
				"slow_peer",
			))
		if err != nil {
			errc <- err
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

		close(fastc)
		<-slowdonec
		close(errc)
	}()

	if err := <-errc; err != nil {
		t.Logf("connection failed: %v", err)
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
			t.Errorf("expected NodeInfo to be invalid, got %v", err)
		}
	} else {
		t.Errorf("expected ErrRejected, got %v", err)
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
			t.Errorf("expected auth failure, got %v", err)
		}
	} else {
		t.Errorf("expected ErrRejected, got %v", err)
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
				t.Errorf("expected auth failure, got %v", err)
			}
		} else {
			t.Errorf("expected ErrRejected, got %v", err)
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
			t.Errorf("expected to reject incompatible, got %v", err)
		}
	} else {
		t.Errorf("expected ErrRejected, got %v", err)
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
			t.Errorf("expected ErrRejected, got %v", err)
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
		t.Errorf("expected ErrRejected, got %v", nil)
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
			_, err := protoio.NewDelimitedWriter(c).WriteMsg(peerNodeInfo.(DefaultNodeInfo).ToProto())
			if err != nil {
				t.Error(err)
			}
		}(c)
		go func(c net.Conn) {
			var (
				// ni   DefaultNodeInfo
				pbni tmp2p.DefaultNodeInfo
			)

			protoReader := protoio.NewDelimitedReader(c, MaxNodeInfoSize())
			err := protoReader.ReadMsg(&pbni)
			if err != nil {
				t.Error(err)
			}

			_, err = DefaultNodeInfoFromToProto(&pbni)
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

	// give the listener some time to get ready
	time.Sleep(20 * time.Millisecond)

	return mt
}

type testTransportAddr struct{}

func (a *testTransportAddr) Network() string { return "tcp" }
func (a *testTransportAddr) String() string  { return "test.local:1234" }

type testTransportConn struct{}

func (c *testTransportConn) Close() error {
	return fmt.Errorf("close() not implemented")
}

func (c *testTransportConn) LocalAddr() net.Addr {
	return &testTransportAddr{}
}

func (c *testTransportConn) RemoteAddr() net.Addr {
	return &testTransportAddr{}
}

func (c *testTransportConn) Read(_ []byte) (int, error) {
	return -1, fmt.Errorf("read() not implemented")
}

func (c *testTransportConn) SetDeadline(_ time.Time) error {
	return fmt.Errorf("setDeadline() not implemented")
}

func (c *testTransportConn) SetReadDeadline(_ time.Time) error {
	return fmt.Errorf("setReadDeadline() not implemented")
}

func (c *testTransportConn) SetWriteDeadline(_ time.Time) error {
	return fmt.Errorf("setWriteDeadline() not implemented")
}

func (c *testTransportConn) Write(_ []byte) (int, error) {
	return -1, fmt.Errorf("write() not implemented")
}
