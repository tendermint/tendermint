package core_client

import (
	"github.com/gorilla/websocket"
	"github.com/tendermint/tendermint/rpc"
	"net/http"
)

// A websocket client subscribes and unsubscribes to events
type WSClient struct {
	host string
	conn *websocket.Conn
}

// create a new connection
func NewWSClient(addr string) *WSClient {
	return &WSClient{
		host: addr,
	}
}

func (wsc *WSClient) Dial() error {
	dialer := websocket.DefaultDialer
	rHeader := http.Header{}
	conn, _, err := dialer.Dial(wsc.host, rHeader)
	if err != nil {
		return err
	}
	wsc.conn = conn
	return nil
}

// subscribe to an event
func (wsc *WSClient) Subscribe(eventid string) error {
	return wsc.conn.WriteJSON(rpc.WSRequest{
		Type:  "subscribe",
		Event: eventid,
	})
}

// unsubscribe from an event
func (wsc *WSClient) Unsubscribe(eventid string) error {
	return wsc.conn.WriteJSON(rpc.WSRequest{
		Type:  "unsubscribe",
		Event: eventid,
	})
}
