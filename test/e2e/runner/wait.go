package main

import (
	"context"
	"fmt"

	"github.com/tendermint/tendermint/libs/log"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Wait waits for a number of blocks to be produced, and for all nodes to catch
// up with it.
func Wait(ctx context.Context, logger log.Logger, testnet *e2e.Testnet, blocks int64) error {
	block, err := getLatestBlock(ctx, testnet)
	if err != nil {
		return err
	}
	return WaitUntil(ctx, logger, testnet, block.Height+blocks)
}

// WaitUntil waits until a given height has been reached.
func WaitUntil(ctx context.Context, logger log.Logger, testnet *e2e.Testnet, height int64) error {
	logger.Info(fmt.Sprintf("Waiting for all nodes to reach height %v...", height))

	_, _, err := waitForHeight(ctx, testnet, height)

	return err
}
