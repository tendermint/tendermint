package privval

import (
	"fmt"
	"net"
	"sync"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
)

// SignerValidatorEndpointOption sets an optional parameter on the SocketVal.
type SignerValidatorEndpointOption func(*SignerListenerEndpoint)

// SignerListenerEndpoint listens for an external process to dial in
// and keeps the connection alive by dropping and reconnecting
type SignerListenerEndpoint struct {
	cmn.BaseService

	mtx      sync.Mutex
	listener net.Listener
	conn     net.Conn

	timeoutReadWrite time.Duration
}

// NewSignerListenerEndpoint returns an instance of SignerListenerEndpoint.
func NewSignerListenerEndpoint(logger log.Logger, listener net.Listener) *SignerListenerEndpoint {

	sc := &SignerListenerEndpoint{
		listener:         listener,
		timeoutReadWrite: defaultTimeoutReadWriteSeconds * time.Second,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "SignerListenerEndpoint", sc)

	return sc
}

// OnStart implements cmn.Service.
func (ve *SignerListenerEndpoint) OnStart() error {
	ve.Logger.Debug("SignerListenerEndpoint: OnStart")

	err := ve.acceptNewConnection()
	if err != nil {
		ve.Logger.Error("OnStart", "err", err)
		return err
	}

	ve.Logger.Debug("SignerListenerEndpoint OnStart", "connected", ve.isConnected())
	return nil
}

// OnStop implements cmn.Service.
func (ve *SignerListenerEndpoint) OnStop() {
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
	ve.mtx.Lock()
	defer ve.mtx.Unlock()

	// TODO(jleni): Pass maxwait through
	return ve.ensureConnection()
}

// Close closes the underlying net.Conn.
func (ve *SignerListenerEndpoint) Close() error {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()

	ve.Logger.Debug("SignerListenerEndpoint: Close")

	ve.dropConnection()

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

	ve.Logger.Debug("SignerListenerEndpoint: Send request", "connected", ve.isConnected())
	err := ve.ensureConnection()
	if err != nil {
		return nil, err
	}

	ve.Logger.Debug("Send request. Write")

	err = ve.writeMessage(request)
	if err != nil {
		return nil, err
	}

	ve.Logger.Debug("Send request. Read")

	res, err := ve.readMessage()
	if err != nil {
		ve.Logger.Debug("Read Error", "err", err)
		return nil, err
	}

	return res, nil
}

// IsConnected indicates if there is an active connection
func (ve *SignerListenerEndpoint) isConnected() bool {
	return ve.IsRunning() && ve.conn != nil
}

func (ve *SignerListenerEndpoint) ensureConnection() error {
	if !ve.isConnected() {
		err := ve.acceptNewConnection()
		if err != nil {
			return cmn.ErrorWrap(ErrListenerNoConnection, "could not reconnect")
		}
	}
	return nil
}

// dropConnection closes the current connection but does not touch the listening socket
func (ve *SignerListenerEndpoint) dropConnection() {
	ve.Logger.Debug("SignerListenerEndpoint: dropConnection")

	if ve.conn != nil {
		if err := ve.conn.Close(); err != nil {
			ve.Logger.Error("Closing connection", "err", err)
		}
		ve.conn = nil
	}
}

func (ve *SignerListenerEndpoint) readMessage() (msg RemoteSignerMsg, err error) {
	if !ve.isConnected() {
		return nil, cmn.ErrorWrap(ErrListenerNoConnection, "endpoint is not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(ve.timeoutReadWrite)
	ve.Logger.Debug(
		"SignerListenerEndpoint: readMessage",
		"timeout", ve.timeoutReadWrite,
		"deadline", deadline)

	err = ve.conn.SetReadDeadline(deadline)
	if err != nil {
		return
	}

	const maxRemoteSignerMsgSize = 1024 * 10
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(ve.conn, &msg, maxRemoteSignerMsgSize)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrListenerTimeout, err.Error())
		ve.dropConnection()
	}

	return
}

func (ve *SignerListenerEndpoint) writeMessage(msg RemoteSignerMsg) (err error) {
	if !ve.isConnected() {
		return cmn.ErrorWrap(ErrListenerNoConnection, "endpoint is not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(ve.timeoutReadWrite)
	ve.Logger.Debug(
		"SignerListenerEndpoint: writeMessage",
		"timeout", ve.timeoutReadWrite,
		"deadline", deadline)

	err = ve.conn.SetWriteDeadline(deadline)
	if err != nil {
		return
	}

	_, err = cdc.MarshalBinaryLengthPrefixedWriter(ve.conn, msg)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrListenerTimeout, err.Error())
	}

	return
}

func (ve *SignerListenerEndpoint) acceptNewConnection() error {
	ve.Logger.Debug("SignerListenerEndpoint: AcceptNewConnection")

	if !ve.IsRunning() || ve.listener == nil {
		return fmt.Errorf("endpoint is closing")
	}

	// if the conn already exists and close it.
	if ve.conn != nil {
		if tmpErr := ve.conn.Close(); tmpErr != nil {
			ve.Logger.Error("AcceptNewConnection: error closing old connection", "err", tmpErr)
		}
	}

	// Forget old connection
	ve.conn = nil

	// wait for a new conn
	conn, err := ve.listener.Accept()
	if err != nil {
		ve.Logger.Debug("listener accept failed", "err", err)
		return err
	}

	ve.conn = conn
	ve.Logger.Info("SignerListenerEndpoint: New connection")

	return nil
}
