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

	stopCh, stoppedCh chan struct{}
	connectCh         chan struct{}
	connectedCh       chan net.Conn
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

	ve.connectCh = make(chan struct{})
	ve.connectedCh = make(chan net.Conn)

	go ve.serviceLoop()
	ve.connectCh <- struct{}{}

	return nil
}

// OnStop implements cmn.Service.
func (ve *SignerListenerEndpoint) OnStop() {
	ve.Logger.Debug("SignerListenerEndpoint: OnStop calling Close")
	_ = ve.Close()

	ve.Logger.Debug("SignerListenerEndpoint: OnStop stop listening")
	// Stop listening
	if ve.listener != nil {
		if err := ve.listener.Close(); err != nil {
			ve.Logger.Error("Closing Listener", "err", err)
		}
	}

	// Stop service loop
	ve.stopCh <- struct{}{}
	<-ve.stoppedCh
}

// Close closes the underlying net.Conn.
func (ve *SignerListenerEndpoint) Close() error {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()
	ve.Logger.Debug("SignerListenerEndpoint: Close")

	ve.dropConnection()
	return nil
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
	return ve.ensureConnection(maxWait)
}

// SendRequest sends a request and waits for a response
func (ve *SignerListenerEndpoint) SendRequest(request RemoteSignerMsg) (RemoteSignerMsg, error) {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()

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

func (ve *SignerListenerEndpoint) ensureConnection(maxWait time.Duration) error {
	if !ve.isConnected() {
		// Is there a connection ready?
		select {
		case ve.conn = <-ve.connectedCh:
			{
				ve.Logger.Debug("SignerListenerEndpoint: received connection")
				return nil
			}
		default:
			{
				ve.Logger.Debug("SignerListenerEndpoint: no connection is ready")
			}
		}

		// should we trigger a reconnect?
		select {
		case ve.connectCh <- struct{}{}:
			{
				ve.Logger.Debug("SignerListenerEndpoint: triggered a reconnect")
			}
		default:
			{
				ve.Logger.Debug("SignerListenerEndpoint: reconnect in progress")
			}
		}

		// block until connected or timeout
		select {
		case ve.conn = <-ve.connectedCh:
			{
				ve.Logger.Debug("SignerListenerEndpoint: connected")
			}
		case <-time.After(maxWait):
			{
				ve.Logger.Debug("SignerListenerEndpoint: timeout")
				return ErrListenerTimeout
			}
		}
	}
	return nil
}

// dropConnection closes the current connection but does not touch the listening socket
func (ve *SignerListenerEndpoint) dropConnection() {
	if ve.conn != nil {
		if err := ve.conn.Close(); err != nil {
			ve.Logger.Error("SignerListenerEndpoint::dropConnection", "err", err)
		}
		ve.conn = nil
	}
}

func (ve *SignerListenerEndpoint) serviceLoop() {
	defer close(ve.stoppedCh)
	ve.Logger.Debug("SignerListenerEndpoint::serviceLoop")

	for {
		select {
		case <-ve.connectCh:
			{
				for {
					ve.Logger.Info("Listening for new connection")
					conn, err := ve.acceptNewConnection()
					if err == nil {
						ve.Logger.Info("Connected")

						select {
						case ve.connectedCh <- conn:
							{
								ve.Logger.Debug("SignerListenerEndpoint: connection relayed")
							}
						case <-ve.stopCh:
							{
								ve.Logger.Debug("SignerListenerEndpoint: stopping")
								return
							}
						}
					}
				}
			}

		case <-ve.stopCh:
			{
				ve.Logger.Debug("SignerListenerEndpoint::serviceLoop Stop")
				return
			}
		}
	}
}

func (ve *SignerListenerEndpoint) acceptNewConnection() (net.Conn, error) {
	ve.Logger.Debug("SignerListenerEndpoint: AcceptNewConnection")

	if !ve.IsRunning() || ve.listener == nil {
		return nil, fmt.Errorf("endpoint is closing")
	}

	// wait for a new conn
	conn, err := ve.listener.Accept()
	if err != nil {
		ve.Logger.Debug("listener accept failed", "err", err)
		return nil, err
	}

	ve.Logger.Info("SignerListenerEndpoint: New connection")
	return conn, nil
}
