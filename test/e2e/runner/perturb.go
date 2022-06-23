package main

import (
	"fmt"
	"time"

	rpctypes "github.com/tendermint/tendermint/rpc/core/types"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Perturbs a running testnet.
func Perturb(testnet *e2e.Testnet) error {
	for _, node := range testnet.Nodes {
		for _, perturbation := range node.Perturbations {
			_, err := PerturbNode(node, perturbation)
			if err != nil {
				return err
			}
			time.Sleep(3 * time.Second) // give network some time to recover between each
		}
	}
	return nil
}

// PerturbNode perturbs a node with a given perturbation, returning its status
// after recovering.
func PerturbNode(node *e2e.Node, perturbation e2e.Perturbation) (*rpctypes.ResultStatus, error) {
	testnet := node.Testnet
	switch perturbation {
	case e2e.PerturbationDisconnect:
		logger.Info(fmt.Sprintf("Disconnecting node %v...", node.Name))
		if err := execDocker("network", "disconnect", testnet.Name+"_"+testnet.Name, node.Name); err != nil {
			return nil, err
		}
		time.Sleep(10 * time.Second)
		if err := execDocker("network", "connect", testnet.Name+"_"+testnet.Name, node.Name); err != nil {
			return nil, err
		}

	case e2e.PerturbationKill:
		logger.Info(fmt.Sprintf("Killing node %v...", node.Name))
		if err := execCompose(testnet.Dir, "kill", "-s", "SIGKILL", node.Name); err != nil {
			return nil, err
		}
		if err := execCompose(testnet.Dir, "start", node.Name); err != nil {
			return nil, err
		}

	case e2e.PerturbationPause:
		logger.Info(fmt.Sprintf("Pausing node %v...", node.Name))
		if err := execCompose(testnet.Dir, "pause", node.Name); err != nil {
			return nil, err
		}
		time.Sleep(10 * time.Second)
		if err := execCompose(testnet.Dir, "unpause", node.Name); err != nil {
			return nil, err
		}

	case e2e.PerturbationRestart:
		logger.Info(fmt.Sprintf("Restarting node %v...", node.Name))
		if err := execCompose(testnet.Dir, "restart", node.Name); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unexpected perturbation %q", perturbation)
	}

	status, err := waitForNode(node, 0, 20*time.Second)
	if err != nil {
		return nil, err
	}
	logger.Info(fmt.Sprintf("Node %v recovered at height %v", node.Name, status.SyncInfo.LatestBlockHeight))
	return status, nil
}
