package main

import (
	"fmt"
	"sort"
	"time"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

func Start(testnet *e2e.Testnet) error {

	// Sort nodes by starting order
	nodeQueue := testnet.Nodes
	sort.SliceStable(nodeQueue, func(i, j int) bool {
		return nodeQueue[i].StartAt < nodeQueue[j].StartAt
	})

	// Start initial nodes (StartAt: 0)
	logger.Info("Starting initial network nodes...")
	for len(nodeQueue) > 0 && nodeQueue[0].StartAt == 0 {
		node := nodeQueue[0]
		nodeQueue = nodeQueue[1:]
		if err := execCompose(testnet.Dir, "up", "-d", node.Name); err != nil {
			return err
		}
		if _, err := waitForNode(node, 0, 15*time.Second); err != nil {
			return err
		}
		logger.Info(fmt.Sprintf("Node %v up on http://127.0.0.1:%v", node.Name, node.ProxyPort))
	}

	// Wait for initial height
	logger.Info(fmt.Sprintf("Waiting for initial height %v...", testnet.InitialHeight))
	block, blockID, err := waitForHeight(testnet, testnet.InitialHeight)
	if err != nil {
		return err
	}

	// Update any state sync nodes with a trusted height and hash
	for _, node := range nodeQueue {
		if node.StateSync {
			err = UpdateConfigStateSync(node, block.Height, blockID.Hash.Bytes())
			if err != nil {
				return err
			}
		}
	}

	// Start up remaining nodes
	for _, node := range nodeQueue {
		logger.Info(fmt.Sprintf("Starting node %v at height %v...", node.Name, node.StartAt))
		if _, _, err := waitForHeight(testnet, node.StartAt); err != nil {
			return err
		}
		if err := execCompose(testnet.Dir, "up", "-d", node.Name); err != nil {
			return err
		}
		status, err := waitForNode(node, node.StartAt, 1*time.Minute)
		if err != nil {
			return err
		}
		logger.Info(fmt.Sprintf("Node %v up on http://127.0.0.1:%v at height %v",
			node.Name, node.ProxyPort, status.SyncInfo.LatestBlockHeight))
	}

	return nil
}
