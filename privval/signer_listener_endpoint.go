package privval

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

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
	sl.stopCh = make(chan struct{})
	sl.stoppedCh = make(chan struct{})

	sl.connectCh = make(chan struct{})
	sl.connectedCh = make(chan net.Conn)

	go sl.serviceLoop()
	sl.connectCh <- struct{}{}

	return nil
}

// OnStop implements cmn.Service
func (sl *SignerListenerEndpoint) OnStop() {
	_ = sl.Close()

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

	err := sl.ensureConnection(sl.timeoutAccept)
	if err != nil {
		return nil, err
	}

	err = sl.writeMessage(request)
	if err != nil {
		return nil, err
	}

	res, err := sl.readMessage()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (sl *SignerListenerEndpoint) isConnected() bool {
	return sl.IsRunning() && sl.conn != nil
}

func (sl *SignerListenerEndpoint) readMessage() (msg RemoteSignerMsg, err error) {
	if !sl.isConnected() {
		return nil, errors.Wrap(ErrListenerNoConnection, "endpoint is not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(sl.timeoutReadWrite)

	err = sl.conn.SetReadDeadline(deadline)
	if err != nil {
		return
	}

	const maxRemoteSignerMsgSize = 1024 * 10
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(sl.conn, &msg, maxRemoteSignerMsgSize)
	if _, ok := err.(timeoutError); ok {
		err = errors.Wrap(ErrListenerTimeout, err.Error())
		sl.dropConnection()
	}

	return
}

func (sl *SignerListenerEndpoint) writeMessage(msg RemoteSignerMsg) (err error) {
	if !sl.isConnected() {
		return errors.Wrap(ErrListenerNoConnection, "endpoint is not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(sl.timeoutReadWrite)

	err = sl.conn.SetWriteDeadline(deadline)
	if err != nil {
		return
	}

	_, err = cdc.MarshalBinaryLengthPrefixedWriter(sl.conn, msg)
	if _, ok := err.(timeoutError); ok {
		err = errors.Wrap(ErrListenerTimeout, err.Error())
		sl.dropConnection()
	}

	return
}

func (sl *SignerListenerEndpoint) ensureConnection(maxWait time.Duration) error {
	if !sl.isConnected() {
		// Is there a connection ready?
		select {
		case sl.conn = <-sl.connectedCh:
			return nil
		default:
		}

		// should we trigger a reconnect?
		select {
		case sl.connectCh <- struct{}{}:
			break
		default:
			break
		}

		// block until connected or timeout
		select {
		case sl.conn = <-sl.connectedCh:
			break
		case <-time.After(maxWait):
			return ErrListenerTimeout
		}
	}

	return nil
}

func (sl *SignerListenerEndpoint) acceptNewConnection() (net.Conn, error) {
	if !sl.IsRunning() || sl.listener == nil {
		return nil, fmt.Errorf("endpoint is closing")
	}

	// wait for a new conn
	conn, err := sl.listener.Accept()
	if err != nil {
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
						break
					case <-sl.stopCh:
						return
					}
				}

				select {
				case sl.connectCh <- struct{}{}:
				default:
				}
			}
		case <-sl.stopCh:
			return
		}
	}
}
