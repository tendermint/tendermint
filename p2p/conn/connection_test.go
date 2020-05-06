package conn

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	amino "github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/libs/log"
	tmp2p "github.com/tendermint/tendermint/proto/p2p"
)

const maxPingPongPacketSize = 1024 // bytes

func createTestMConnection(conn net.Conn) *MConnection {
	onReceive := func(chID byte, msgBytes []byte) {
	}
	onError := func(r interface{}) {
	}
	c := createMConnectionWithCallbacks(conn, onReceive, onError)
	c.SetLogger(log.TestingLogger())
	return c
}

func createMConnectionWithCallbacks(
	conn net.Conn,
	onReceive func(chID byte, msgBytes []byte),
	onError func(r interface{}),
) *MConnection {
	cfg := DefaultMConnConfig()
	cfg.PingInterval = 90 * time.Millisecond
	cfg.PongTimeout = 45 * time.Millisecond
	chDescs := []*ChannelDescriptor{{ID: 0x01, Priority: 1, SendQueueCapacity: 1}}
	c := NewMConnectionWithConfig(conn, chDescs, onReceive, onError, cfg)
	c.SetLogger(log.TestingLogger())
	return c
}

func TestMConnectionSendFlushStop(t *testing.T) {
	server, client := NetPipe()
	defer server.Close() // nolint: errcheck
	defer client.Close() // nolint: errcheck

	clientConn := createTestMConnection(client)
	err := clientConn.Start()
	require.Nil(t, err)
	defer clientConn.Stop()

	msg := []byte("abc")
	assert.True(t, clientConn.Send(0x01, msg))

	aminoMsgLength := 14

	// start the reader in a new routine, so we can flush
	errCh := make(chan error)
	go func() {
		msgB := make([]byte, aminoMsgLength)
		_, err := server.Read(msgB)
		if err != nil {
			t.Error(err)
			return
		}
		errCh <- err
	}()

	// stop the conn - it should flush all conns
	clientConn.FlushStop()

	timer := time.NewTimer(3 * time.Second)
	select {
	case <-errCh:
	case <-timer.C:
		t.Error("timed out waiting for msgs to be read")
	}
}

func TestMConnectionSend(t *testing.T) {
	server, client := NetPipe()
	defer server.Close() // nolint: errcheck
	defer client.Close() // nolint: errcheck

	mconn := createTestMConnection(client)
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	msg := []byte("Ant-Man")
	assert.True(t, mconn.Send(0x01, msg))
	// Note: subsequent Send/TrySend calls could pass because we are reading from
	// the send queue in a separate goroutine.
	_, err = server.Read(make([]byte, len(msg)))
	if err != nil {
		t.Error(err)
	}
	assert.True(t, mconn.CanSend(0x01))

	msg = []byte("Spider-Man")
	assert.True(t, mconn.TrySend(0x01, msg))
	_, err = server.Read(make([]byte, len(msg)))
	if err != nil {
		t.Error(err)
	}

	assert.False(t, mconn.CanSend(0x05), "CanSend should return false because channel is unknown")
	assert.False(t, mconn.Send(0x05, []byte("Absorbing Man")), "Send should return false because channel is unknown")
}

func TestMConnectionReceive(t *testing.T) {
	server, client := NetPipe()
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
	require.Nil(t, err)
	defer mconn1.Stop()

	mconn2 := createTestMConnection(server)
	err = mconn2.Start()
	require.Nil(t, err)
	defer mconn2.Stop()

	msg := []byte("Cyclops")
	assert.True(t, mconn2.Send(0x01, msg))

	select {
	case receivedBytes := <-receivedCh:
		assert.Equal(t, msg, receivedBytes)
	case err := <-errorsCh:
		t.Fatalf("Expected %s, got %+v", msg, err)
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Did not receive %s message in 500ms", msg)
	}
}

func TestMConnectionStatus(t *testing.T) {
	server, client := NetPipe()
	defer server.Close() // nolint: errcheck
	defer client.Close() // nolint: errcheck

	mconn := createTestMConnection(client)
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	status := mconn.Status()
	assert.NotNil(t, status)
	assert.Zero(t, status.Channels[0].SendQueueSize)
}

func TestMConnectionPongTimeoutResultsInError(t *testing.T) {
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
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	serverGotPing := make(chan struct{})
	go func() {
		// read ping
		var pkt tmp2p.PacketPing

		bz, err := readAll(server, maxPingPongPacketSize)
		require.NoError(t, err)
		proto.Unmarshal(bz, &pkt)
		assert.Nil(t, err)
		serverGotPing <- struct{}{}
	}()
	<-serverGotPing

	pongTimerExpired := mconn.config.PongTimeout + 20*time.Millisecond
	select {
	case msgBytes := <-receivedCh:
		t.Fatalf("Expected error, but got %v", msgBytes)
	case err := <-errorsCh:
		assert.NotNil(t, err)
	case <-time.After(pongTimerExpired):
		t.Fatalf("Expected to receive error after %v", pongTimerExpired)
	}
}

