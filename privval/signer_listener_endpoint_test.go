package privval

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

var (
	testTimeoutAccept = defaultTimeoutAcceptSeconds * time.Second

	testTimeoutReadWrite    = 100 * time.Millisecond
	testTimeoutReadWrite2o3 = 66 * time.Millisecond // 2/3 of the other one
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

	serviceEndpoint := NewSignerDialerEndpoint(
		log.TestingLogger(),
		common.RandStr(12),
		types.NewMockPV(),
		DialTCPFn(ln.Addr().String(), testTimeoutReadWrite, ed25519.GenPrivKey()),
	)
	defer serviceEndpoint.Stop()

	SignerDialerEndpointTimeoutReadWrite(time.Millisecond)(serviceEndpoint)
	SignerDialerEndpointConnRetries(retries)(serviceEndpoint)

	err = serviceEndpoint.Start()
	assert.NoError(t, err)

	select {
	case attempts := <-attemptCh:
		assert.Equal(t, retries, attempts)
	case <-time.After(1500 * time.Millisecond):
		t.Error("expected remote to observe connection attempts")
	}
}

func TestRetryConnToRemoteSigner(t *testing.T) {
	for _, tc := range getDialerTestCases(t) {
		func() {
			var (
				logger  = log.TestingLogger()
				chainID = common.RandStr(12)
				readyCh = make(chan struct{})

				serviceEndpoint = NewSignerDialerEndpoint(
					logger,
					chainID,
					types.NewMockPV(),
					tc.dialer,
				)
				thisConnTimeout   = testTimeoutReadWrite
				validatorEndpoint = newSignerValidatorEndpoint(logger, tc.addr, thisConnTimeout)
			)

			SignerDialerEndpointTimeoutReadWrite(testTimeoutReadWrite)(serviceEndpoint)
			SignerDialerEndpointConnRetries(10)(serviceEndpoint)

			getStartEndpoint(t, readyCh, validatorEndpoint)
			defer validatorEndpoint.Stop()
			require.NoError(t, serviceEndpoint.Start())
			assert.True(t, serviceEndpoint.IsRunning())

			<-readyCh

			serviceEndpoint.Stop()
			rs2 := NewSignerDialerEndpoint(
				logger,
				chainID,
				types.NewMockPV(),
				tc.dialer,
			)
			// let some pings pass
			require.NoError(t, rs2.Start())
			assert.True(t, rs2.IsRunning())
			defer rs2.Stop()

			// give the client some time to re-establish the conn to the remote signer
			// should see sth like this in the logs:
			//
			// E[10016-01-10|17:12:46.128] Ping                                         err="remote signer timed out"
			// I[10016-01-10|17:16:42.447] Re-created connection to remote signer       impl=SocketVal
			time.Sleep(testTimeoutReadWrite * 2)
		}()
	}
}

///////////////////////////////////

func newSignerValidatorEndpoint(logger log.Logger, addr string, timeoutReadWrite time.Duration) *SignerListenerEndpoint {
	proto, address := common.ProtocolAndAddress(addr)

	ln, err := net.Listen(proto, address)
	logger.Info("Listening at", "proto", proto, "address", address)
	if err != nil {
		panic(err)
	}

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

	return NewSignerListenerEndpoint(logger, listener)
}

func getStartEndpoint(t *testing.T, readyCh chan struct{}, sv *SignerListenerEndpoint) {
	go func(sv *SignerListenerEndpoint) {
		require.NoError(t, sv.Start())
		assert.True(t, sv.IsRunning())
		readyCh <- struct{}{}
	}(sv)
}

func getMockEndpoints(
	t *testing.T,
	chainID string,
	privValidator types.PrivValidator,
	addr string,
	socketDialer SocketDialer,
) (*SignerListenerEndpoint, *SignerDialerEndpoint) {

	var (
		logger          = log.TestingLogger()
		privVal         = privValidator
		readyCh         = make(chan struct{})
		serviceEndpoint = NewSignerDialerEndpoint(
			logger,
			chainID,
			privVal,
			socketDialer,
		)

		validatorEndpoint = newSignerValidatorEndpoint(logger, addr, testTimeoutReadWrite)
	)

	SignerDialerEndpointTimeoutReadWrite(testTimeoutReadWrite)(serviceEndpoint)
	SignerDialerEndpointConnRetries(1e6)(serviceEndpoint)

	getStartEndpoint(t, readyCh, validatorEndpoint)

	require.NoError(t, serviceEndpoint.Start())
	assert.True(t, serviceEndpoint.IsRunning())

	<-readyCh

	return validatorEndpoint, serviceEndpoint
}
