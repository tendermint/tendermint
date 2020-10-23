package main

import (
	"fmt"
	"time"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

const interphaseWaitPeriod = 5

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

// WaitForAllMisbehaviors calculates the height of the last misbehavior and ensures the entire
// testnet has surpassed this height before moving on to the next phase
func waitForAllMisbehaviors(testnet *e2e.Testnet) error {
	_, _, err := waitForHeight(testnet, lastMisbehaviorHeight(testnet))
	return err
}

func lastMisbehaviorHeight(testnet *e2e.Testnet) int64 {
	lastHeight := testnet.InitialHeight
	for _, n := range testnet.Nodes {
		for height := range n.Misbehaviors {
			if height > lastHeight {
				lastHeight = height
			}
		}
	}
	return lastHeight + interphaseWaitPeriod
}
