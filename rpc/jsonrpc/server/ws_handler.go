package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gorilla/websocket"

	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// WebSocket handler

const (
	defaultWSWriteChanCapacity = 100
	defaultWSWriteWait         = 10 * time.Second
	defaultWSReadWait          = 30 * time.Second
	defaultWSPingPeriod        = (defaultWSReadWait * 9) / 10
)

// WebsocketManager provides a WS handler for incoming connections and passes a
// map of functions along with any additional params to new connections.
// NOTE: The websocket path is defined externally, e.g. in node/node.go
type WebsocketManager struct {
	websocket.Upgrader

	funcMap       map[string]*RPCFunc
	logger        log.Logger
	wsConnOptions []func(*wsConnection)
}

// NewWebsocketManager returns a new WebsocketManager that passes a map of
// functions, connection options and logger to new WS connections.
func NewWebsocketManager(logger log.Logger, funcMap map[string]*RPCFunc, wsConnOptions ...func(*wsConnection)) *WebsocketManager {
	return &WebsocketManager{
		funcMap: funcMap,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO ???
				//
				// The default behavior would be relevant to browser-based clients,
				// afaik. I suppose having a pass-through is a workaround for allowing
				// for more complex security schemes, shifting the burden of
				// AuthN/AuthZ outside the Tendermint RPC.
				// I can't think of other uses right now that would warrant a TODO
				// though. The real backstory of this TODO shall remain shrouded in
				// mystery
				return true
			},
		},
		logger:        logger,
		wsConnOptions: wsConnOptions,
	}
}

// WebsocketHandler upgrades the request/response (via http.Hijack) and starts
// the wsConnection.
func (wm *WebsocketManager) WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	wsConn, err := wm.Upgrade(w, r, nil)
	if err != nil {
		// The upgrader has already reported an HTTP error to the client, so we
		// need only log it.
		wm.logger.Error("Failed to upgrade connection", "err", err)
		return
	}
	defer func() {
		if err := wsConn.Close(); err != nil {
			wm.logger.Error("Failed to close connection", "err", err)
		}
	}()

	// register connection
	logger := wm.logger.With("remote", wsConn.RemoteAddr())
	conn := newWSConnection(wsConn, wm.funcMap, logger, wm.wsConnOptions...)
	wm.logger.Info("New websocket connection", "remote", conn.remoteAddr)

	// starting the conn is blocking
	if err = conn.Start(r.Context()); err != nil {
		wm.logger.Error("Failed to start connection", "err", err)
		writeInternalError(w, err)
		return
	}

	if err := conn.Stop(); err != nil {
		wm.logger.Error("error while stopping connection", "error", err)
	}
}

// WebSocket connection

// A single websocket connection contains listener id, underlying ws
// connection, and the event switch for subscribing to events.
//
// In case of an error, the connection is stopped.
type wsConnection struct {
	Logger log.Logger

	remoteAddr string
	baseConn   *websocket.Conn
	// writeChan is never closed, to allow WriteRPCResponse() to fail.
	writeChan chan rpctypes.RPCResponse

	// chan, which is closed when/if readRoutine errors
	// used to abort writeRoutine
	readRoutineQuit chan struct{}

	funcMap map[string]*RPCFunc

	// Connection times out if we haven't received *anything* in this long, not even pings.
	readWait time.Duration

	// Send pings to server with this period. Must be less than readWait, but greater than zero.
	pingPeriod time.Duration

	// Maximum message size.
	readLimit int64

	// callback which is called upon disconnect
	onDisconnect func(remoteAddr string)

	ctx    context.Context
	cancel context.CancelFunc
}

// NewWSConnection wraps websocket.Conn.
//
// See the commentary on the func(*wsConnection) functions for a detailed
// description of how to configure ping period and pong wait time. NOTE: if the
// write buffer is full, pongs may be dropped, which may cause clients to
// disconnect. see https://github.com/gorilla/websocket/issues/97
func newWSConnection(baseConn *websocket.Conn, funcMap map[string]*RPCFunc, logger log.Logger, options ...func(*wsConnection)) *wsConnection {
	wsc := &wsConnection{
		Logger:          logger,
		remoteAddr:      baseConn.RemoteAddr().String(),
		baseConn:        baseConn,
		funcMap:         funcMap,
		readWait:        defaultWSReadWait,
		pingPeriod:      defaultWSPingPeriod,
		readRoutineQuit: make(chan struct{}),
	}
	for _, option := range options {
		option(wsc)
	}
	wsc.baseConn.SetReadLimit(wsc.readLimit)
	return wsc
}

