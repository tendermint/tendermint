package privval

import (
	"fmt"
	"net"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
)

// SignerValidatorEndpointOption sets an optional parameter on the SocketVal.
type SignerValidatorEndpointOption func(*SignerListenerEndpoint)

// SignerListenerEndpoint listens for an external process to dial in
// and keeps the connection alive by dropping and reconnecting
type SignerListenerEndpoint struct {
	signerEndpoint

	listener net.Listener
	connectCh   chan struct{}
	connectedCh chan net.Conn

	timeoutAccept time.Duration

	// TODO: Unify
	stopServiceLoopCh    chan struct{}
	stoppedServiceLoopCh chan struct{}
}

// NewSignerListenerEndpoint returns an instance of SignerListenerEndpoint.
func NewSignerListenerEndpoint(
	logger log.Logger,
	listener net.Listener,
) *SignerListenerEndpoint {
	sc := &SignerListenerEndpoint{
		listener:      listener,
		timeoutAccept: defaultTimeoutAcceptSeconds * time.Second,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "SignerListenerEndpoint", sc)
	sc.signerEndpoint.timeoutReadWrite = defaultTimeoutReadWriteSeconds * time.Second
	return sc
}

// OnStart implements cmn.Service.
func (sl *SignerListenerEndpoint) OnStart() error {
	sl.stopServiceLoopCh = make(chan struct{})
	sl.stoppedServiceLoopCh = make(chan struct{})

	/// FIXME: Specific
	sl.connectCh = make(chan struct{})
	sl.connectedCh = make(chan net.Conn)
	go sl.serviceLoop()
	sl.connectCh <- struct{}{}
	////

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
	close(sl.stopServiceLoopCh)
	<-sl.stoppedServiceLoopCh
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

func (sl *SignerListenerEndpoint) serviceLoop() {
	defer close(sl.stoppedServiceLoopCh)

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
					case <-sl.stopServiceLoopCh:
						return
					}
				}

				select {
				case sl.connectCh <- struct{}{}:
				default:
				}
			}
		case <-sl.stopServiceLoopCh:
			return
		}
	}
}
