package rpcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	metrics "github.com/rcrowley/go-metrics"

	types "github.com/tendermint/tendermint/rpc/lib/types"
	cmn "github.com/tendermint/tmlibs/common"
)

const (
	// Time allowed to write a message to the server.
	writeWait = 10 * time.Second

	// Maximum reconnect attempts
	maxReconnectAttempts = 25

	defaultPongWait   = 30 * time.Second
	defaultPingPeriod = (defaultPongWait * 9) / 10
)

// WSClient is a WebSocket client. The methods of WSClient are safe for use by
// multiple goroutines.
type WSClient struct {
	cmn.BaseService

	conn *websocket.Conn

	Address  string // IP:PORT or /path/to/socket
	Endpoint string // /websocket/url/endpoint
	Dialer   func(string, string) (net.Conn, error)

	// Time between sending a ping and receiving a pong. See
	// https://godoc.org/github.com/rcrowley/go-metrics#Timer.
	PingPongLatencyTimer metrics.Timer

	// user facing channels, closed only when the client is being stopped.
	ResultsCh chan json.RawMessage
	ErrorsCh  chan error

	// internal channels
	send            chan types.RPCRequest // user requests
	backlog         chan types.RPCRequest // stores a single user request received during a conn failure
	reconnectAfter  chan error            // reconnect requests
	readRoutineQuit chan struct{}         // a way for readRoutine to close writeRoutine

	wg sync.WaitGroup

	mtx            sync.RWMutex
	sentLastPingAt time.Time
	reconnecting   bool

	// Time allowed to read the next pong message from the server.
	pongWait time.Duration

	// Send pings to server with this period. Must be less than pongWait.
	pingPeriod time.Duration
}

// NewWSClient returns a new client. See the commentary on the func(*WSClient)
// functions for a detailed description of how to configure ping period and
// pong wait time.
func NewWSClient(remoteAddr, endpoint string, options ...func(*WSClient)) *WSClient {
	addr, dialer := makeHTTPDialer(remoteAddr)
	c := &WSClient{
		Address:              addr,
		Dialer:               dialer,
		Endpoint:             endpoint,
		PingPongLatencyTimer: metrics.NewTimer(),
		pongWait:             defaultPongWait,
		pingPeriod:           defaultPingPeriod,
	}
	c.BaseService = *cmn.NewBaseService(nil, "WSClient", c)
	for _, option := range options {
		option(c)
	}
	return c
}

// PingPong allows changing ping period and pong wait time. If ping period
// greater or equal to pong wait time, panic will be thrown.
func PingPong(pingPeriod, pongWait time.Duration) func(*WSClient) {
	return func(c *WSClient) {
		if pingPeriod >= pongWait {
			panic(fmt.Sprintf("ping period (%v) must be less than pong wait time (%v)", pingPeriod, pongWait))
		}
		c.pingPeriod = pingPeriod
		c.pongWait = pongWait
	}
}

// String returns WS client full address.
func (c *WSClient) String() string {
	return fmt.Sprintf("%s (%s)", c.Address, c.Endpoint)
}

// OnStart implements cmn.Service by dialing a server and creating read and
// write routines.
func (c *WSClient) OnStart() error {
	err := c.dial()
	if err != nil {
		return err
	}

	c.ResultsCh = make(chan json.RawMessage)
	c.ErrorsCh = make(chan error)

	c.send = make(chan types.RPCRequest)
	// 1 additional error may come from the read/write
	// goroutine depending on which failed first.
	c.reconnectAfter = make(chan error, 1)
	// capacity for 1 request. a user won't be able to send more because the send
	// channel is unbuffered.
	c.backlog = make(chan types.RPCRequest, 1)

	c.startReadWriteRoutines()
	go c.reconnectRoutine()

	return nil
}

// OnStop implements cmn.Service.
func (c *WSClient) OnStop() {}

// Stop overrides cmn.Service#Stop. There is no other way to wait until Quit
// channel is closed.
func (c *WSClient) Stop() bool {
	success := c.BaseService.Stop()
	// only close user-facing channels when we can't write to them
	c.wg.Wait()
	close(c.ResultsCh)
	close(c.ErrorsCh)
	return success
}

// IsReconnecting returns true if the client is reconnecting right now.
func (c *WSClient) IsReconnecting() bool {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	return c.reconnecting
}

// IsActive returns true if the client is running and not reconnecting.
func (c *WSClient) IsActive() bool {
	return c.IsRunning() && !c.IsReconnecting()
}