// OnDisconnect sets a callback which is used upon disconnect - not
// Goroutine-safe. Nop by default.
func OnDisconnect(onDisconnect func(remoteAddr string)) func(*wsConnection) {
	return func(wsc *wsConnection) {
		wsc.onDisconnect = onDisconnect
	}
}

// ReadWait sets the amount of time to wait before a websocket read times out.
// It should only be used in the constructor - not Goroutine-safe.
func ReadWait(readWait time.Duration) func(*wsConnection) {
	return func(wsc *wsConnection) {
		wsc.readWait = readWait
	}
}

// PingPeriod sets the duration for sending websocket pings.
// It should only be used in the constructor - not Goroutine-safe.
func PingPeriod(pingPeriod time.Duration) func(*wsConnection) {
	return func(wsc *wsConnection) {
		wsc.pingPeriod = pingPeriod
	}
}

// ReadLimit sets the maximum size for reading message.
// It should only be used in the constructor - not Goroutine-safe.
func ReadLimit(readLimit int64) func(*wsConnection) {
	return func(wsc *wsConnection) {
		wsc.readLimit = readLimit
	}
}

// Start starts the client service routines and blocks until there is an error.
func (wsc *wsConnection) Start(ctx context.Context) error {
	wsc.writeChan = make(chan rpctypes.RPCResponse, defaultWSWriteChanCapacity)

	// Read subscriptions/unsubscriptions to events
	go wsc.readRoutine(ctx)
	// Write responses, BLOCKING.
	wsc.writeRoutine(ctx)

	return nil
}

// Stop unsubscribes the remote from all subscriptions.
func (wsc *wsConnection) Stop() error {
	if wsc.onDisconnect != nil {
		wsc.onDisconnect(wsc.remoteAddr)
	}
	if wsc.ctx != nil {
		wsc.cancel()
	}
	return nil
}

// GetRemoteAddr returns the remote address of the underlying connection.
// It implements WSRPCConnection
func (wsc *wsConnection) GetRemoteAddr() string {
	return wsc.remoteAddr
}

// WriteRPCResponse pushes a response to the writeChan, and blocks until it is
// accepted.
// It implements WSRPCConnection. It is Goroutine-safe.
func (wsc *wsConnection) WriteRPCResponse(ctx context.Context, resp rpctypes.RPCResponse) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case wsc.writeChan <- resp:
		return nil
	}
}

// TryWriteRPCResponse attempts to push a response to the writeChan, but does
// not block.
// It implements WSRPCConnection. It is Goroutine-safe
func (wsc *wsConnection) TryWriteRPCResponse(ctx context.Context, resp rpctypes.RPCResponse) bool {
	select {
	case <-ctx.Done():
		return false
	case wsc.writeChan <- resp:
		return true
	default:
		return false
	}
}

// Context returns the connection's context.
// The context is canceled when the client's connection closes.
func (wsc *wsConnection) Context() context.Context {
	if wsc.ctx != nil {
		return wsc.ctx
	}
	wsc.ctx, wsc.cancel = context.WithCancel(context.Background())
	return wsc.ctx
}

