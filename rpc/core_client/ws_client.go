package core_client

import (
	"net/http"
	"strings"
	"time"

	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/gorilla/websocket"
	. "github.com/tendermint/tendermint/common"
	_ "github.com/tendermint/tendermint/config/tendermint_test"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/rpc/types"
	"github.com/tendermint/tendermint/wire"
)

const (
	wsEventsChannelCapacity  = 10
	wsResultsChannelCapacity = 10
	wsWriteTimeoutSeconds    = 10
)

type WSClient struct {
	QuitService
	Address string
	*websocket.Conn
	EventsCh  chan ctypes.ResultEvent
	ResultsCh chan ctypes.Result
}

// create a new connection
func NewWSClient(addr string) *WSClient {
	wsClient := &WSClient{
		Address:   addr,
		Conn:      nil,
		EventsCh:  make(chan ctypes.ResultEvent, wsEventsChannelCapacity),
		ResultsCh: make(chan ctypes.Result, wsResultsChannelCapacity),
	}
	wsClient.QuitService = *NewQuitService(log, "WSClient", wsClient)
	return wsClient
}

func (wsc *WSClient) OnStart() error {
	wsc.QuitService.OnStart()
	err := wsc.dial()
	if err != nil {
		return err
	}
	go wsc.receiveEventsRoutine()
	return nil
}

func (wsc *WSClient) dial() error {
	// Dial
	dialer := websocket.DefaultDialer
	rHeader := http.Header{}
	con, _, err := dialer.Dial(wsc.Address, rHeader)
	if err != nil {
		return err
	}
	// Set the ping/pong handlers
	con.SetPingHandler(func(m string) error {
		con.WriteControl(websocket.PongMessage, []byte(m), time.Now().Add(time.Second*wsWriteTimeoutSeconds))
		return nil
	})
	con.SetPongHandler(func(m string) error {
		return nil
	})
	wsc.Conn = con
	return nil
}

func (wsc *WSClient) OnStop() {
	wsc.QuitService.OnStop()
}

func (wsc *WSClient) receiveEventsRoutine() {
	for {
		_, data, err := wsc.ReadMessage()
		if err != nil {
			log.Info("WSClient failed to read message", "error", err, "data", string(data))
			wsc.Stop()
			break
		} else {
			var response ctypes.Response
			wire.ReadJSON(&response, data, &err)
			if err != nil {
				log.Info("WSClient failed to parse message", "error", err)
				wsc.Stop()
				break
			}
			if strings.HasSuffix(response.Id, "#event") {
				wsc.EventsCh <- *response.Result.(*ctypes.ResultEvent)
			} else {
				wsc.ResultsCh <- response.Result
			}
		}
	}
}

// subscribe to an event
func (wsc *WSClient) Subscribe(eventid string) error {
	err := wsc.WriteJSON(rpctypes.RPCRequest{
		JSONRPC: "2.0",
		Id:      "",
		Method:  "subscribe",
		Params:  []interface{}{eventid},
	})
	return err
}

// unsubscribe from an event
func (wsc *WSClient) Unsubscribe(eventid string) error {
	err := wsc.WriteJSON(rpctypes.RPCRequest{
		JSONRPC: "2.0",
		Id:      "",
		Method:  "unsubscribe",
		Params:  []interface{}{eventid},
	})
	return err
}
