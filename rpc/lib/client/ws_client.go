package rpcclient

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	types "github.com/tendermint/tendermint/rpc/lib/types"
	cmn "github.com/tendermint/tmlibs/common"
)

const (
	wsResultsChannelCapacity = 10
	wsErrorsChannelCapacity  = 1
	wsWriteTimeoutSeconds    = 10
)

type WSClient struct {
	cmn.BaseService
	Address  string // IP:PORT or /path/to/socket
	Endpoint string // /websocket/url/endpoint
	Dialer   func(string, string) (net.Conn, error)
	*websocket.Conn
	ResultsCh chan json.RawMessage // closes upon WSClient.Stop()
	ErrorsCh  chan error           // closes upon WSClient.Stop()
}

// create a new connection
func NewWSClient(remoteAddr, endpoint string) *WSClient {
	addr, dialer := makeHTTPDialer(remoteAddr)
	wsClient := &WSClient{
		Address:  addr,
		Dialer:   dialer,
		Endpoint: endpoint,
		Conn:     nil,
	}
	wsClient.BaseService = *cmn.NewBaseService(nil, "WSClient", wsClient)
	return wsClient
}

func (wsc *WSClient) String() string {
	return wsc.Address + ", " + wsc.Endpoint
}

// OnStart implements cmn.BaseService interface
func (wsc *WSClient) OnStart() error {
	wsc.BaseService.OnStart()
	err := wsc.dial()
	if err != nil {
		return err
	}
	wsc.ResultsCh = make(chan json.RawMessage, wsResultsChannelCapacity)
	wsc.ErrorsCh = make(chan error, wsErrorsChannelCapacity)
	go wsc.receiveEventsRoutine()
	return nil
}

// OnReset implements cmn.BaseService interface
func (wsc *WSClient) OnReset() error {
	return nil
}

func (wsc *WSClient) dial() error {

	// Dial
	dialer := &websocket.Dialer{
		NetDial: wsc.Dialer,
		Proxy:   http.ProxyFromEnvironment,
	}
	rHeader := http.Header{}
	con, _, err := dialer.Dial("ws://"+wsc.Address+wsc.Endpoint, rHeader)
	if err != nil {
		return err
	}
	// Set the ping/pong handlers
	con.SetPingHandler(func(m string) error {
		// NOTE: https://github.com/gorilla/websocket/issues/97
		go con.WriteControl(websocket.PongMessage, []byte(m), time.Now().Add(time.Second*wsWriteTimeoutSeconds))
		return nil
	})
	con.SetPongHandler(func(m string) error {
		// NOTE: https://github.com/gorilla/websocket/issues/97
		return nil
	})
	wsc.Conn = con
	return nil
}

// OnStop implements cmn.BaseService interface
func (wsc *WSClient) OnStop() {
	wsc.BaseService.OnStop()
	wsc.Conn.Close()
	// ResultsCh/ErrorsCh is closed in receiveEventsRoutine.
}

func (wsc *WSClient) receiveEventsRoutine() {
	for {
		_, data, err := wsc.ReadMessage()
		if err != nil {
			wsc.Logger.Info("WSClient failed to read message", "error", err, "data", string(data))
			wsc.Stop()
			break
		} else {
			var response types.RPCResponse
			err := json.Unmarshal(data, &response)
			if err != nil {
				wsc.Logger.Info("WSClient failed to parse message", "error", err, "data", string(data))
				wsc.ErrorsCh <- err
				continue
			}
			if response.Error != "" {
				wsc.ErrorsCh <- errors.Errorf(response.Error)
				continue
			}
			wsc.ResultsCh <- *response.Result
		}
	}
	// this must be modified in the same go-routine that reads from the
	// connection to avoid race conditions
	wsc.Conn = nil

	// Cleanup
	close(wsc.ResultsCh)
	close(wsc.ErrorsCh)
}

// Subscribe to an event. Note the server must have a "subscribe" route
// defined.
func (wsc *WSClient) Subscribe(eventid string) error {
	params := map[string]interface{}{"event": eventid}
	request, err := types.MapToRequest("", "subscribe", params)
	if err == nil {
		err = wsc.WriteJSON(request)
	}
	return err
}

// Unsubscribe from an event. Note the server must have a "unsubscribe" route
// defined.
func (wsc *WSClient) Unsubscribe(eventid string) error {
	params := map[string]interface{}{"event": eventid}
	request, err := types.MapToRequest("", "unsubscribe", params)
	if err == nil {
		err = wsc.WriteJSON(request)
	}
	return err
}

// Call asynchronously calls a given method by sending an RPCRequest to the
// server. Results will be available on ResultsCh, errors, if any, on ErrorsCh.
func (wsc *WSClient) Call(method string, params map[string]interface{}) error {
	request, err := types.MapToRequest("", method, params)
	if err == nil {
		err = wsc.WriteJSON(request)
	}
	return err
}
