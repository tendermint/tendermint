package p2p

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/tmlibs/log"
)

func createTestMConnection(conn net.Conn) *MConnection {
	onReceive := func(chID byte, msgBytes []byte) {
	}
	onError := func(r interface{}) {
	}
	c := createMConnectionWithCallbacks(conn, onReceive, onError)
	c.SetLogger(log.TestingLogger())
	return c
}

func createMConnectionWithCallbacks(conn net.Conn, onReceive func(chID byte, msgBytes []byte), onError func(r interface{})) *MConnection {
	chDescs := []*ChannelDescriptor{&ChannelDescriptor{ID: 0x01, Priority: 1, SendQueueCapacity: 1}}
	c := NewMConnection(conn, chDescs, onReceive, onError)
	c.SetLogger(log.TestingLogger())
	return c
}

func TestMConnectionSend(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	server, client := netPipe()
	defer server.Close() // nolint: errcheck
	defer client.Close() // nolint: errcheck

	mconn := createTestMConnection(client)
	err := mconn.Start()
	require.Nil(err)
	defer mconn.Stop()

	msg := "Ant-Man"
	assert.True(mconn.Send(0x01, msg))
	// Note: subsequent Send/TrySend calls could pass because we are reading from
	// the send queue in a separate goroutine.
	_, err = server.Read(make([]byte, len(msg)))
	if err != nil {
		t.Error(err)
	}
	assert.True(mconn.CanSend(0x01))

	msg = "Spider-Man"
	assert.True(mconn.TrySend(0x01, msg))
	_, err = server.Read(make([]byte, len(msg)))
	if err != nil {
		t.Error(err)
	}

	assert.False(mconn.CanSend(0x05), "CanSend should return false because channel is unknown")
	assert.False(mconn.Send(0x05, "Absorbing Man"), "Send should return false because channel is unknown")
}

func TestMConnectionReceive(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	server, client := netPipe()
	defer server.Close() // nolint: errcheck
	defer client.Close() // nolint: errcheck

	receivedCh := make(chan []byte)
	errorsCh := make(chan interface{})
	onReceive := func(chID byte, msgBytes []byte) {
		receivedCh <- msgBytes
	}
	onError := func(r interface{}) {
		errorsCh <- r
	}
	mconn1 := createMConnectionWithCallbacks(client, onReceive, onError)
	err := mconn1.Start()
	require.Nil(err)
	defer mconn1.Stop()

	mconn2 := createTestMConnection(server)
	err = mconn2.Start()
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

	server, client := netPipe()
	defer server.Close() // nolint: errcheck
	defer client.Close() // nolint: errcheck

	mconn := createTestMConnection(client)
	err := mconn.Start()
	require.Nil(err)
	defer mconn.Stop()

	status := mconn.Status()
	assert.NotNil(status)
	assert.Zero(status.Channels[0].SendQueueSize)
}

