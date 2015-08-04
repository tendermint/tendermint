package rpcclient

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/gorilla/websocket"
	. "github.com/tendermint/tendermint/common"
	_ "github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/rpc/types"
	"github.com/tendermint/tendermint/wire"
)

const wsEventsChannelCapacity = 10
const wsResponsesChannelCapacity = 10

type WSClient struct {
	QuitService
	*websocket.Conn
	EventsCh    chan rpctypes.RPCEventResult
	ResponsesCh chan rpctypes.RPCResponse
}

// create a new connection
func NewWSClient(addr string) (*WSClient, error) {
	dialer := websocket.DefaultDialer
	rHeader := http.Header{}
	con, _, err := dialer.Dial(addr, rHeader)
	if err != nil {
		return nil, err
	}
	wsClient := &WSClient{
		Conn:        con,
		EventsCh:    make(chan rpctypes.RPCEventResult, wsEventsChannelCapacity),
		ResponsesCh: make(chan rpctypes.RPCResponse, wsResponsesChannelCapacity),
	}
	wsClient.QuitService = *NewQuitService(log, "WSClient", wsClient)
	return wsClient, nil
}

func (wsc *WSClient) OnStart() {
	wsc.QuitService.OnStart()
	go wsc.receiveEventsRoutine()
}

func (wsc *WSClient) OnStop() {
	wsc.QuitService.OnStop()
}

func (wsc *WSClient) receiveEventsRoutine() {
	for {
		_, data, err := wsc.ReadMessage()
		if err != nil {
			log.Info("WSClient failed to read message: %v", err)
			wsc.Stop()
			break
		} else {
			var response rpctypes.RPCResponse
			if err := json.Unmarshal(data, &response); err != nil {
				log.Info("WSClient failed to parse message: %v", err)
				wsc.Stop()
				break
			}
			if strings.HasSuffix(response.Id, "#event") {
				var eventResult rpctypes.RPCEventResult
				var err error
				wire.ReadJSONObject(&eventResult, response.Result, &err)
				if err != nil {
					log.Info("WSClient failed to parse RPCEventResult: %v", err)
					wsc.Stop()
					break
				}
				wsc.EventsCh <- eventResult
			} else {
				wsc.ResponsesCh <- response
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