func TestMConnectionMultiplePongsInTheBeginning(t *testing.T) {
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
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	// sending 3 pongs in a row (abuse)
	bz, err := encodeMsg(&tmp2p.PacketPong{})
	require.NoError(t, err)
	_, err = server.Write(bz)
	require.Nil(t, err)
	bz2, err := encodeMsg(&tmp2p.PacketPong{})
	require.NoError(t, err)
	_, err = server.Write(bz2)
	require.Nil(t, err)
	bz3, err := encodeMsg(&tmp2p.PacketPong{})
	require.NoError(t, err)
	_, err = server.Write(bz3)
	require.Nil(t, err)

	serverGotPing := make(chan struct{})
	go func() {
		// read ping (one byte)
		var (
			packet tmp2p.Packet
			err    error
		)

		rl := io.LimitReader(server, maxPingPongPacketSize)
		pz, err := ioutil.ReadAll(rl)
		require.NoError(t, err)

		err = proto.Unmarshal(pz, &packet)
		require.NoError(t, err)

		serverGotPing <- struct{}{}
		// respond with pong
		bz, err = encodeMsg(&tmp2p.PacketPong{})
		require.NoError(t, err)
		_, err = server.Write(bz)
		require.Nil(t, err)
	}()
	<-serverGotPing

	pongTimerExpired := mconn.config.PongTimeout + 20*time.Millisecond
	select {
	case msgBytes := <-receivedCh:
		t.Fatalf("Expected no data, but got %v", msgBytes)
	case err := <-errorsCh:
		t.Fatalf("Expected no error, but got %v", err)
	case <-time.After(pongTimerExpired):
		assert.True(t, mconn.IsRunning())
	}
}

func TestMConnectionMultiplePings(t *testing.T) {
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
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	// sending 3 pings in a row (abuse)
	// see https://github.com/tendermint/tendermint/issues/1190
	bz, err := encodeMsg(&tmp2p.PacketPing{})
	require.NoError(t, err)
	_, err = server.Write(bz)
	require.NoError(t, err)

	var pkt tmp2p.PacketPong

	bz, err = readAll(server, maxPingPongPacketSize)
	require.NoError(t, err)
	err = proto.Unmarshal(bz, &pkt)
	require.NoError(t, err)

	bz, err = encodeMsg(&tmp2p.PacketPing{})
	require.NoError(t, err)
	_, err = server.Write(bz)
	require.NoError(t, err)

	bz, err = readAll(server, maxPingPongPacketSize)
	require.NoError(t, err)
	err = proto.Unmarshal(bz, &pkt)
	require.NoError(t, err)

	bz, err = encodeMsg(&tmp2p.PacketPing{})
	require.NoError(t, err)
	_, err = server.Write(bz)
	require.NoError(t, err)

	bz, err = readAll(server, maxPingPongPacketSize)
	require.NoError(t, err)
	err = proto.Unmarshal(bz, &pkt)
	require.NoError(t, err)

	assert.True(t, mconn.IsRunning())
}

func TestMConnectionPingPongs(t *testing.T) {
	// check that we are not leaking any go-routines
	defer leaktest.CheckTimeout(t, 10*time.Second)()

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
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	serverGotPing := make(chan struct{})
	go func() {
		// read ping
		var pkt tmp2p.PacketPing
		bz, err := readAll(server, maxPingPongPacketSize)
		require.NoError(t, err)
		err = proto.Unmarshal(bz, &pkt)
		require.NoError(t, err)

		serverGotPing <- struct{}{}
		// respond with pong
		bz2, err := encodeMsg(&tmp2p.PacketPong{})
		require.NoError(t, err)
		_, err = server.Write(bz2)
		require.NoError(t, err)

		time.Sleep(mconn.config.PingInterval)

		// read ping
		bz, err = readAll(server, maxPingPongPacketSize)
		require.NoError(t, err)
		err = proto.Unmarshal(bz, &pkt)
		require.NoError(t, err)

		// respond with pong
		bz2, err = encodeMsg(&tmp2p.PacketPong{})
		require.NoError(t, err)
		_, err = server.Write(bz2)
		require.NoError(t, err)
	}()
	<-serverGotPing

	pongTimerExpired := (mconn.config.PongTimeout + 20*time.Millisecond) * 2
	select {
	case msgBytes := <-receivedCh:
		t.Fatalf("Expected no data, but got %v", msgBytes)
	case err := <-errorsCh:
		t.Fatalf("Expected no error, but got %v", err)
	case <-time.After(2 * pongTimerExpired):
		assert.True(t, mconn.IsRunning())
	}
}

func TestMConnectionStopsAndReturnsError(t *testing.T) {
	server, client := NetPipe()
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
	require.Nil(t, err)
	defer mconn.Stop()

	if err := client.Close(); err != nil {
		t.Error(err)
	}

	select {
	case receivedBytes := <-receivedCh:
		t.Fatalf("Expected error, got %v", receivedBytes)
	case err := <-errorsCh:
		assert.NotNil(t, err)
		assert.False(t, mconn.IsRunning())
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Did not receive error in 500ms")
	}
}

