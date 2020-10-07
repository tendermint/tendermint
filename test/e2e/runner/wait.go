package main

import (
	"fmt"
	"time"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Wait waits for a number of blocks to be produced, and for all nodes to catch
// up with it.
func Wait(testnet *e2e.Testnet, blocks int64) error {
	block, _, err := waitForHeight(testnet, 0)
	if err != nil {
		return err
	}
	waitFor := block.Height + blocks
	logger.Info(fmt.Sprintf("Waiting for all nodes to reach height %v...", waitFor))
	_, err = waitForAllNodes(testnet, waitFor, 20*time.Second)
	if err != nil {
		return err
	}
	return nil
}
