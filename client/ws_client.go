package rpcclient

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-rpc/types"
)

const (
	wsResultsChannelCapacity = 10
	wsErrorsChannelCapacity  = 1
	wsWriteTimeoutSeconds    = 10
)

type WSClient struct {
	BaseService
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
		Address:   addr,
		Dialer:    dialer,
		Endpoint:  endpoint,
		Conn:      nil,
		ResultsCh: make(chan json.RawMessage, wsResultsChannelCapacity),
		ErrorsCh:  make(chan error, wsErrorsChannelCapacity),
	}
	wsClient.BaseService = *NewBaseService(log, "WSClient", wsClient)
	return wsClient
}

func (wsc *WSClient) String() string {
	return wsc.Address + ", " + wsc.Endpoint
}

func (wsc *WSClient) OnStart() error {
	wsc.BaseService.OnStart()
	err := wsc.dial()
	if err != nil {
		return err
	}
	go wsc.receiveEventsRoutine()
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

func (wsc *WSClient) OnStop() {
	wsc.BaseService.OnStop()
	// ResultsCh/ErrorsCh is closed in receiveEventsRoutine.
}

func (wsc *WSClient) receiveEventsRoutine() {
	for {
		_, data, err := wsc.ReadMessage()
		if err != nil {
			log.Info("WSClient failed to read message", "error", err, "data", string(data))
			wsc.Stop()
			break
		} else {
			var response rpctypes.RPCResponse
			err := json.Unmarshal(data, &response)
			if err != nil {
				log.Info("WSClient failed to parse message", "error", err, "data", string(data))
				wsc.ErrorsCh <- err
				continue
			}
			if response.Error != "" {
				wsc.ErrorsCh <- fmt.Errorf(response.Error)
				continue
			}
			wsc.ResultsCh <- *response.Result
		}
	}

	// Cleanup
	close(wsc.ResultsCh)
	close(wsc.ErrorsCh)
}

// subscribe to an event
func (wsc *WSClient) Subscribe(eventid string) error {
	err := wsc.WriteJSON(rpctypes.RPCRequest{
		JSONRPC: "2.0",
		ID:      "",
		Method:  "subscribe",
		Params:  []interface{}{eventid},
	})
	return err
}

// unsubscribe from an event
func (wsc *WSClient) Unsubscribe(eventid string) error {
	err := wsc.WriteJSON(rpctypes.RPCRequest{
		JSONRPC: "2.0",
		ID:      "",
		Method:  "unsubscribe",
		Params:  []interface{}{eventid},
	})
	return err
}
