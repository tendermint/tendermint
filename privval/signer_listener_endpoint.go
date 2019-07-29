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

	listener              net.Listener
	connectRequestCh      chan struct{}
	connectionAvailableCh chan net.Conn

	timeoutAccept time.Duration
	pingTimer     *time.Ticker

	instanceMtx sync.Mutex // Ensures instance public methods access, i.e. SendRequest
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
	sl.connectRequestCh = make(chan struct{})
	sl.connectionAvailableCh = make(chan net.Conn)

	sl.pingTimer = time.NewTicker(defaultPingPeriodMilliseconds * time.Millisecond)

	go sl.serviceLoop()
	go sl.pingLoop()

	sl.connectRequestCh <- struct{}{}

	return nil
}

// OnStop implements cmn.Service
func (sl *SignerListenerEndpoint) OnStop() {
	sl.instanceMtx.Lock()
	defer sl.instanceMtx.Unlock()
	_ = sl.Close()

	// Stop listening
	if sl.listener != nil {
		if err := sl.listener.Close(); err != nil {
			sl.Logger.Error("Closing Listener", "err", err)
			sl.listener = nil
		}
	}

	sl.pingTimer.Stop()
}

// WaitForConnection waits maxWait for a connection or returns a timeout error
func (sl *SignerListenerEndpoint) WaitForConnection(maxWait time.Duration) error {
	sl.instanceMtx.Lock()
	defer sl.instanceMtx.Unlock()
	return sl.ensureConnection(maxWait)
}

// SendRequest ensures there is a connection, sends a request and waits for a response
func (sl *SignerListenerEndpoint) SendRequest(request SignerMessage) (SignerMessage, error) {
	sl.instanceMtx.Lock()
	defer sl.instanceMtx.Unlock()

	err := sl.ensureConnection(sl.timeoutAccept)
	if err != nil {
		return nil, err
	}

	err = sl.WriteMessage(request)
	if err != nil {
		return nil, err
	}

	res, err := sl.ReadMessage()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (sl *SignerListenerEndpoint) ensureConnection(maxWait time.Duration) error {
	if sl.IsConnected() {
		return nil
	}

	// Is there a connection ready? then use it
	if sl.GetAvailableConnection(sl.connectionAvailableCh) {
		return nil
	}

	// block until connected or timeout
	sl.triggerConnect()
	err := sl.WaitConnection(sl.connectionAvailableCh, maxWait)
	if err != nil {
		return err
	}

	return nil
}

func (sl *SignerListenerEndpoint) acceptNewConnection() (net.Conn, error) {
	if !sl.IsRunning() || sl.listener == nil {
		return nil, fmt.Errorf("endpoint is closing")
	}

	// wait for a new conn
	sl.Logger.Info("SignerListener: Listening for new connection")
	conn, err := sl.listener.Accept()
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (sl *SignerListenerEndpoint) triggerConnect() {
	select {
	case sl.connectRequestCh <- struct{}{}:
	default:
	}
}

func (sl *SignerListenerEndpoint) triggerReconnect() {
	sl.DropConnection()
	sl.triggerConnect()
}

func (sl *SignerListenerEndpoint) serviceLoop() {
	for {
		select {
		case <-sl.connectRequestCh:
			{
				conn, err := sl.acceptNewConnection()
				if err == nil {
					sl.Logger.Info("SignerListener: Connected")

					// We have a good connection, wait for someone that needs one otherwise cancellation
					select {
					case sl.connectionAvailableCh <- conn:
					case <-sl.Quit():
						return
					}
				}

				select {
				case sl.connectRequestCh <- struct{}{}:
				default:
				}
			}
		case <-sl.Quit():
			return
		}
	}
}

func (sl *SignerListenerEndpoint) pingLoop() {
	for {
		select {
		case <-sl.pingTimer.C:
			{
				_, err := sl.SendRequest(&PingRequest{})
				if err != nil {
					sl.Logger.Error("SignerListener: Ping timeout")
					sl.triggerReconnect()
				}
			}
		case <-sl.Quit():
			return
		}
	}
}
