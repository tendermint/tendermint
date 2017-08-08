package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tmlibs/log"
)

const (
	sendTimeout = 10 * time.Second
	// see https://github.com/tendermint/go-rpc/blob/develop/server/handlers.go#L313
	pingPeriod = (30 * 9 / 10) * time.Second
)

type transacter struct {
	Target      string
	Rate        int
	Connections int

	conns   []*websocket.Conn
	wg      sync.WaitGroup
	stopped bool

	logger log.Logger
}

func newTransacter(target string, connections int, rate int) *transacter {
	return &transacter{
		Target:      target,
		Rate:        rate,
		Connections: connections,
		conns:       make([]*websocket.Conn, connections),
		logger:      log.NewNopLogger(),
	}
}

// SetLogger lets you set your own logger
func (t *transacter) SetLogger(l log.Logger) {
	t.logger = l
}

// Start opens N = `t.Connections` connections to the target and creates read
// and write goroutines for each connection.
func (t *transacter) Start() error {
	t.stopped = false

	for i := 0; i < t.Connections; i++ {
		c, _, err := connect(t.Target)
		if err != nil {
			return err
		}
		t.conns[i] = c
	}

	t.wg.Add(2 * t.Connections)
	for i := 0; i < t.Connections; i++ {
		go t.sendLoop(i)
		go t.receiveLoop(i)
	}

	return nil
}

// Stop closes the connections.
func (t *transacter) Stop() {
	t.stopped = true
	t.wg.Wait()
	for _, c := range t.conns {
		c.Close()
	}
}

// receiveLoop reads messages from the connection (empty in case of
// `broadcast_tx_async`).
func (t *transacter) receiveLoop(connIndex int) {
	c := t.conns[connIndex]
	defer t.wg.Done()
	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				t.logger.Error("failed to read response", "err", err)
			}
			return
		}
		if t.stopped {
			return
		}
	}
}

// sendLoop generates transactions at a given rate.
func (t *transacter) sendLoop(connIndex int) {
	c := t.conns[connIndex]

	c.SetPingHandler(func(message string) error {
		err := c.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(sendTimeout))
		if err == websocket.ErrCloseSent {
			return nil
		} else if e, ok := err.(net.Error); ok && e.Temporary() {
			return nil
		}
		return err
	})

	logger := t.logger.With("addr", c.RemoteAddr())

	var txNumber = 0

	pingsTicker := time.NewTicker(pingPeriod)
	txsTicker := time.NewTicker(1 * time.Second)
	defer func() {
		pingsTicker.Stop()
		txsTicker.Stop()
		t.wg.Done()
	}()

	for {
		select {
		case <-txsTicker.C:
			startTime := time.Now()

			for i := 0; i < t.Rate; i++ {
				// each transaction embeds connection index and tx number
				tx := generateTx(connIndex, txNumber)
				paramsJson, err := json.Marshal(map[string]interface{}{"tx": hex.EncodeToString(tx)})
				if err != nil {
					fmt.Printf("failed to encode params: %v\n", err)
					os.Exit(1)
				}
				rawParamsJson := json.RawMessage(paramsJson)

				c.SetWriteDeadline(time.Now().Add(sendTimeout))
				err = c.WriteJSON(rpctypes.RPCRequest{
					JSONRPC: "2.0",
					ID:      "",
					Method:  "broadcast_tx_async",
					Params:  &rawParamsJson,
				})
				if err != nil {
					fmt.Printf("%v. Try increasing the connections count and reducing the rate.\n", errors.Wrap(err, "txs send failed"))
					os.Exit(1)
				}

				txNumber++
			}

			timeToSend := time.Now().Sub(startTime)
			time.Sleep(time.Second - timeToSend)
			logger.Info(fmt.Sprintf("sent %d transactions", t.Rate), "took", timeToSend)
		case <-pingsTicker.C:
			// Right now go-rpc server closes the connection in the absence of pings
			c.SetWriteDeadline(time.Now().Add(sendTimeout))
			if err := c.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				logger.Error("failed to write ping message", "err", err)
			}
		}

		if t.stopped {
			// To cleanly close a connection, a client should send a close
			// frame and wait for the server to close the connection.
			c.SetWriteDeadline(time.Now().Add(sendTimeout))
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Error("failed to write close message", "err", err)
			}

			return
		}
	}
}

func connect(host string) (*websocket.Conn, *http.Response, error) {
	u := url.URL{Scheme: "ws", Host: host, Path: "/websocket"}
	return websocket.DefaultDialer.Dial(u.String(), nil)
}

func generateTx(a int, b int) []byte {
	tx := make([]byte, 250)
	binary.PutUvarint(tx[:32], uint64(a))
	binary.PutUvarint(tx[32:64], uint64(b))
	if _, err := rand.Read(tx[234:]); err != nil {
		panic(errors.Wrap(err, "failed to generate transaction"))
	}
	return tx
}
