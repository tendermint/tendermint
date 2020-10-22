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

// WaitForAllMisbehaviors calculates how many blocks from the start does the chain
// need to run through before it has gone through all the misbehaviors
// It adds the InterphaseWaitPeriod afterwards
func WaitForAllMisbehaviors(testnet *e2e.Testnet) error {
	return Wait(testnet, lastMisbehaviorHeight(testnet))
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
	// remember we want to know how many blocks to wait so we
	// must calculate the difference between the initial height
	return (lastHeight + interphaseWaitPeriod) - testnet.InitialHeight
}
