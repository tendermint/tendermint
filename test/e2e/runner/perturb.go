package main

import (
	"context"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/tendermint/tendermint/rpc/coretypes"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Perturbs a running testnet.
func Perturb(ctx context.Context, logger log.Logger, testnet *e2e.Testnet) error {
	timer := time.NewTimer(0) // first tick fires immediately; reset below
	defer timer.Stop()

	for _, node := range testnet.Nodes {
		for _, perturbation := range node.Perturbations {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				_, err := PerturbNode(ctx, logger, node, perturbation)
				if err != nil {
					return err
				}

				// give network some time to recover between each
				timer.Reset(20 * time.Second)
			}
		}
	}
	return nil
}

// PerturbNode perturbs a node with a given perturbation, returning its status
// after recovering.
func PerturbNode(ctx context.Context, logger log.Logger, node *e2e.Node, perturbation e2e.Perturbation) (*rpctypes.ResultStatus, error) {
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
		time.Sleep(10 * time.Second)
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
		if err := execCompose(testnet.Dir, "kill", "-s", "SIGTERM", node.Name); err != nil {
			return nil, err
		}
		time.Sleep(10 * time.Second)
		if err := execCompose(testnet.Dir, "start", node.Name); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unexpected perturbation %q", perturbation)
	}

	// Seed nodes do not have an RPC endpoint exposed so we cannot assert that
	// the node recovered. All we can do is hope.
	if node.Mode == e2e.ModeSeed {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	status, err := waitForNode(ctx, logger, node, 0)
	if err != nil {
		return nil, err
	}
	logger.Info(fmt.Sprintf("Node %v recovered at height %v", node.Name, status.SyncInfo.LatestBlockHeight))
	return status, nil
}