// Read from the socket and subscribe to or unsubscribe from events
func (wsc *wsConnection) readRoutine(ctx context.Context) {
	// readRoutine will block until response is written or WS connection is closed
	writeCtx := context.Background()

	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("WSJSONRPC: %v", r)
			}
			req := rpctypes.NewRequest(uriReqID)
			wsc.Logger.Error("Panic in WSJSONRPC handler", "err", err, "stack", string(debug.Stack()))
			if err := wsc.WriteRPCResponse(writeCtx,
				req.MakeErrorf(rpctypes.CodeInternalError, "Panic in handler: %v", err)); err != nil {
				wsc.Logger.Error("error writing RPC response", "err", err)
			}
			go wsc.readRoutine(ctx)
		}
	}()

	wsc.baseConn.SetPongHandler(func(m string) error {
		return wsc.baseConn.SetReadDeadline(time.Now().Add(wsc.readWait))
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// reset deadline for every type of message (control or data)
			if err := wsc.baseConn.SetReadDeadline(time.Now().Add(wsc.readWait)); err != nil {
				wsc.Logger.Error("failed to set read deadline", "err", err)
			}

			_, r, err := wsc.baseConn.NextReader()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					wsc.Logger.Info("Client closed the connection")
				} else {
					wsc.Logger.Error("Failed to read request", "err", err)
				}
				if err := wsc.Stop(); err != nil {
					wsc.Logger.Error("error closing websocket connection", "err", err)
				}
				close(wsc.readRoutineQuit)
				return
			}

			dec := json.NewDecoder(r)
			var request rpctypes.RPCRequest
			err = dec.Decode(&request)
			if err != nil {
				if err := wsc.WriteRPCResponse(writeCtx,
					request.MakeErrorf(rpctypes.CodeParseError, "unmarshaling request: %v", err)); err != nil {
					wsc.Logger.Error("error writing RPC response", "err", err)
				}
				continue
			}

			// A Notification is a Request object without an "id" member.
			// The Server MUST NOT reply to a Notification, including those that are within a batch request.
			if request.IsNotification() {
				wsc.Logger.Debug(
					"WSJSONRPC received a notification, skipping... (please send a non-empty ID if you want to call a method)",
					"req", request,
				)
				continue
			}

			// Now, fetch the RPCFunc and execute it.
			rpcFunc := wsc.funcMap[request.Method]
			if rpcFunc == nil {
				if err := wsc.WriteRPCResponse(writeCtx,
					request.MakeErrorf(rpctypes.CodeMethodNotFound, request.Method)); err != nil {
					wsc.Logger.Error("error writing RPC response", "err", err)
				}
				continue
			}

			fctx := rpctypes.WithCallInfo(wsc.Context(), &rpctypes.CallInfo{
				RPCRequest: &request,
				WSConn:     wsc,
			})
			var resp rpctypes.RPCResponse
			result, err := rpcFunc.Call(fctx, request.Params)
			if err == nil {
				resp = request.MakeResponse(result)
			} else {
				resp = request.MakeError(err)
			}
			if err := wsc.WriteRPCResponse(writeCtx, resp); err != nil {
				wsc.Logger.Error("error writing RPC response", "err", err)
			}
		}
	}
}

// receives on a write channel and writes out on the socket
func (wsc *wsConnection) writeRoutine(ctx context.Context) {
	pingTicker := time.NewTicker(wsc.pingPeriod)
	defer pingTicker.Stop()

	// https://github.com/gorilla/websocket/issues/97
	pongs := make(chan string, 1)
	wsc.baseConn.SetPingHandler(func(m string) error {
		select {
		case pongs <- m:
		default:
		}
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		case <-wsc.readRoutineQuit: // error in readRoutine
			return
		case m := <-pongs:
			err := wsc.writeMessageWithDeadline(websocket.PongMessage, []byte(m))
			if err != nil {
				wsc.Logger.Info("Failed to write pong (client may disconnect)", "err", err)
			}
		case <-pingTicker.C:
			err := wsc.writeMessageWithDeadline(websocket.PingMessage, []byte{})
			if err != nil {
				wsc.Logger.Error("Failed to write ping", "err", err)
				return
			}
		case msg := <-wsc.writeChan:
			data, err := json.Marshal(msg)
			if err != nil {
				wsc.Logger.Error("Failed to marshal RPCResponse to JSON", "msg", msg, "err", err)
				continue
			}
			if err = wsc.writeMessageWithDeadline(websocket.TextMessage, data); err != nil {
				wsc.Logger.Error("Failed to write response", "msg", msg, "err", err)
				return
			}
		}
	}
}

// All writes to the websocket must (re)set the write deadline.
// If some writes don't set it while others do, they may timeout incorrectly
// (https://github.com/tendermint/tendermint/issues/553)
func (wsc *wsConnection) writeMessageWithDeadline(msgType int, msg []byte) error {
	if err := wsc.baseConn.SetWriteDeadline(time.Now().Add(defaultWSWriteWait)); err != nil {
		return err
	}
	return wsc.baseConn.WriteMessage(msgType, msg)
}
