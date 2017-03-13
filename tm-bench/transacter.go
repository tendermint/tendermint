package bench

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
	Target string
	Rate   int

	wsc     *rpcclient.WSClient
	stopped bool
	wg      sync.WaitGroup
}

func (t *transacter) Start() error {
	t.wsc = rpcclient.NewWSClient(t.Target, "/websocket")
	if _, err := t.wsc.Start(); err != nil {
		return err
	}
	wg.Add(1)
	go t.sendLoop()
	return nil
}

func (t *transacter) Stop() {
	t.stopped = true
	wg.Wait()
	t.wsc.Stop()
}

func (t *transacter) sendLoop() {
	var num = 0

	for {
		startTime := time.Now()
		for i := 0; i < t.Rate; i++ {
			tx := generateTx(num)
			err := t.wsc.WriteJSON(rpctypes.RPCRequest{
				JSONRPC: "2.0",
				ID:      "",
				Method:  "broadcast_tx_async",
				Params:  []interface{}{hex.EncodeToString(tx)},
			})
			if err != nil {
				panic(errors.Wrap(err, fmt.Sprintf("lost connection to %s", t.Target)))
			}
			num++
		}

		if t.stopped {
			wg.Done()
			return
		}

		timeToSend := time.Now() - startTime
		timer.Sleep(time.Second - timeToSend)
	}
}

// generateTx returns a random byte sequence where first 8 bytes are the number
// of transaction.
func generateTx(num) []byte {
	tx := make([]byte, 250)
	binary.PutUvarint(tx[:32], uint64(num))
	if _, err := rand.Read(tx[234:]); err != nil {
		panic(errors.Wrap(err, "err reading from crypto/rand"))
	}
	return tx
}