// Send the given RPC request to the server. Results will be available on
// ResultsCh, errors, if any, on ErrorsCh. Will block until send succeeds or
// ctx.Done is closed.
func (c *WSClient) Send(ctx context.Context, request types.RPCRequest) error {
	select {
	case c.send <- request:
		c.Logger.Info("sent a request", "req", request)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Call the given method. See Send description.
func (c *WSClient) Call(ctx context.Context, method string, params map[string]interface{}) error {
	request, err := types.MapToRequest("", method, params)
	if err != nil {
		return err
	}
	return c.Send(ctx, request)
}

// CallWithArrayParams the given method with params in a form of array. See
// Send description.
func (c *WSClient) CallWithArrayParams(ctx context.Context, method string, params []interface{}) error {
	request, err := types.ArrayToRequest("", method, params)
	if err != nil {
		return err
	}
	return c.Send(ctx, request)
}

///////////////////////////////////////////////////////////////////////////////
// Private methods

func (c *WSClient) dial() error {
	dialer := &websocket.Dialer{
		NetDial: c.Dialer,
		Proxy:   http.ProxyFromEnvironment,
	}
	rHeader := http.Header{}
	conn, _, err := dialer.Dial("ws://"+c.Address+c.Endpoint, rHeader)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

// reconnect tries to redial up to maxReconnectAttempts with exponential
// backoff.
func (c *WSClient) reconnect() error {
	attempt := 0

	c.mtx.Lock()
	c.reconnecting = true
	c.mtx.Unlock()
	defer func() {
		c.mtx.Lock()
		c.reconnecting = false
		c.mtx.Unlock()
	}()

	for {
		c.Logger.Info("reconnecting", "attempt", attempt+1)

		d := time.Duration(math.Exp2(float64(attempt)))
		time.Sleep(d * time.Second)

		err := c.dial()
		if err != nil {
			c.Logger.Error("failed to redial", "err", err)
		} else {
			c.Logger.Info("reconnected")
			return nil
		}

		attempt++

		if attempt > maxReconnectAttempts {
			return errors.Wrap(err, "reached maximum reconnect attempts")
		}
	}
}

func (c *WSClient) startReadWriteRoutines() {
	c.wg.Add(2)
	c.readRoutineQuit = make(chan struct{})
	go c.readRoutine()
	go c.writeRoutine()
}

func (c *WSClient) reconnectRoutine() {
	for {
		select {
		case originalError := <-c.reconnectAfter:
			// wait until writeRoutine and readRoutine finish
			c.wg.Wait()
			err := c.reconnect()
			if err != nil {
				c.Logger.Error("failed to reconnect", "err", err, "original_err", originalError)
				c.Stop()
				return
			} else {
				// drain reconnectAfter
			LOOP:
				for {
					select {
					case <-c.reconnectAfter:
					default:
						break LOOP
					}
				}
				c.startReadWriteRoutines()
			}
		case <-c.Quit:
			return
		}
	}
}

// The client ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *WSClient) writeRoutine() {
	ticker := time.NewTicker(c.pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		c.wg.Done()
	}()

	for {
		select {
		case request := <-c.backlog:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.conn.WriteJSON(request)
			if err != nil {
				c.Logger.Error("failed to resend request", "err", err)
				c.reconnectAfter <- err
				// add request to the backlog, so we don't lose it
				c.backlog <- request
				return
			}
			c.Logger.Info("resend a request", "req", request)
		case request := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.conn.WriteJSON(request)
			if err != nil {
				c.Logger.Error("failed to send request", "err", err)
				c.reconnectAfter <- err
				// add request to the backlog, so we don't lose it
				c.backlog <- request
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.conn.WriteMessage(websocket.PingMessage, []byte{})
			if err != nil {
				c.Logger.Error("failed to write ping", "err", err)
				c.reconnectAfter <- err
				return
			}
			c.mtx.Lock()
			c.sentLastPingAt = time.Now()
			c.mtx.Unlock()
			c.Logger.Debug("sent ping")
		case <-c.readRoutineQuit:
			return
		case <-c.Quit:
			c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return
		}
	}
}

// The client ensures that there is at most one reader to a connection by
// executing all reads from this goroutine.
func (c *WSClient) readRoutine() {
	defer func() {
		c.conn.Close()
		c.wg.Done()
	}()

	c.conn.SetReadDeadline(time.Now().Add(c.pongWait))

	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
		c.mtx.RLock()
		c.PingPongLatencyTimer.UpdateSince(c.sentLastPingAt)
		c.mtx.RUnlock()
		c.Logger.Debug("got pong")
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return
			}

			c.Logger.Error("failed to read response", "err", err)
			close(c.readRoutineQuit)
			c.reconnectAfter <- err
			return
		}

		var response types.RPCResponse
		err = json.Unmarshal(data, &response)
		if err != nil {
			c.Logger.Error("failed to parse response", "err", err, "data", string(data))
			c.ErrorsCh <- err
			continue
		}
		if response.Error != "" {
			c.ErrorsCh <- errors.Errorf(response.Error)
			continue
		}
		c.Logger.Info("got response", "resp", response.Result)
		c.ResultsCh <- *response.Result
	}
}

///////////////////////////////////////////////////////////////////////////////
// Predefined methods

// Subscribe to an event. Note the server must have a "subscribe" route
// defined.
func (c *WSClient) Subscribe(ctx context.Context, eventType string) error {
	params := map[string]interface{}{"event": eventType}
	return c.Call(ctx, "subscribe", params)
}

// Unsubscribe from an event. Note the server must have a "unsubscribe" route
// defined.
func (c *WSClient) Unsubscribe(ctx context.Context, eventType string) error {
	params := map[string]interface{}{"event": eventType}
	return c.Call(ctx, "unsubscribe", params)
}

// UnsubscribeAll from all. Note the server must have a "unsubscribe_all" route
// defined.
func (c *WSClient) UnsubscribeAll(ctx context.Context) error {
	params := map[string]interface{}{}
	return c.Call(ctx, "unsubscribe_all", params)
}
