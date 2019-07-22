package privval

import (
	"io"
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

// SignerDialerEndpointConnRetries sets the amount of attempted retries to acceptNewConnection.
func SignerDialerEndpointConnRetries(retries int) SignerServiceEndpointOption {
	return func(ss *SignerDialerEndpoint) { ss.maxConnRetries = retries }
}

// SignerDialerEndpoint dials using its dialer and responds to any
// signature requests using its privVal.
type SignerDialerEndpoint struct {
	signerEndpoint

	dialer SocketDialer
	chainID string
	privVal types.PrivValidator

	retryWait      time.Duration
	maxConnRetries int

	// TODO: Unify
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
	sd.Logger.Debug("SignerDialerEndpoint: OnStop calling Close")

	// Stop service loop
	close(sd.stopServiceLoopCh)
	<-sd.stoppedServiceLoopCh

	_ = sd.Close()
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

	sd.Logger.Error("Before handling request")
	res, err := HandleValidatorRequest(req, sd.chainID, sd.privVal)

	if err != nil {
		// only log the error; we'll reply with an error in res
		sd.Logger.Error("handleMessage handleMessage", "err", err)
	}

	sd.Logger.Error("calling write")
	err = sd.writeMessage(res)
	if err != nil {
		sd.Logger.Error("handleMessage writeMessage", "err", err)
		return
	}
}

func (sd *SignerDialerEndpoint) serviceLoop() {
	defer close(sd.stoppedServiceLoopCh)

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

		case <-sd.stopServiceLoopCh:
			return
		}
	}
}