func newClientAndServerConnsForReadErrors(t *testing.T, chOnErr chan struct{}) (*MConnection, *MConnection) {
	server, client := NetPipe()

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
	require.Nil(t, err)

	// create server conn with 1 channel
	// it fires on chOnErr when there's an error
	serverLogger := log.TestingLogger().With("module", "server")
	onError = func(r interface{}) {
		chOnErr <- struct{}{}
	}
	mconnServer := createMConnectionWithCallbacks(server, onReceive, onError)
	mconnServer.SetLogger(serverLogger)
	err = mconnServer.Start()
	require.Nil(t, err)
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
	chOnErr := make(chan struct{})
	mconnClient, mconnServer := newClientAndServerConnsForReadErrors(t, chOnErr)
	defer mconnClient.Stop()
	defer mconnServer.Stop()

	client := mconnClient.conn

	// send badly encoded msgPacket
	bz, err := encodeMsg(&tmp2p.PacketMsg{})
	require.NoError(t, err)
	bz[1] += 0x01 // change the bytes

	// Write it.
	_, err = client.Write(bz)
	assert.Nil(t, err)
	assert.True(t, expectSend(chOnErr), "badly encoded msgPacket")
}

func TestMConnectionReadErrorUnknownChannel(t *testing.T) {
	chOnErr := make(chan struct{})
	mconnClient, mconnServer := newClientAndServerConnsForReadErrors(t, chOnErr)
	defer mconnClient.Stop()
	defer mconnServer.Stop()

	msg := []byte("Ant-Man")

	// fail to send msg on channel unknown by client
	assert.False(t, mconnClient.Send(0x03, msg))

	// send msg on channel unknown by the server.
	// should cause an error
	assert.True(t, mconnClient.Send(0x02, msg))
	assert.True(t, expectSend(chOnErr), "unknown channel")
}

func TestMConnectionReadErrorLongMessage(t *testing.T) {
	chOnErr := make(chan struct{})
	chOnRcv := make(chan struct{})

	mconnClient, mconnServer := newClientAndServerConnsForReadErrors(t, chOnErr)
	defer mconnClient.Stop()
	defer mconnServer.Stop()

	mconnServer.onReceive = func(chID byte, msgBytes []byte) {
		chOnRcv <- struct{}{}
	}

	client := mconnClient.conn

	// send msg thats just right
	var err error
	var buf = new(bytes.Buffer)
	var packet = tmp2p.PacketMsg{
		ChannelID: 0x01,
		EOF:       1,
		Data:      make([]byte, mconnClient.config.MaxPacketMsgPayloadSize),
	}
	bz, err := encodeMsg(&packet)
	require.NoError(t, err)

	_, err = buf.Write(bz)
	require.NoError(t, err)

	_, err = client.Write(buf.Bytes())
	require.NoError(t, err)

	assert.True(t, expectSend(chOnRcv), "msg just right")

	// send msg thats too long
	buf = new(bytes.Buffer)
	packet = tmp2p.PacketMsg{
		ChannelID: 0x01,
		EOF:       1,
		Data:      make([]byte, mconnClient.config.MaxPacketMsgPayloadSize+100),
	}
	bz, err = encodeMsg(&packet)
	require.NoError(t, err)

	_, err = buf.Write(bz)
	require.NoError(t, err)

	_, err = client.Write(buf.Bytes())
	require.NoError(t, err)

	assert.True(t, expectSend(chOnErr), "msg too long")
}

func TestMConnectionReadErrorUnknownMsgType(t *testing.T) {
	chOnErr := make(chan struct{})
	mconnClient, mconnServer := newClientAndServerConnsForReadErrors(t, chOnErr)
	defer mconnClient.Stop()
	defer mconnServer.Stop()

	// send msg with unknown msg type
	err := amino.EncodeUvarint(mconnClient.conn, 4)
	assert.Nil(t, err)
	_, err = mconnClient.conn.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	assert.Nil(t, err)
	assert.True(t, expectSend(chOnErr), "unknown msg type")
}

func TestMConnectionTrySend(t *testing.T) {
	server, client := NetPipe()
	defer server.Close()
	defer client.Close()

	mconn := createTestMConnection(client)
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	msg := []byte("Semicolon-Woman")
	resultCh := make(chan string, 2)
	assert.True(t, mconn.TrySend(0x01, msg))
	server.Read(make([]byte, len(msg)))
	assert.True(t, mconn.CanSend(0x01))
	assert.True(t, mconn.TrySend(0x01, msg))
	assert.False(t, mconn.CanSend(0x01))
	go func() {
		mconn.TrySend(0x01, msg)
		resultCh <- "TrySend"
	}()
	assert.False(t, mconn.CanSend(0x01))
	assert.False(t, mconn.TrySend(0x01, msg))
	assert.Equal(t, "TrySend", <-resultCh)
}
