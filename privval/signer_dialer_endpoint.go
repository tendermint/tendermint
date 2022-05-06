package privval

import (
	"context"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

const (
	defaultMaxDialRetries        = 10
	defaultRetryWaitMilliseconds = 100
)

// SignerServiceEndpointOption sets an optional parameter on the SignerDialerEndpoint.
type SignerServiceEndpointOption func(*SignerDialerEndpoint)

// SignerDialerEndpointTimeoutReadWrite sets the read and write timeout for
// connections from client processes.
func SignerDialerEndpointTimeoutReadWrite(timeout time.Duration) SignerServiceEndpointOption {
	return func(ss *SignerDialerEndpoint) { ss.timeoutReadWrite = timeout }
}

// SignerDialerEndpointConnRetries sets the amount of attempted retries to
// acceptNewConnection.
func SignerDialerEndpointConnRetries(retries int) SignerServiceEndpointOption {
	return func(ss *SignerDialerEndpoint) { ss.maxConnRetries = retries }
}

// SignerDialerEndpointRetryWaitInterval sets the retry wait interval to a
// custom value.
func SignerDialerEndpointRetryWaitInterval(interval time.Duration) SignerServiceEndpointOption {
	return func(ss *SignerDialerEndpoint) { ss.retryWait = interval }
}

// SignerDialerEndpoint dials using its dialer and responds to any signature
// requests using its privVal.
type SignerDialerEndpoint struct {
	signerEndpoint

	dialer SocketDialer

	retryWait      time.Duration
	maxConnRetries int
}

// NewSignerDialerEndpoint returns a SignerDialerEndpoint that will dial using the given
// dialer and respond to any signature requests over the connection
// using the given privVal.
func NewSignerDialerEndpoint(
	logger log.Logger,
	dialer SocketDialer,
	options ...SignerServiceEndpointOption,
) *SignerDialerEndpoint {

	sd := &SignerDialerEndpoint{
		dialer:         dialer,
		retryWait:      defaultRetryWaitMilliseconds * time.Millisecond,
		maxConnRetries: defaultMaxDialRetries,
	}
	sd.signerEndpoint.logger = logger

	sd.BaseService = *service.NewBaseService(logger, "SignerDialerEndpoint", sd)
	sd.signerEndpoint.timeoutReadWrite = defaultTimeoutReadWriteSeconds * time.Second

	for _, optionFunc := range options {
		optionFunc(sd)
	}

	return sd
}

func (sd *SignerDialerEndpoint) OnStart(context.Context) error { return nil }
func (sd *SignerDialerEndpoint) OnStop()                       {}

func (sd *SignerDialerEndpoint) ensureConnection(ctx context.Context) error {
	if sd.IsConnected() {
		return nil
	}

	timer := time.NewTimer(0)
	defer timer.Stop()
	retries := 0
	for retries < sd.maxConnRetries {
		if err := ctx.Err(); err != nil {
			return err
		}
		conn, err := sd.dialer()

		if err != nil {
			retries++
			sd.logger.Debug("SignerDialer: Reconnection failed", "retries", retries, "max", sd.maxConnRetries, "err", err)

			// Wait between retries
			timer.Reset(sd.retryWait)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		} else {
			sd.SetConnection(conn)
			sd.logger.Debug("SignerDialer: Connection Ready")
			return nil
		}
	}

	sd.logger.Debug("SignerDialer: Max retries exceeded", "retries", retries, "max", sd.maxConnRetries)

	return ErrNoConnection
}
