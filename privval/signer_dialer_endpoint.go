package privval

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

const (
	defaultMaxDialRetries        = 10
	defaultRetryWaitMilliseconds = 100
)

// SignerServiceEndpointOption sets an optional parameter on the SignerDialerEndpoint.
type SignerServiceEndpointOption func(*SignerDialerEndpoint)

// SignerDialerEndpointTimeoutReadWrite sets the read and write timeout for connections
// from external signing processes.
func SignerDialerEndpointTimeoutReadWrite(timeout time.Duration) SignerServiceEndpointOption {
	return func(ss *SignerDialerEndpoint) { ss.timeoutReadWrite = timeout }
}

// SignerDialerEndpointConnRetries sets the amount of attempted retries to AcceptNewConnection.
func SignerDialerEndpointConnRetries(retries int) SignerServiceEndpointOption {
	return func(ss *SignerDialerEndpoint) { ss.maxConnRetries = retries }
}

// SignerDialerEndpoint dials using its dialer and responds to any
// signature requests using its privVal.
type SignerDialerEndpoint struct {
	cmn.BaseService

	mtx    sync.Mutex
	dialer SocketDialer
	conn   net.Conn

	timeoutReadWrite time.Duration
	retryWait        time.Duration
	maxConnRetries   int

	chainID string
	privVal types.PrivValidator

	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewSignerDialerEndpoint returns a SignerDialerEndpoint that will dial using the given
// dialer and respond to any signature requests over the connection
// using the given privVal.
func NewSignerDialerEndpoint(
	logger log.Logger,
	chainID string,
	privVal types.PrivValidator,
	dialer SocketDialer,
) *SignerDialerEndpoint {

	se := &SignerDialerEndpoint{
		dialer:           dialer,
		timeoutReadWrite: defaultTimeoutReadWriteSeconds * time.Second,
		retryWait:        defaultRetryWaitMilliseconds * time.Millisecond,
		maxConnRetries:   defaultMaxDialRetries,

		chainID: chainID,
		privVal: privVal,
	}

	se.BaseService = *cmn.NewBaseService(logger, "SignerDialerEndpoint", se)
	return se
}

// OnStart implements cmn.Service.
func (sd *SignerDialerEndpoint) OnStart() error {
	sd.Logger.Debug("SignerDialerEndpoint: OnStart")

	sd.stopCh = make(chan struct{})
	sd.stoppedCh = make(chan struct{})

	go sd.serviceLoop()

	return nil
}

// OnStop implements cmn.Service.
func (sd *SignerDialerEndpoint) OnStop() {
	sd.Logger.Debug("SignerDialerEndpoint: OnStop calling Close")

	// Stop service loop
	close(sd.stopCh)
	<-sd.stoppedCh

	_ = sd.Close()
}

// Close closes the underlying net.Conn.
func (sd *SignerDialerEndpoint) Close() error {
	sd.mtx.Lock()
	defer sd.mtx.Unlock()
	sd.Logger.Debug("SignerDialerEndpoint: Close")

	sd.dropConnection()
	return nil
}

// IsConnected indicates if there is an active connection
func (sd *SignerDialerEndpoint) IsConnected() bool {
	sd.mtx.Lock()
	defer sd.mtx.Unlock()
	return sd.isConnected()
}

func (sd *SignerDialerEndpoint) handleRequest() {
	if !sd.IsRunning() {
		return // Ignore error from listener closing.
	}

	sd.Logger.Info("SignerDialerEndpoint: connected", "timeout", sd.timeoutReadWrite)

	req, err := sd.readMessage()
	if err != nil {
		if err != io.EOF {
			sd.Logger.Error("SignerDialerEndpoint handleMessage", "err", err)
		}
		return
	}

	res, err := HandleValidatorRequest(req, sd.chainID, sd.privVal)

	if err != nil {
		// only log the error; we'll reply with an error in res
		sd.Logger.Error("handleMessage handleMessage", "err", err)
	}

	err = sd.writeMessage(res)
	if err != nil {
		sd.Logger.Error("handleMessage writeMessage", "err", err)
		return
	}
}

func (sd *SignerDialerEndpoint) isConnected() bool {
	return sd.IsRunning() && sd.conn != nil
}

func (sd *SignerDialerEndpoint) readMessage() (msg RemoteSignerMsg, err error) {
	if !sd.isConnected() {
		return nil, fmt.Errorf("not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(sd.timeoutReadWrite)
	sd.Logger.Debug(
		"SignerDialerEndpoint: readMessage",
		"timeout", sd.timeoutReadWrite,
		"deadline", deadline)

	err = sd.conn.SetReadDeadline(deadline)
	if err != nil {
		return
	}

	const maxRemoteSignerMsgSize = 1024 * 10
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(sd.conn, &msg, maxRemoteSignerMsgSize)
	if _, ok := err.(timeoutError); ok {
		err = errors.Wrap(ErrDialerReadTimeout, err.Error())
	}

	return
}

func (sd *SignerDialerEndpoint) writeMessage(msg RemoteSignerMsg) (err error) {
	if !sd.isConnected() {
		return errors.Wrap(ErrListenerNoConnection, "endpoint is not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(sd.timeoutReadWrite)
	sd.Logger.Debug("SignerDialerEndpoint: readMessage",
		"timeout", sd.timeoutReadWrite,
		"deadline", deadline)

	err = sd.conn.SetWriteDeadline(deadline)
	if err != nil {
		return
	}

	_, err = cdc.MarshalBinaryLengthPrefixedWriter(sd.conn, msg)
	if _, ok := err.(timeoutError); ok {
		err = errors.Wrap(ErrDialerWriteTimeout, err.Error())
	}

	return
}

func (sd *SignerDialerEndpoint) dropConnection() {
	if sd.conn != nil {
		if err := sd.conn.Close(); err != nil {
			sd.Logger.Error("SignerDialerEndpoint::dropConnection", "err", err)
		}
		sd.conn = nil
	}
}

func (sd *SignerDialerEndpoint) serviceLoop() {
	defer close(sd.stoppedCh)

	retries := 0
	var err error

	for {
		select {
		default:
			sd.Logger.Debug("Try connect", "retries", retries, "max", sd.maxConnRetries)

			if retries > sd.maxConnRetries {
				sd.Logger.Error("Maximum retries reached", "retries", retries)
				return
			}

			if sd.conn == nil {
				sd.conn, err = sd.dialer()
				if err != nil {
					sd.Logger.Info("Try connect", "err", err)
					sd.conn = nil // Explicitly set to nil because dialer returns an interface (https://golang.org/doc/faq#nil_error)
					retries++

					// Wait between retries
					time.Sleep(sd.retryWait)
					continue
				}
			}

			retries = 0
			sd.handleRequest()

		case <-sd.stopCh:
			return
		}
	}
}
