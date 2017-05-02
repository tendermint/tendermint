package p2p_test

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	p2p "github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tmlibs/log"
)

func createMConnection(conn net.Conn) *p2p.MConnection {
	onReceive := func(chID byte, msgBytes []byte) {
	}
	onError := func(r interface{}) {
	}
	c := createMConnectionWithCallbacks(conn, onReceive, onError)
	c.SetLogger(log.TestingLogger())
	return c
}

func createMConnectionWithCallbacks(conn net.Conn, onReceive func(chID byte, msgBytes []byte), onError func(r interface{})) *p2p.MConnection {
	chDescs := []*p2p.ChannelDescriptor{&p2p.ChannelDescriptor{ID: 0x01, Priority: 1, SendQueueCapacity: 1}}
	c := p2p.NewMConnection(conn, chDescs, onReceive, onError)
	c.SetLogger(log.TestingLogger())
	return c
}

func TestMConnectionSend(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	mconn := createMConnection(client)
	_, err := mconn.Start()
	require.Nil(err)
	defer mconn.Stop()

	msg := "Ant-Man"
	assert.True(mconn.Send(0x01, msg))
	// Note: subsequent Send/TrySend calls could pass because we are reading from
	// the send queue in a separate goroutine.
	server.Read(make([]byte, len(msg)))
	assert.True(mconn.CanSend(0x01))

	msg = "Spider-Man"
	assert.True(mconn.TrySend(0x01, msg))
	server.Read(make([]byte, len(msg)))

	assert.False(mconn.CanSend(0x05), "CanSend should return false because channel is unknown")
	assert.False(mconn.Send(0x05, "Absorbing Man"), "Send should return false because channel is unknown")
}

func TestMConnectionReceive(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	receivedCh := make(chan []byte)
	errorsCh := make(chan interface{})
	onReceive := func(chID byte, msgBytes []byte) {
		receivedCh <- msgBytes
	}
	onError := func(r interface{}) {
		errorsCh <- r
	}
	mconn1 := createMConnectionWithCallbacks(client, onReceive, onError)
	_, err := mconn1.Start()
	require.Nil(err)
	defer mconn1.Stop()

	mconn2 := createMConnection(server)
	_, err = mconn2.Start()
	require.Nil(err)
	defer mconn2.Stop()

	msg := "Cyclops"
	assert.True(mconn2.Send(0x01, msg))

	select {
	case receivedBytes := <-receivedCh:
		assert.Equal([]byte(msg), receivedBytes[2:]) // first 3 bytes are internal
	case err := <-errorsCh:
		t.Fatalf("Expected %s, got %+v", msg, err)
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Did not receive %s message in 500ms", msg)
	}
}

func TestMConnectionStatus(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	mconn := createMConnection(client)
	_, err := mconn.Start()
	require.Nil(err)
	defer mconn.Stop()

	status := mconn.Status()
	assert.NotNil(status)
	assert.Zero(status.Channels[0].SendQueueSize)
}

func TestMConnectionStopsAndReturnsError(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	receivedCh := make(chan []byte)
	errorsCh := make(chan interface{})
	onReceive := func(chID byte, msgBytes []byte) {
		receivedCh <- msgBytes
	}
	onError := func(r interface{}) {
		errorsCh <- r
	}
	mconn := createMConnectionWithCallbacks(client, onReceive, onError)
	_, err := mconn.Start()
	require.Nil(err)
	defer mconn.Stop()

	client.Close()

	select {
	case receivedBytes := <-receivedCh:
		t.Fatalf("Expected error, got %v", receivedBytes)
	case err := <-errorsCh:
		assert.NotNil(err)
		assert.False(mconn.IsRunning())
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Did not receive error in 500ms")
	}
}
