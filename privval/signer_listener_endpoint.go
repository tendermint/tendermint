package privval

import (
	"fmt"
	"net"
	"sync"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
)

const (
	defaultHeartbeatSeconds = 2
	defaultMaxDialRetries   = 10
)

var (
	heartbeatPeriod = time.Second * defaultHeartbeatSeconds
)

// SignerValidatorEndpointOption sets an optional parameter on the SocketVal.
type SignerValidatorEndpointOption func(*SignerListenerEndpoint)

// SignerValidatorEndpointSetHeartbeat sets the period on which to check the liveness of the
// connected Signer connections.
func SignerValidatorEndpointSetHeartbeat(period time.Duration) SignerValidatorEndpointOption {
	return func(sc *SignerListenerEndpoint) { sc.heartbeatPeriod = period }
}

// TODO: Add a type for SignerEndpoints
// getConnection
// tryConnect
// read
// write
// close

// TODO: Fix comments
// SocketVal implements PrivValidator.
// It listens for an external process to dial in and uses
// the socket to request signatures.
type SignerListenerEndpoint struct {
	cmn.BaseService

	mtx      sync.Mutex
	listener net.Listener
	conn     net.Conn

	// ping
	cancelPingCh    chan struct{}
	pingTicker      *time.Ticker
	heartbeatPeriod time.Duration
}

// NewSignerListenerEndpoint returns an instance of SignerListenerEndpoint.
func NewSignerListenerEndpoint(logger log.Logger, listener net.Listener) *SignerListenerEndpoint {
	sc := &SignerListenerEndpoint{
		listener:        listener,
		heartbeatPeriod: heartbeatPeriod,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "SignerListenerEndpoint", sc)

	return sc
}

// OnStart implements cmn.Service.
func (ve *SignerListenerEndpoint) OnStart() error {
	ve.Logger.Debug("SignerListenerEndpoint: OnStart")

	err := ve.tryConnect()
	if err != nil {
		ve.Logger.Error("OnStart", "err", err)
		return err
	}

	// Start a routine to keep the connection alive
	ve.cancelPingCh = make(chan struct{}, 1)
	ve.pingTicker = time.NewTicker(ve.heartbeatPeriod)

	// TODO: Move subroutine to another place?
	go func() {
		for {
			select {
			case <-ve.pingTicker.C:
				ve.Logger.Debug("SignerListenerEndpoint: ping ticker ch")
				err := ve.ping()
				if err != nil {
					ve.Logger.Error("Ping", "err", err)
					if err == ErrUnexpectedResponse {
						return
					}

					err := ve.tryConnect()
					if err != nil {
						ve.Logger.Error("Reconnecting to remote signer failed", "err", err)
						continue
					}

					ve.Logger.Info("Re-created connection to remote signer", "impl", ve)
				}
			case <-ve.cancelPingCh:
				ve.Logger.Debug("SignerListenerEndpoint: cancel ping ch")

				ve.pingTicker.Stop()
				return
			}
		}
	}()

	return nil
}

// OnStop implements cmn.Service.
func (ve *SignerListenerEndpoint) OnStop() {
	ve.Logger.Debug("SignerListenerEndpoint: OnStop")

	if ve.cancelPingCh != nil {
		ve.Logger.Debug("SignerListenerEndpoint: Close cancel ping channel")
		close(ve.cancelPingCh)
		ve.cancelPingCh = nil
	}

	ve.Logger.Debug("SignerListenerEndpoint: OnStop calling Close")
	_ = ve.Close()
}

// IsConnected indicates if there is an active connection
func (ve *SignerListenerEndpoint) IsConnected() bool {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()
	return ve.isConnected()
}

// WaitForConnection waits maxWait for a connection or returns a timeout error
func (ve *SignerListenerEndpoint) WaitForConnection(maxWait time.Duration) error {
	// TODO: complete this
	return nil
}

// Close closes the underlying net.Conn.
func (ve *SignerListenerEndpoint) Close() error {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()

	ve.Logger.Debug("SignerListenerEndpoint: Close")

	if ve.conn != nil {
		if err := ve.conn.Close(); err != nil {
			ve.Logger.Error("Closing connection", "err", err)
			return err
		}
		ve.conn = nil
	}

	if ve.listener != nil {
		if err := ve.listener.Close(); err != nil {
			ve.Logger.Error("Closing Listener", "err", err)
			return err
		}
	}

	return nil
}

// SendRequest sends a request and waits for a response
func (ve *SignerListenerEndpoint) SendRequest(request RemoteSignerMsg) (RemoteSignerMsg, error) {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()

	if !ve.isConnected() {
		return nil, cmn.ErrorWrap(ErrListenerTimeout, "endpoint is not connected")
	}

	err := ve.writeMessage(request)
	if err != nil {
		return nil, err
	}

	res, err := ve.readMessage()
	if err != nil {
		return nil, err
	}

	return res, nil
}

// IsConnected indicates if there is an active connection
func (ve *SignerListenerEndpoint) isConnected() bool {
	return ve.IsRunning() && ve.conn != nil
}

func (ve *SignerListenerEndpoint) readMessage() (msg RemoteSignerMsg, err error) {
	if !ve.isConnected() {
		return nil, cmn.ErrorWrap(ErrListenerTimeout, "endpoint is not connected")
	}

	const maxRemoteSignerMsgSize = 1024 * 10
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(ve.conn, &msg, maxRemoteSignerMsgSize)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrListenerTimeout, err.Error())
	}

	return
}

func (ve *SignerListenerEndpoint) writeMessage(msg RemoteSignerMsg) (err error) {
	if !ve.isConnected() {
		return cmn.ErrorWrap(ErrListenerTimeout, "endpoint is not connected")
	}

	_, err = cdc.MarshalBinaryLengthPrefixedWriter(ve.conn, msg)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrListenerTimeout, err.Error())
	}

	return
}

// tryConnect waits to accept a new connection.
func (ve *SignerListenerEndpoint) tryConnect() error {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()

	ve.Logger.Debug("SignerListenerEndpoint: tryConnect")

	if !ve.IsRunning() || ve.listener == nil {
		return fmt.Errorf("endpoint is closing")
	}

	// if the conn already exists and close it.
	if ve.conn != nil {
		if tmpErr := ve.conn.Close(); tmpErr != nil {
			ve.Logger.Error("error closing socket val connection during tryConnect", "err", tmpErr)
		}
	}

	// Forget old connection
	ve.conn = nil

	// wait for a new conn
	conn, err := ve.listener.Accept()
	if err != nil {
		return err
	}

	ve.Logger.Debug("SignerListenerEndpoint: New connection")

	ve.conn = conn
	// TODO: maybe we need to inform the owner that a connection has been received

	return nil
}

// Ping is used to check connection health.
func (ve *SignerListenerEndpoint) ping() error {
	response, err := ve.SendRequest(&PingRequest{})

	if err != nil {
		return err
	}

	_, ok := response.(*PingResponse)
	if !ok {
		return ErrUnexpectedResponse
	}

	ve.Logger.Debug("SignerListenerEndpoint: pong")

	return nil
}
