package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tendermint/tendermint/libs/log"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/test/loadtime/payload"
	"github.com/tendermint/tendermint/types"
)

const workerPoolSize = 16

// Load generates transactions against the network until the given context is
// canceled.
func Load(ctx context.Context, testnet *e2e.Testnet) error {
	initialTimeout := 1 * time.Minute
	stallTimeout := 30 * time.Second
	chSuccess := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger.Info("load", "msg", log.NewLazySprintf("Starting transaction load (%v workers)...", workerPoolSize))
	started := time.Now()
	u := [16]byte(uuid.New()) // generate run ID on startup

	txCh := make(chan types.Tx)
	go loadGenerate(ctx, txCh, testnet, u[:])

	for _, n := range testnet.Nodes {
		if n.SendNoLoad {
			continue
		}

		for w := 0; w < testnet.LoadTxConnections; w++ {
			go loadProcess(ctx, txCh, chSuccess, n)
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
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-t.C:
		case <-ctx.Done():
			close(txCh)
			return
		}
		t.Reset(time.Second)

		// A context with a timeout is created here to time the createTxBatch
		// function out. If createTxBatch has not completed its work by the time
		// the next batch is set to be sent out, then the context is cancled so that
		// the current batch is halted, allowing the next batch to begin.
		tctx, cf := context.WithTimeout(ctx, time.Second)
		createTxBatch(tctx, txCh, testnet, id)
		cf()
	}
}

// createTxBatch creates new transactions and sends them into the txCh. createTxBatch
// returns when either a full batch has been sent to the txCh or the context
// is canceled.
func createTxBatch(ctx context.Context, txCh chan<- types.Tx, testnet *e2e.Testnet, id []byte) {
	wg := &sync.WaitGroup{}
	genCh := make(chan struct{})
	for i := 0; i < workerPoolSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range genCh {
				tx, err := payload.NewBytes(&payload.Payload{
					Id:          id,
					Size:        uint64(testnet.LoadTxSizeBytes),
					Rate:        uint64(testnet.LoadTxBatchSize),
					Connections: uint64(testnet.LoadTxConnections),
				})
				if err != nil {
					panic(fmt.Sprintf("Failed to generate tx: %v", err))
				}

				select {
				case txCh <- tx:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	for i := 0; i < testnet.LoadTxBatchSize; i++ {
		select {
		case genCh <- struct{}{}:
		case <-ctx.Done():
			break
		}
	}
	close(genCh)
	wg.Wait()
}

// loadProcess processes transactions by sending transactions received on the txCh
// to the client.
func loadProcess(ctx context.Context, txCh <-chan types.Tx, chSuccess chan<- struct{}, n *e2e.Node) {
	var client *rpchttp.HTTP
	var err error
	s := struct{}{}
	for tx := range txCh {
		if client == nil {
			client, err = n.Client()
			if err != nil {
				logger.Info("non-fatal error creating node client", "error", err)
				continue
			}
		}
		if _, err = client.BroadcastTxSync(ctx, tx); err != nil {
			continue
		}
		chSuccess <- s
	}
}
