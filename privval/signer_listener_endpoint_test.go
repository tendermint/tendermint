package privval

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/types"
)

var (
	testTimeoutAccept = defaultTimeoutAcceptSeconds * time.Second

	testTimeoutReadWrite    = 100 * time.Millisecond
	testTimeoutReadWrite2o3 = 60 * time.Millisecond // 2/3 of the other one
)

type dialerTestCase struct {
	addr   string
	dialer SocketDialer
}

// TestSignerRemoteRetryTCPOnly will test connection retry attempts over TCP. We
// don't need this for Unix sockets because the OS instantly knows the state of
// both ends of the socket connection. This basically causes the
// SignerDialerEndpoint.dialer() call inside SignerDialerEndpoint.acceptNewConnection() to return
// successfully immediately, putting an instant stop to any retry attempts.
func TestSignerRemoteRetryTCPOnly(t *testing.T) {
	var (
		attemptCh = make(chan int)
		retries   = 10
	)

	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	// Continuously Accept connection and close {attempts} times
	go func(ln net.Listener, attemptCh chan<- int) {
		attempts := 0
		for {
			conn, err := ln.Accept()
			require.NoError(t, err)

			err = conn.Close()
			require.NoError(t, err)

			attempts++

			if attempts == retries {
				attemptCh <- attempts
				break
			}
		}
	}(ln, attemptCh)

	dialerEndpoint := NewSignerDialerEndpoint(logger,
		DialTCPFn(ln.Addr().String(), testTimeoutReadWrite, ed25519.GenPrivKey()),
	)
	SignerDialerEndpointTimeoutReadWrite(time.Millisecond)(dialerEndpoint)
	SignerDialerEndpointConnRetries(retries)(dialerEndpoint)

	chainID := tmrand.Str(12)
	mockPV := types.NewMockPV()
	signerServer := NewSignerServer(dialerEndpoint, chainID, mockPV)

	err = signerServer.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(signerServer.Wait)

	select {
	case attempts := <-attemptCh:
		assert.Equal(t, retries, attempts)
	case <-time.After(1500 * time.Millisecond):
		t.Error("expected remote to observe connection attempts")
	}
}

func TestRetryConnToRemoteSigner(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	for _, tc := range getDialerTestCases(t) {
		var (
			chainID          = tmrand.Str(12)
			mockPV           = types.NewMockPV()
			endpointIsOpenCh = make(chan struct{})
			thisConnTimeout  = testTimeoutReadWrite
			listenerEndpoint = newSignerListenerEndpoint(t, logger, tc.addr, thisConnTimeout)
		)
		t.Cleanup(listenerEndpoint.Wait)

		dialerEndpoint := NewSignerDialerEndpoint(
			logger,
			tc.dialer,
		)
		SignerDialerEndpointTimeoutReadWrite(testTimeoutReadWrite)(dialerEndpoint)
		SignerDialerEndpointConnRetries(10)(dialerEndpoint)

		signerServer := NewSignerServer(dialerEndpoint, chainID, mockPV)

		startListenerEndpointAsync(ctx, t, listenerEndpoint, endpointIsOpenCh)

		require.NoError(t, signerServer.Start(ctx))
		assert.True(t, signerServer.IsRunning())
		t.Cleanup(signerServer.Wait)

		<-endpointIsOpenCh
		signerServer.Stop()

		dialerEndpoint2 := NewSignerDialerEndpoint(
			logger,
			tc.dialer,
		)
		signerServer2 := NewSignerServer(dialerEndpoint2, chainID, mockPV)

		// let some pings pass
		require.NoError(t, signerServer2.Start(ctx))
		assert.True(t, signerServer2.IsRunning())
		t.Cleanup(signerServer2.Stop)
		t.Cleanup(signerServer2.Wait)

		// give the client some time to re-establish the conn to the remote signer
		// should see sth like this in the logs:
		//
		// E[10016-01-10|17:12:46.128] Ping                                         err="remote signer timed out"
		// I[10016-01-10|17:16:42.447] Re-created connection to remote signer       impl=SocketVal
		time.Sleep(testTimeoutReadWrite * 2)
	}
}

func newSignerListenerEndpoint(t *testing.T, logger log.Logger, addr string, timeoutReadWrite time.Duration) *SignerListenerEndpoint {
	proto, address := tmnet.ProtocolAndAddress(addr)

	ln, err := net.Listen(proto, address)
	require.NoError(t, err)

	var listener net.Listener

	if proto == "unix" {
		unixLn := NewUnixListener(ln)
		UnixListenerTimeoutAccept(testTimeoutAccept)(unixLn)
		UnixListenerTimeoutReadWrite(timeoutReadWrite)(unixLn)
		listener = unixLn
	} else {
		tcpLn := NewTCPListener(ln, ed25519.GenPrivKey())
		TCPListenerTimeoutAccept(testTimeoutAccept)(tcpLn)
		TCPListenerTimeoutReadWrite(timeoutReadWrite)(tcpLn)
		listener = tcpLn
	}

	return NewSignerListenerEndpoint(
		logger,
		listener,
		SignerListenerEndpointTimeoutReadWrite(testTimeoutReadWrite),
	)
}

func startListenerEndpointAsync(
	ctx context.Context,
	t *testing.T,
	sle *SignerListenerEndpoint,
	endpointIsOpenCh chan struct{},
) {
	t.Helper()

	go func(sle *SignerListenerEndpoint) {
		require.NoError(t, sle.Start(ctx))
		assert.True(t, sle.IsRunning())
		close(endpointIsOpenCh)
	}(sle)
}

func getMockEndpoints(
	ctx context.Context,
	t *testing.T,
	logger log.Logger,
	addr string,
	socketDialer SocketDialer,
) (*SignerListenerEndpoint, *SignerDialerEndpoint) {

	var (
		endpointIsOpenCh = make(chan struct{})

		dialerEndpoint = NewSignerDialerEndpoint(
			logger,
			socketDialer,
		)

		listenerEndpoint = newSignerListenerEndpoint(t, logger, addr, testTimeoutReadWrite)
	)

	SignerDialerEndpointTimeoutReadWrite(testTimeoutReadWrite)(dialerEndpoint)
	SignerDialerEndpointConnRetries(1e6)(dialerEndpoint)

	startListenerEndpointAsync(ctx, t, listenerEndpoint, endpointIsOpenCh)

	require.NoError(t, dialerEndpoint.Start(ctx))
	assert.True(t, dialerEndpoint.IsRunning())

	<-endpointIsOpenCh

	return listenerEndpoint, dialerEndpoint
}
