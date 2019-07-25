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
	signerEndpoint

	listener    net.Listener
	connectCh   chan struct{}
	connectedCh chan net.Conn

	timeoutAccept time.Duration
	mtx           sync.Mutex

	// TODO: Unify
	stopServiceLoopCh    chan struct{}
	stoppedServiceLoopCh chan struct{}
	stoppedPingLoopCh    chan struct{}
	pingTimer            *time.Ticker
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
	sl.stoppedPingLoopCh = make(chan struct{})

	sl.pingTimer = time.NewTicker(defaultPingPeriodMilliseconds * time.Millisecond)

	/// FIXME: Specific
	sl.connectCh = make(chan struct{})
	sl.connectedCh = make(chan net.Conn)
	go sl.serviceLoop()
	go sl.pingLoop()
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

	sl.pingTimer.Stop()

	// Stop service/ping loops
	close(sl.stopServiceLoopCh)

	<-sl.stoppedServiceLoopCh
	<-sl.stoppedPingLoopCh
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

	sl.Logger.Debug("SignerListener::EnsureConnection")
	err := sl.ensureConnection(sl.timeoutAccept)
	if err != nil {
		return nil, err
	}

	sl.Logger.Debug("SignerListener::Write")
	err = sl.WriteMessage(request)
	if err != nil {
		sl.Logger.Debug("SignerListener::Write error")
		return nil, err
	}
	sl.Logger.Debug("SignerListener::WriteFinished")

	sl.Logger.Debug("SignerListener::Read")
	res, err := sl.ReadMessage()
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

	sl.Logger.Info("SignerListener: New connection")
	return conn, nil
}

func (sl *SignerListenerEndpoint) serviceLoop() {
	defer close(sl.stoppedServiceLoopCh)

	for {
		select {
		case <-sl.connectCh:
			{
				sl.Logger.Info("SignerListener: Listening for new connection")

				conn, err := sl.acceptNewConnection()
				if err == nil {
					sl.Logger.Info("SignerListener: Connected")

					// We have a good connection, wait for someone that needs one or cancellation
					select {
					case sl.connectedCh <- conn:
						sl.Logger.Debug("SignerListener: Returned connection")
					case <-sl.stopServiceLoopCh:
						sl.Logger.Debug("SignerListener: Stopping service")
						return
					}
				}

				select {
				case sl.connectCh <- struct{}{}:
				default:
				}

				sl.Logger.Debug("SignerListener: service loop end ")
			}
		case <-sl.stopServiceLoopCh:
			return
		}
	}
}

func (sl *SignerListenerEndpoint) pingLoop() {
	defer close(sl.stoppedPingLoopCh)

	for {
		select {
		case <-sl.pingTimer.C:
			{
				sl.Logger.Info("Ping!")
				_, err := sl.SendRequest(&PingRequest{})

				if err != nil {
					sl.Logger.Error("SignerListener: Ping timeout")
					// Drop and try to reconnect
					sl.DropConnection()
					sl.connectCh <- struct{}{}
				} else {
					sl.Logger.Info("Pong!")
				}
			}
		case <-sl.stopServiceLoopCh:
			return
		}
	}
}