func TestMConnectionStopsAndReturnsError(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	server, client := netPipe()
	defer server.Close() // nolint: errcheck
	defer client.Close() // nolint: errcheck

	receivedCh := make(chan []byte)
	errorsCh := make(chan interface{})
	onReceive := func(chID byte, msgBytes []byte) {
		receivedCh <- msgBytes
	}
	onError := func(r interface{}) {
		errorsCh <- r
	}
	mconn := createMConnectionWithCallbacks(client, onReceive, onError)
	err := mconn.Start()
	require.Nil(err)
	defer mconn.Stop()

	if err := client.Close(); err != nil {
		t.Error(err)
	}

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

func newClientAndServerConnsForReadErrors(require *require.Assertions, chOnErr chan struct{}) (*MConnection, *MConnection) {
	server, client := netPipe()

	onReceive := func(chID byte, msgBytes []byte) {}
	onError := func(r interface{}) {}

	// create client conn with two channels
	chDescs := []*ChannelDescriptor{
		{ID: 0x01, Priority: 1, SendQueueCapacity: 1},
		{ID: 0x02, Priority: 1, SendQueueCapacity: 1},
	}
	mconnClient := NewMConnection(client, chDescs, onReceive, onError)
	mconnClient.SetLogger(log.TestingLogger().With("module", "client"))
	err := mconnClient.Start()
	require.Nil(err)

	// create server conn with 1 channel
	// it fires on chOnErr when there's an error
	serverLogger := log.TestingLogger().With("module", "server")
	onError = func(r interface{}) {
		chOnErr <- struct{}{}
	}
	mconnServer := createMConnectionWithCallbacks(server, onReceive, onError)
	mconnServer.SetLogger(serverLogger)
	err = mconnServer.Start()
	require.Nil(err)
	return mconnClient, mconnServer
}

func expectSend(ch chan struct{}) bool {
	after := time.After(time.Second * 5)
	select {
	case <-ch:
		return true
	case <-after:
		return false
	}
}

func TestMConnectionReadErrorBadEncoding(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	chOnErr := make(chan struct{})
	mconnClient, mconnServer := newClientAndServerConnsForReadErrors(require, chOnErr)
	defer mconnClient.Stop()
	defer mconnServer.Stop()

	client := mconnClient.conn
	msg := "Ant-Man"

	// send badly encoded msgPacket
	var n int
	var err error
	wire.WriteByte(packetTypeMsg, client, &n, &err)
	wire.WriteByteSlice([]byte(msg), client, &n, &err)
	assert.True(expectSend(chOnErr), "badly encoded msgPacket")
}

func TestMConnectionReadErrorUnknownChannel(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	chOnErr := make(chan struct{})
	mconnClient, mconnServer := newClientAndServerConnsForReadErrors(require, chOnErr)
	defer mconnClient.Stop()
	defer mconnServer.Stop()

	msg := "Ant-Man"

	// fail to send msg on channel unknown by client
	assert.False(mconnClient.Send(0x03, msg))

	// send msg on channel unknown by the server.
	// should cause an error
	assert.True(mconnClient.Send(0x02, msg))
	assert.True(expectSend(chOnErr), "unknown channel")
}

func TestMConnectionReadErrorLongMessage(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	chOnErr := make(chan struct{})
	chOnRcv := make(chan struct{})

	mconnClient, mconnServer := newClientAndServerConnsForReadErrors(require, chOnErr)
	defer mconnClient.Stop()
	defer mconnServer.Stop()

	mconnServer.onReceive = func(chID byte, msgBytes []byte) {
		chOnRcv <- struct{}{}
	}

	client := mconnClient.conn

	// send msg thats just right
	var n int
	var err error
	packet := msgPacket{
		ChannelID: 0x01,
		Bytes:     make([]byte, mconnClient.config.maxMsgPacketTotalSize()-5),
		EOF:       1,
	}
	writeMsgPacketTo(packet, client, &n, &err)
	assert.True(expectSend(chOnRcv), "msg just right")

	// send msg thats too long
	packet = msgPacket{
		ChannelID: 0x01,
		Bytes:     make([]byte, mconnClient.config.maxMsgPacketTotalSize()-4),
		EOF:       1,
	}
	writeMsgPacketTo(packet, client, &n, &err)
	assert.True(expectSend(chOnErr), "msg too long")
}

func TestMConnectionReadErrorUnknownMsgType(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	chOnErr := make(chan struct{})
	mconnClient, mconnServer := newClientAndServerConnsForReadErrors(require, chOnErr)
	defer mconnClient.Stop()
	defer mconnServer.Stop()

	// send msg with unknown msg type
	var n int
	var err error
	wire.WriteByte(0x04, mconnClient.conn, &n, &err)
	assert.True(expectSend(chOnErr), "unknown msg type")
}

func TestMConnectionTrySend(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	server, client := netPipe()
	defer server.Close()
	defer client.Close()

	mconn := createTestMConnection(client)
	err := mconn.Start()
	require.Nil(err)
	defer mconn.Stop()

	msg := "Semicolon-Woman"
	resultCh := make(chan string, 2)
	assert.True(mconn.TrySend(0x01, msg))
	server.Read(make([]byte, len(msg)))
	assert.True(mconn.CanSend(0x01))
	assert.True(mconn.TrySend(0x01, msg))
	assert.False(mconn.CanSend(0x01))
	go func() {
		mconn.TrySend(0x01, msg)
		resultCh <- "TrySend"
	}()
	go func() {
		mconn.Send(0x01, msg)
		resultCh <- "Send"
	}()
	assert.False(mconn.CanSend(0x01))
	assert.False(mconn.TrySend(0x01, msg))
	assert.Equal("TrySend", <-resultCh)
	server.Read(make([]byte, len(msg)))
	assert.Equal("Send", <-resultCh) // Order constrained by parallel blocking above
}
