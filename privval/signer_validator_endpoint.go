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
type SignerValidatorEndpointOption func(*SignerValidatorEndpoint)

// SignerValidatorEndpointSetHeartbeat sets the period on which to check the liveness of the
// connected Signer connections.
func SignerValidatorEndpointSetHeartbeat(period time.Duration) SignerValidatorEndpointOption {
	return func(sc *SignerValidatorEndpoint) { sc.heartbeatPeriod = period }
}


// TODO: Add a type for SignerEndpoints
// getConnection
// connect
// read
// write
// close

// TODO: Fix comments
// SocketVal implements PrivValidator.
// It listens for an external process to dial in and uses
// the socket to request signatures.
type SignerValidatorEndpoint struct {
	cmn.BaseService

	mtx sync.Mutex
	listener net.Listener
	conn net.Conn

	// ping
	cancelPingCh    chan struct{}
	pingTicker      *time.Ticker
	heartbeatPeriod time.Duration
}

// NewSignerValidatorEndpoint returns an instance of SignerValidatorEndpoint.
func NewSignerValidatorEndpoint(logger log.Logger, listener net.Listener) *SignerValidatorEndpoint {
	sc := &SignerValidatorEndpoint{
		listener:        listener,
		heartbeatPeriod: heartbeatPeriod,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "SignerValidatorEndpoint", sc)

	return sc
}

// OnStart implements cmn.Service.
func (ve *SignerValidatorEndpoint) OnStart() error {
	closed, err := ve.connect()
	// TODO: Improve. Connection state should be kept in a variable

	if err != nil {
		ve.Logger.Error("OnStart", "err", err)
		return err
	}

	if closed {
		return fmt.Errorf("listener is closed")
	}

	// Start a routine to keep the connection alive
	ve.cancelPingCh = make(chan struct{}, 1)
	ve.pingTicker = time.NewTicker(ve.heartbeatPeriod)

	// TODO: Move subroutine to another place?
	go func() {
		for {
			select {
			case <-ve.pingTicker.C:
				err := ve.ping()
				if err != nil {
					ve.Logger.Error("Ping", "err", err)
					if err == ErrUnexpectedResponse {
						return
					}

					closed, err := ve.connect()
					if err != nil {
						ve.Logger.Error("Reconnecting to remote signer failed", "err", err)
						continue
					}
					if closed {
						ve.Logger.Info("listener is closing")
						return
					}

					ve.Logger.Info("Re-created connection to remote signer", "impl", ve)
				}
			case <-ve.cancelPingCh:
				ve.pingTicker.Stop()
				return
			}
		}
	}()

	return nil
}

// OnStop implements cmn.Service.
func (ve *SignerValidatorEndpoint) OnStop() {
	if ve.cancelPingCh != nil {
		close(ve.cancelPingCh)
	}
	_ = ve.Close()
}

// Close closes the underlying net.Conn.
func (ve *SignerValidatorEndpoint) Close() error {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()

	if ve.conn != nil {
		if err := ve.conn.Close(); err != nil {
			ve.Logger.Error("Closing connection", "err", err)
			return err
		}
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
func (ve *SignerValidatorEndpoint) SendRequest(request RemoteSignerMsg) (RemoteSignerMsg, error) {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()

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

// Ping is used to check connection health.
func (ve *SignerValidatorEndpoint) ping() error {
	response, err := ve.SendRequest(&PingRequest{})

	if err != nil {
		return err
	}

	_, ok := response.(*PingResponse)
	if !ok {
		return ErrUnexpectedResponse
	}

	return nil
}

func (ve *SignerValidatorEndpoint) readMessage() (msg RemoteSignerMsg, err error) {
	// TODO: Check connection status

	const maxRemoteSignerMsgSize = 1024 * 10
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(ve.conn, &msg, maxRemoteSignerMsgSize)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrConnTimeout, err.Error())
	}

	return
}

func (ve *SignerValidatorEndpoint) writeMessage(msg RemoteSignerMsg) (err error) {
	// TODO: Check connection status

	_, err = cdc.MarshalBinaryLengthPrefixedWriter(ve.conn, msg)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrConnTimeout, err.Error())
	}

	return
}

// waits to accept and sets a new connection.
// connection is closed in OnStop.
// returns true if the listener is closed (ie. it returns a nil conn).
// TODO: Improve this
func (ve *SignerValidatorEndpoint) connect() (closed bool, err error) {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()

	// first check if the conn already exists and close it.
	if ve.conn != nil {
		if tmpErr := ve.conn.Close(); tmpErr != nil {
			ve.Logger.Error("error closing socket val connection during connect", "err", tmpErr)
		}
	}

	// wait for a new conn
	ve.conn, err = ve.acceptConnection()
	if err != nil {
		return false, err
	}

	// listener is closed
	if ve.conn == nil {
		return true, nil
	}

	if err != nil {
		// TODO: This does not belong here... but maybe we need to inform the owner that a connection has been received
		// failed to fetch the pubkey. close out the connection.
		if tmpErr := ve.conn.Close(); tmpErr != nil {
			ve.Logger.Error("error closing connection", "err", tmpErr)
		}
		return false, err
	}
	return false, nil
}

// acceptConnection attempts to accept a connection
// it will timeout after the listener's timeoutAccept
// TODO: There is no reason for this separate accept
func (ve *SignerValidatorEndpoint) acceptConnection() (net.Conn, error) {
	conn, err := ve.listener.Accept()
	if err != nil {
		if !ve.IsRunning() {
			return nil, nil // Ignore error from listener closing.
		}
		return nil, err
	}
	return conn, nil
}
