package privval

import (
	"io"
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

// ValidationRequestHandlerFunc handles different remoteSigner requests
type ValidationRequestHandlerFunc func(
	privVal types.PrivValidator,
	req RemoteSignerMsg,
	chainID string) (RemoteSignerMsg, error)

// SignerDialerEndpointTimeoutReadWrite sets the read and write timeout for connections
// from external signing processes.
func SignerDialerEndpointTimeoutReadWrite(timeout time.Duration) SignerServiceEndpointOption {
	return func(ss *SignerDialerEndpoint) { ss.timeoutReadWrite = timeout }
}

// SignerDialerEndpointConnRetries sets the amount of attempted retries to acceptNewConnection.
func SignerDialerEndpointConnRetries(retries int) SignerServiceEndpointOption {
	return func(ss *SignerDialerEndpoint) { ss.maxConnRetries = retries }
}

// SignerDialerEndpoint dials using its dialer and responds to any
// signature requests using its privVal.
type SignerDialerEndpoint struct {
	signerEndpoint

	dialer  SocketDialer
	chainID string
	privVal types.PrivValidator

	retryWait      time.Duration
	maxConnRetries int

	mtx                      sync.Mutex
	validationRequestHandler ValidationRequestHandlerFunc

	stopServiceLoopCh    chan struct{}
	stoppedServiceLoopCh chan struct{}
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
		dialer:         dialer,
		retryWait:      defaultRetryWaitMilliseconds * time.Millisecond,
		maxConnRetries: defaultMaxDialRetries,
		chainID:        chainID,
		privVal:        privVal,
	}

	se.BaseService = *cmn.NewBaseService(logger, "SignerDialerEndpoint", se)
	se.signerEndpoint.timeoutReadWrite = defaultTimeoutReadWriteSeconds * time.Second
	se.validationRequestHandler = DefaultValidationRequestHandler

	return se
}

// OnStart implements cmn.Service.
func (sd *SignerDialerEndpoint) OnStart() error {
	sd.stopServiceLoopCh = make(chan struct{})
	sd.stoppedServiceLoopCh = make(chan struct{})

	go sd.serviceLoop()

	return nil
}

// OnStop implements cmn.Service.
func (sd *SignerDialerEndpoint) OnStop() {
	sd.Logger.Debug("SignerDialer: OnStop calling Close")

	// Stop service loop
	close(sd.stopServiceLoopCh)
	<-sd.stoppedServiceLoopCh

	_ = sd.Close()
}

// SetRequestHandler override the default function that is used to service requests
func (sd *SignerDialerEndpoint) SetRequestHandler(validationRequestHandler ValidationRequestHandlerFunc) {
	sd.mtx.Lock()
	defer sd.mtx.Unlock()
	sd.validationRequestHandler = validationRequestHandler
}

func (sd *SignerDialerEndpoint) servicePendingRequest() {
	if !sd.IsRunning() {
		return // Ignore error from listener closing.
	}

	req, err := sd.ReadMessage()
	if err != nil {
		if err != io.EOF {
			sd.Logger.Error("SignerDialer: HandleMessage", "err", err)
		}
		return
	}

	var res RemoteSignerMsg
	{
		// limit the scope of the lock
		sd.mtx.Lock()
		defer sd.mtx.Unlock()
		res, err = sd.validationRequestHandler(sd.privVal, req, sd.chainID)
		if err != nil {
			// only log the error; we'll reply with an error in res
			sd.Logger.Error("handleMessage handleMessage", "err", err)
		}
	}

	if res != nil {
		err = sd.WriteMessage(res)
		if err != nil {
			sd.Logger.Error("handleMessage writeMessage", "err", err)
		}
	}
}

func (sd *SignerDialerEndpoint) serviceLoop() {
	defer close(sd.stoppedServiceLoopCh)

	retries := 0
	var err error

	for {
		select {
		default:
			if retries > sd.maxConnRetries {
				sd.Logger.Error("Maximum retries reached", "retries", retries)
				return
			}

			if sd.conn == nil {
				sd.Logger.Debug("SignerDialer: Trying to reconnect", "retries", retries, "max", sd.maxConnRetries)
				sd.conn, err = sd.dialer()
				if err != nil {
					sd.Logger.Error("SignerDialer: Failed connecting", "err", err)
					sd.conn = nil // Explicitly set to nil because dialer returns an interface (https://golang.org/doc/faq#nil_error)
					retries++

					// Wait between retries
					time.Sleep(sd.retryWait)
					continue
				}
			}

			retries = 0
			sd.servicePendingRequest()

		case <-sd.stopServiceLoopCh:
			return
		}
	}
}
