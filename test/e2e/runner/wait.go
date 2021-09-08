package main

import (
	"context"
	"fmt"
	"time"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Wait waits for a number of blocks to be produced, and for all nodes to catch
// up with it.
func Wait(ctx context.Context, testnet *e2e.Testnet, blocks int64) error {
	block, _, err := waitForHeight(testnet, 0)
	if err != nil {
		return err
	}
	return WaitUntil(ctx, testnet, block.Height+blocks)
}

// WaitUntil waits until a given height has been reached.
func WaitUntil(ctx context.Context, testnet *e2e.Testnet, height int64) error {
	logger.Info(fmt.Sprintf("Waiting for all nodes to reach height %v...", height))
	wctx, wcancel := context.WithTimeout(ctx, waitingTime(len(testnet.Nodes)))
	defer wcancel()

	_, err := waitForAtLeastOneNode(wctx, testnet, height)
	return err
}

// waitingTime estimates how long it should take for a node to reach the height.
// More nodes in a network implies we may expect a slower network and may have to wait longer.
func waitingTime(nodes int) time.Duration {
	return time.Minute + (time.Duration(nodes) * (30 * time.Second))
}
