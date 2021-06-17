package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"time"

	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

// Load generates transactions against the network until the given context is
// canceled. A multiplier of greater than one can be supplied if load needs to
// be generated beyond a minimum amount.
func Load(ctx context.Context, testnet *e2e.Testnet, multiplier int) error {
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

	chTx := make(chan types.Tx)
	chSuccess := make(chan types.Tx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Spawn job generator and processors.
	logger.Info(fmt.Sprintf("Starting transaction load (%v workers)...", concurrency))
	started := time.Now()

	go loadGenerate(ctx, chTx, multiplier)

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

// loadGenerate generates jobs until the context is canceled
func loadGenerate(ctx context.Context, chTx chan<- types.Tx, multiplier int) {
	for i := 0; i < math.MaxInt64; i++ {
		// We keep generating the same 1000 keys over and over, with different values.
		// This gives a reasonable load without putting too much data in the app.
		id := i % 1000

		bz := make([]byte, 1024) // 1kb hex-encoded
		_, err := rand.Read(bz)
		if err != nil {
			panic(fmt.Sprintf("Failed to read random bytes: %v", err))
		}
		tx := types.Tx(fmt.Sprintf("load-%X=%x", id, bz))

		select {
		case chTx <- tx:
			time.Sleep(time.Duration(100/multiplier) * time.Millisecond)

		case <-ctx.Done():
			close(chTx)
			return
		}
	}
}

// loadProcess processes transactions
func loadProcess(ctx context.Context, testnet *e2e.Testnet, chTx <-chan types.Tx, chSuccess chan<- types.Tx) {
	// Each worker gets its own client to each node, which allows for some
	// concurrency while still bounding it.
	clients := map[string]*rpchttp.HTTP{}

	var err error
	for tx := range chTx {
		node := testnet.RandomNode()
		client, ok := clients[node.Name]
		if !ok {
			client, err = node.Client()
			if err != nil {
				continue
			}

			// check that the node is up
			_, err = client.Health(ctx)
			if err != nil {
				continue
			}

			clients[node.Name] = client
		}

		if _, err = client.BroadcastTxSync(ctx, tx); err != nil {
			continue
		}
		chSuccess <- tx
	}
}
