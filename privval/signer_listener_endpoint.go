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

	extMtx   sync.Mutex
	listener net.Listener
	conn     net.Conn

	timeoutReadWrite time.Duration

	stopCh, stoppedCh chan struct{}
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

	ve.stopCh = make(chan struct{})
	ve.stoppedCh = make(chan struct{})

	go ve.serviceLoop()

	return nil
}

// OnStop implements cmn.Service.
func (ve *SignerListenerEndpoint) OnStop() {
	ve.Logger.Debug("SignerListenerEndpoint: OnStop calling Close")
	_ = ve.Close()

	// Stop listening
	if ve.listener != nil {
		if err := ve.listener.Close(); err != nil {
			ve.Logger.Error("Closing Listener", "err", err)
		}
	}

	// Stop service loop
	close(ve.stopCh)
	<-ve.stoppedCh
}

// Close closes the underlying net.Conn.
func (ve *SignerListenerEndpoint) Close() error {
	ve.extMtx.Lock()
	defer ve.extMtx.Unlock()
	ve.Logger.Debug("SignerListenerEndpoint: Close")

	ve.dropConnection()

	return nil
}

// IsConnected indicates if there is an active connection
func (ve *SignerListenerEndpoint) IsConnected() bool {
	ve.extMtx.Lock()
	defer ve.extMtx.Unlock()
	return ve.isConnected()
}

// WaitForConnection waits maxWait for a connection or returns a timeout error
func (ve *SignerListenerEndpoint) WaitForConnection(maxWait time.Duration) error {
	ve.extMtx.Lock()
	defer ve.extMtx.Unlock()
	return ve.ensureConnection(maxWait)
}

// SendRequest sends a request and waits for a response
func (ve *SignerListenerEndpoint) SendRequest(request RemoteSignerMsg) (RemoteSignerMsg, error) {
	ve.extMtx.Lock()
	defer ve.extMtx.Unlock()

	// TODO: Add retries.. that include dropping the connection and

	ve.Logger.Debug("SignerListenerEndpoint: Send request", "connected", ve.isConnected())
	err := ve.ensureConnection(ve.timeoutReadWrite)
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

func (ve *SignerListenerEndpoint) ensureConnection(maxWait time.Duration) error {
	// TODO: Check that is connected
	if !ve.isConnected() {
	}
	return nil
}

// dropConnection closes the current connection but does not touch the listening socket
func (ve *SignerListenerEndpoint) dropConnection() {
	ve.Logger.Debug("SignerListenerEndpoint: dropConnection")
	if ve.conn != nil {
		if err := ve.conn.Close(); err != nil {
			ve.Logger.Error("SignerListenerEndpoint::dropConnection", "err", err)
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

func (ve *SignerListenerEndpoint) serviceLoop() {
	defer close(ve.stoppedCh)

	for {
		select {
		default:
			{
				ve.Logger.Debug("Listening for new connection")
				_ = ve.acceptNewConnection()
			}

		case <-ve.stopCh:
			{
				return
			}
		}
	}
}

func (ve *SignerListenerEndpoint) acceptNewConnection() error {
	// TODO: add proper locking
	ve.Logger.Debug("SignerListenerEndpoint: AcceptNewConnection")

	if !ve.IsRunning() || ve.listener == nil {
		return fmt.Errorf("endpoint is closing")
	}

	// TODO: add proper locking
	// if the conn already exists and close it.
	ve.dropConnection()

	// wait for a new conn
	conn, err := ve.listener.Accept()
	if err != nil {
		ve.Logger.Debug("listener accept failed", "err", err)
		ve.conn = nil // Explicitly set to nil because dialer returns an interface (https://golang.org/doc/faq#nil_error)
		return err
	}

	ve.conn = conn
	ve.Logger.Info("SignerListenerEndpoint: New connection")
	// TODO: add proper locking

	return nil
}
