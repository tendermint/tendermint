package main

import (
	"container/ring"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

// Load generates transactions against the network until the given context is
// canceled.
func Load(ctx context.Context, testnet *e2e.Testnet) error {
	// Since transactions are executed across all nodes in the network, we need
	// to reduce transaction load for larger networks to avoid using too much
	// CPU. This gives high-throughput small networks and low-throughput large ones.
	// This also limits the number of TCP connections, since each worker has
	// a connection to all nodes.
	concurrency := 64 / len(testnet.Nodes)
	if concurrency == 0 {
		concurrency = 1
	}
	initialTimeout := 1 * time.Minute
	stallTimeout := 30 * time.Second

	chTx := make(chan types.Tx, len(testnet.Nodes))
	chSuccess := make(chan types.Tx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Spawn job generator and processors.
	logger.Info(fmt.Sprintf("Starting transaction load (%v workers)...", concurrency))
	started := time.Now()

	go loadGenerate(ctx, chTx, testnet.TxSize)

	for w := 0; w < concurrency; w++ {
		go loadProcess(ctx, testnet, chTx, chSuccess)
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
			logger.Info(fmt.Sprintf("Ending transaction load after %v txs (%.1f tx/s)...",
				success, float64(success)/time.Since(started).Seconds()))
			return nil
		}
	}
}

// loadGenerate generates jobs until the context is canceled.
//
// The chTx has multiple consumers, thus the rate limiting of the load
// generation is primarily the result of backpressure from the
// broadcast transaction, though at most one transaction will be
// produced every 10ms.
func loadGenerate(ctx context.Context, chTx chan<- types.Tx, size int64) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	defer close(chTx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			// We keep generating the same 100 keys over and over, with different values.
			// This gives a reasonable load without putting too much data in the app.
			id := rand.Int63() % 100 // nolint: gosec

			bz := make([]byte, size)
			_, err := rand.Read(bz) // nolint: gosec
			if err != nil {
				panic(fmt.Sprintf("Failed to read random bytes: %v", err))
			}
			tx := types.Tx(fmt.Sprintf("load-%X=%x", id, bz))

			select {
			case <-ctx.Done():
				return
			case chTx <- tx:
				// sleep for 10ms + a jitter value
				// that's at least 10ms and at most
				// 10ms plus 10ms for each
				fuzzFactor := 10 * (1 + time.Duration(rand.Int63n(int64(cap(chTx))))) // nolint: gosec
				timer.Reset(fuzzFactor * time.Millisecond)
				continue
			}

		}
	}
}

// loadProcess processes transactions
func loadProcess(ctx context.Context, testnet *e2e.Testnet, chTx <-chan types.Tx, chSuccess chan<- types.Tx) {
	// Each worker gets its own client to each usable node, which
	// allows for some concurrency while still bounding it.
	clients := make([]*rpchttp.HTTP, 0, len(testnet.Nodes))

	for idx := range testnet.Nodes {
		// Construct a list of usable nodes for the creating
		// load. Don't send load through seed nodes because
		// they do not provide the RPC endpoints required to
		// broadcast transaction.
		if testnet.Nodes[idx].Mode == e2e.ModeSeed {
			continue
		}

		client, err := testnet.Nodes[idx].Client()
		if err != nil {
			continue
		}

		clients = append(clients, client)
	}

	if len(clients) == 0 {
		panic("no clients to process load")
	}

	// Put the clients in a ring so they can be used in a
	// round-robin fashion.
	clientRing := ring.New(len(clients))
	for idx := range clients {
		clientRing.Value = clients[idx]
		clientRing = clientRing.Next()
	}

	var err error

	for {
		select {
		case <-ctx.Done():
			return
		case tx := <-chTx:
			clientRing = clientRing.Next()
			client := clientRing.Value.(*rpchttp.HTTP)

			if _, err := client.Health(ctx); err != nil {
				continue
			}

			if _, err = client.BroadcastTxSync(ctx, tx); err != nil {
				continue
			}

			select {
			case chSuccess <- tx:
				continue
			case <-ctx.Done():
				return
			}

		}
	}
}
