package core_client

import (
	"github.com/gorilla/websocket"
	rpctypes "github.com/tendermint/tendermint/rpc/types"
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

func (wsc *WSClient) Dial() (*http.Response, error) {
	dialer := websocket.DefaultDialer
	rHeader := http.Header{}
	conn, r, err := dialer.Dial(wsc.host, rHeader)
	if err != nil {
		return r, err
	}
	wsc.conn = conn
	return r, nil
}

// subscribe to an event
func (wsc *WSClient) Subscribe(eventid string) error {
	return wsc.conn.WriteJSON(rpctypes.WSRequest{
		Type:  "subscribe",
		Event: eventid,
	})
}

// unsubscribe from an event
func (wsc *WSClient) Unsubscribe(eventid string) error {
	return wsc.conn.WriteJSON(rpctypes.WSRequest{
		Type:  "unsubscribe",
		Event: eventid,
	})
}

type WSMsg struct {
	Data  []byte
	Error error
}

// returns a channel from which messages can be pulled
// from a go routine that reads the socket.
// if the ws returns an error (eg. closes), we return
func (wsc *WSClient) Read() chan *WSMsg {
	ch := make(chan *WSMsg)
	go func() {
		for {
			_, p, err := wsc.conn.ReadMessage()
			ch <- &WSMsg{p, err}
			if err != nil {
				return
			}
		}
	}()
	return ch
}
