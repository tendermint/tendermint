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

	timeoutAccept    time.Duration
	timeoutReadWrite time.Duration

	stopCh, stoppedCh chan struct{}
	connectCh         chan struct{}
	connectedCh       chan net.Conn
}

// NewSignerListenerEndpoint returns an instance of SignerListenerEndpoint.
func NewSignerListenerEndpoint(logger log.Logger, listener net.Listener) *SignerListenerEndpoint {

	sc := &SignerListenerEndpoint{
		listener:         listener,
		timeoutAccept:    defaultTimeoutAcceptSeconds * time.Second,
		timeoutReadWrite: defaultTimeoutReadWriteSeconds * time.Second,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "SignerListenerEndpoint", sc)

	return sc
}

// OnStart implements cmn.Service.
func (sl *SignerListenerEndpoint) OnStart() error {
	sl.Logger.Debug("SignerListenerEndpoint: OnStart")

	sl.stopCh = make(chan struct{})
	sl.stoppedCh = make(chan struct{})

	sl.connectCh = make(chan struct{})
	sl.connectedCh = make(chan net.Conn)

	go sl.serviceLoop()
	sl.connectCh <- struct{}{}

	return nil
}

// OnStop implements cmn.Service.
func (sl *SignerListenerEndpoint) OnStop() {
	sl.Logger.Debug("SignerListenerEndpoint: OnStop calling Close")
	_ = sl.Close()

	sl.Logger.Debug("SignerListenerEndpoint: OnStop stop listening")
	// Stop listening
	if sl.listener != nil {
		if err := sl.listener.Close(); err != nil {
			sl.Logger.Error("Closing Listener", "err", err)
		}
	}

	// Stop service loop
	close(sl.stopCh)
	<-sl.stoppedCh
}

// Close closes the connection
func (sl *SignerListenerEndpoint) Close() error {
	sl.mtx.Lock()
	defer sl.mtx.Unlock()
	sl.Logger.Debug("SignerListenerEndpoint: Close")

	sl.dropConnection()
	return nil
}

// IsConnected indicates if there is an active connection
func (sl *SignerListenerEndpoint) IsConnected() bool {
	sl.mtx.Lock()
	defer sl.mtx.Unlock()
	return sl.isConnected()
}

// WaitForConnection waits maxWait for a connection or returns a timeout error
func (sl *SignerListenerEndpoint) WaitForConnection(maxWait time.Duration) error {
	sl.mtx.Lock()
	defer sl.mtx.Unlock()
	return sl.ensureConnection(maxWait)
}

// SendRequest ensures there is a connection, sends a request and waits for a response
func (sl *SignerListenerEndpoint) SendRequest(request RemoteSignerMsg) (RemoteSignerMsg, error) {
	sl.mtx.Lock()
	defer sl.mtx.Unlock()

	sl.Logger.Debug("SignerListenerEndpoint: Send request", "connected", sl.isConnected())
	err := sl.ensureConnection(sl.timeoutAccept)
	if err != nil {
		return nil, err
	}

	sl.Logger.Debug("Send request. Write")
	err = sl.writeMessage(request)
	if err != nil {
		return nil, err
	}

	sl.Logger.Debug("Send request. Read")
	res, err := sl.readMessage()
	if err != nil {
		sl.Logger.Debug("Read Error", "err", err)
		return nil, err
	}

	return res, nil
}

func (sl *SignerListenerEndpoint) isConnected() bool {
	return sl.IsRunning() && sl.conn != nil
}

func (sl *SignerListenerEndpoint) readMessage() (msg RemoteSignerMsg, err error) {
	if !sl.isConnected() {
		return nil, cmn.ErrorWrap(ErrListenerNoConnection, "endpoint is not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(sl.timeoutReadWrite)
	sl.Logger.Debug(
		"SignerListenerEndpoint: readMessage",
		"timeout", sl.timeoutReadWrite,
		"deadline", deadline)

	err = sl.conn.SetReadDeadline(deadline)
	if err != nil {
		return
	}

	const maxRemoteSignerMsgSize = 1024 * 10
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(sl.conn, &msg, maxRemoteSignerMsgSize)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrListenerTimeout, err.Error())
		sl.dropConnection()
	}

	return
}

