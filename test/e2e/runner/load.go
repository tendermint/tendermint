package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/tendermint/tendermint/libs/log"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/test/loadtime/payload"
	"github.com/tendermint/tendermint/types"
)

// Load generates transactions against the network until the given context is
// canceled.
func Load(ctx context.Context, testnet *e2e.Testnet) error {
	initialTimeout := 1 * time.Minute
	stallTimeout := 30 * time.Second
	chSuccess := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	started := time.Now()
	u := [16]byte(uuid.New()) // generate run ID on startup

	txCh := make(chan types.Tx)
	go loadGenerate(ctx, txCh, testnet, u[:])

	for _, n := range testnet.Nodes {
		if n.SendNoLoad {
			continue
		}

		cli, err := n.Client()
		if err != nil {
			return err
		}

		for w := 0; w < testnet.LoadTxConnections; w++ {
			go loadProcess(ctx, txCh, chSuccess, cli)
		}
	}

	// Monitor successful transactions, and abort on stalls.
	success := 0
	timeout := initialTimeout
	for {
		select {
		case <-chSuccess:
			success++
			timeout = stallTimeout
		case <-time.After(timeout):
			return fmt.Errorf("unable to submit transactions for %v", timeout)
		case <-ctx.Done():
			if success == 0 {
				return errors.New("failed to submit any transactions")
			}
			logger.Info("load", "msg", log.NewLazySprintf("Ending transaction load after %v txs (%.1f tx/s)...",
				success, float64(success)/time.Since(started).Seconds()))
			return nil
		}
	}
}

// loadGenerate generates jobs until the context is canceled
func loadGenerate(ctx context.Context, txCh chan<- types.Tx, testnet *e2e.Testnet, id []byte) {
	for i := 0; i < math.MaxInt64; i++ {
		// We keep generating the same 1000 keys over and over, with different values.
		// This gives a reasonable load without putting too much data in the app.
		tx, err := payload.NewBytes(&payload.Payload{
			Id:          id,
			Size:        uint64(testnet.LoadTxSize),
			Rate:        uint64(testnet.LoadTxBatchSize),
			Connections: uint64(testnet.LoadTxConnections),
		})
		if err != nil {
			panic(fmt.Sprintf("Failed to generate tx: %v", err))
		}

		select {
		case txCh <- tx:
			time.Sleep(time.Second)

		case <-ctx.Done():
			close(txCh)
			return
		}
	}
}

// loadProcess processes transactions
func loadProcess(ctx context.Context, txCh <-chan types.Tx, chSuccess chan<- struct{}, client *rpchttp.HTTP) {
	var err error
	s := struct{}{}
	for tx := range txCh {
		if _, err = client.BroadcastTxSync(ctx, tx); err != nil {
			continue
		}
		chSuccess <- s
	}
}
