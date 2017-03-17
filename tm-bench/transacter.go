package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

	rpcclient "github.com/tendermint/go-rpc/client"
	rpctypes "github.com/tendermint/go-rpc/types"
)

type transacter struct {
	Target      string
	Rate        int
	Connections int

	conns   []*rpcclient.WSClient
	wg      sync.WaitGroup
	stopped bool
}

func newTransacter(target string, connections int, rate int) *transacter {
	conns := make([]*rpcclient.WSClient, connections)
	for i := 0; i < connections; i++ {
		conns[i] = rpcclient.NewWSClient(target, "/websocket")
	}

	return &transacter{
		Target:      target,
		Rate:        rate,
		Connections: connections,
		conns:       conns,
	}
}

func (t *transacter) Start() error {
	t.stopped = false

	for _, c := range t.conns {
		if _, err := c.Start(); err != nil {
			return err
		}
	}

	for i := 0; i < t.Connections; i++ {
		t.wg.Add(1)
		go t.sendLoop(i)
	}

	return nil
}

func (t *transacter) Stop() {
	t.stopped = true
	t.wg.Wait()
	for _, c := range t.conns {
		c.Stop()
	}
}

func (t *transacter) sendLoop(connIndex int) {
	conn := t.conns[connIndex]

	var num = 0
	for {
		startTime := time.Now()

		for i := 0; i < t.Rate; i++ {
			if t.stopped {
				t.wg.Done()
				return
			}

			tx := generateTx(connIndex, num)
			err := conn.WriteJSON(rpctypes.RPCRequest{
				JSONRPC: "2.0",
				ID:      "",
				Method:  "broadcast_tx_async",
				Params:  []interface{}{hex.EncodeToString(tx)},
			})
			if err != nil {
				panic(errors.Wrap(err, fmt.Sprintf("lost connection to %s", conn.Address)))
			}
			num++
		}

		timeToSend := time.Now().Sub(startTime)
		time.Sleep(time.Second - timeToSend)
	}
}

func generateTx(a int, b int) []byte {
	tx := make([]byte, 250)
	binary.PutUvarint(tx[:32], uint64(a))
	binary.PutUvarint(tx[32:64], uint64(b))
	if _, err := rand.Read(tx[234:]); err != nil {
		panic(errors.Wrap(err, "err reading from crypto/rand"))
	}
	return tx
}