func (sl *SignerListenerEndpoint) writeMessage(msg RemoteSignerMsg) (err error) {
	if !sl.isConnected() {
		return cmn.ErrorWrap(ErrListenerNoConnection, "endpoint is not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(sl.timeoutReadWrite)
	sl.Logger.Debug(
		"SignerListenerEndpoint: writeMessage",
		"timeout", sl.timeoutReadWrite,
		"deadline", deadline)

	err = sl.conn.SetWriteDeadline(deadline)
	if err != nil {
		return
	}

	_, err = cdc.MarshalBinaryLengthPrefixedWriter(sl.conn, msg)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrListenerTimeout, err.Error())
		sl.dropConnection()
	}

	return
}

func (sl *SignerListenerEndpoint) ensureConnection(maxWait time.Duration) error {
	if !sl.isConnected() {
		// Is there a connection ready?
		select {
		case sl.conn = <-sl.connectedCh:
			sl.Logger.Debug("SignerListenerEndpoint: received connection")
			return nil
		default:
			sl.Logger.Debug("SignerListenerEndpoint: no connection is ready")
		}

		// should we trigger a reconnect?
		select {
		case sl.connectCh <- struct{}{}:
			sl.Logger.Debug("SignerListenerEndpoint: triggered a reconnect")
		default:
			sl.Logger.Debug("SignerListenerEndpoint: reconnect in progress")
		}

		// block until connected or timeout
		select {
		case sl.conn = <-sl.connectedCh:
			sl.Logger.Debug("SignerListenerEndpoint: connected")
		case <-time.After(maxWait):
			sl.Logger.Debug("SignerListenerEndpoint: timeout")
			return ErrListenerTimeout
		}
	}

	return nil
}

func (sl *SignerListenerEndpoint) acceptNewConnection() (net.Conn, error) {
	sl.Logger.Debug("SignerListenerEndpoint: AcceptNewConnection")

	if !sl.IsRunning() || sl.listener == nil {
		return nil, fmt.Errorf("endpoint is closing")
	}

	// wait for a new conn
	conn, err := sl.listener.Accept()
	if err != nil {
		sl.Logger.Debug("listener accept failed", "err", err)
		return nil, err
	}

	sl.Logger.Info("SignerListenerEndpoint: New connection")
	return conn, nil
}

func (sl *SignerListenerEndpoint) dropConnection() {
	if sl.conn != nil {
		if err := sl.conn.Close(); err != nil {
			sl.Logger.Error("SignerListenerEndpoint::dropConnection", "err", err)
		}
		sl.conn = nil
	}
}

func (sl *SignerListenerEndpoint) serviceLoop() {
	defer close(sl.stoppedCh)
	sl.Logger.Debug("SignerListenerEndpoint::serviceLoop")

	for {
		select {
		case <-sl.connectCh:
			{
				sl.Logger.Info("Listening for new connection")

				conn, err := sl.acceptNewConnection()
				if err == nil {
					sl.Logger.Info("Connected")

					// We have a good connection, wait for someone that needs one or cancellation
					select {
					case sl.connectedCh <- conn:
						sl.Logger.Debug("SignerListenerEndpoint: connection relayed")
						break
					case <-sl.stopCh:
						sl.Logger.Debug("SignerListenerEndpoint::serviceLoop Stop")
						return
					}
				}

				select {
				case sl.connectCh <- struct{}{}:
				default:
				}
			}
		case <-sl.stopCh:
			sl.Logger.Debug("SignerListenerEndpoint::serviceLoop Stop")
			return
		}
	}
}
