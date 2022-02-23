package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	rpctypes "github.com/tendermint/tendermint/rpc/coretypes"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

// waitForHeight waits for the network to reach a certain height (or above),
// returning the block at the height seen. Errors if the network is not making
// progress at all.
// If height == 0, the initial height of the test network is used as the target.
func waitForHeight(ctx context.Context, testnet *e2e.Testnet, height int64) (*types.Block, *types.BlockID, error) {
	var (
		err             error
		clients         = map[string]*rpchttp.HTTP{}
		lastHeight      int64
		lastIncrease    = time.Now()
		nodesAtHeight   = map[string]struct{}{}
		numRunningNodes int
	)
	if height == 0 {
		height = testnet.InitialHeight
	}

	for _, node := range testnet.Nodes {
		if node.Stateless() {
			continue
		}

		if node.HasStarted {
			numRunningNodes++
		}
	}

	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-timer.C:
			for _, node := range testnet.Nodes {
				// skip nodes that have reached the target height
				if _, ok := nodesAtHeight[node.Name]; ok {
					continue
				}

				// skip nodes that don't have state or haven't started yet
				if node.Stateless() {
					continue
				}
				if !node.HasStarted {
					continue
				}

				// cache the clients
				client, ok := clients[node.Name]
				if !ok {
					client, err = node.Client()
					if err != nil {
						continue
					}
					clients[node.Name] = client
				}

				result, err := client.Status(ctx)
				if err != nil {
					continue
				}
				if result.SyncInfo.LatestBlockHeight > lastHeight {
					lastHeight = result.SyncInfo.LatestBlockHeight
					lastIncrease = time.Now()
				}

				if result.SyncInfo.LatestBlockHeight >= height {
					// the node has achieved the target height!

					// add this node to the set of target
					// height nodes
					nodesAtHeight[node.Name] = struct{}{}

					// if not all of the nodes that we
					// have clients for have reached the
					// target height, keep trying.
					if numRunningNodes > len(nodesAtHeight) {
						continue
					}

					// All nodes are at or above the target height. Now fetch the block for that target height
					// and return it. We loop again through all clients because some may have pruning set but
					// at least two of them should be archive nodes.
					for _, c := range clients {
						result, err := c.Block(ctx, &height)
						if err != nil || result == nil || result.Block == nil {
							continue
						}
						return result.Block, &result.BlockID, err
					}
				}
			}

			if len(clients) == 0 {
				return nil, nil, errors.New("unable to connect to any network nodes")
			}
			if time.Since(lastIncrease) >= time.Minute {
				if lastHeight == 0 {
					return nil, nil, errors.New("chain stalled at unknown height (most likely upon starting)")
				}

				return nil, nil, fmt.Errorf("chain stalled at height %v [%d of %d nodes %+v]",
					lastHeight,
					len(nodesAtHeight),
					numRunningNodes,
					nodesAtHeight)

			}
			timer.Reset(1 * time.Second)
		}
	}
}

// waitForNode waits for a node to become available and catch up to the given block height.
func waitForNode(ctx context.Context, logger log.Logger, node *e2e.Node, height int64) (*rpctypes.ResultStatus, error) {
	// If the node is the light client or seed note, we do not check for the last height.
	// The light client and seed note can be behind the full node and validator
	if node.Mode == e2e.ModeSeed {
		return nil, nil
	}
	client, err := node.Client()
	if err != nil {
		return nil, err
	}

	timer := time.NewTimer(0)
	defer timer.Stop()

	var (
		lastFailed bool
		counter    int
	)
	for {
		counter++
		if lastFailed {
			lastFailed = false

			// if there was a problem with the request in
			// the previous recreate the client to ensure
			// reconnection
			client, err = node.Client()
			if err != nil {
				return nil, err
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			status, err := client.Status(ctx)
			switch {
			case errors.Is(err, context.DeadlineExceeded):
				return nil, fmt.Errorf("timed out waiting for %v to reach height %v", node.Name, height)
			case errors.Is(err, context.Canceled):
				return nil, err
				// If the node is the light client, it is not essential to wait for it to catch up, but we must return status info
			case err == nil && node.Mode == e2e.ModeLight:
				return status, nil
			case err == nil && node.Mode != e2e.ModeLight && status.SyncInfo.LatestBlockHeight >= height:
				return status, nil
			case counter%500 == 0:
				switch {
				case err != nil:
					lastFailed = true
					logger.Error("node not yet ready",
						"iter", counter,
						"node", node.Name,
						"target", height,
						"err", err,
					)
				case status != nil:
					logger.Info("node not yet ready",
						"iter", counter,
						"node", node.Name,
						"height", status.SyncInfo.LatestBlockHeight,
						"target", height,
					)
				}
			}
			timer.Reset(250 * time.Millisecond)
		}
	}
}

// getLatestBlock returns the last block that all active nodes in the network have
// agreed upon i.e. the earlist of each nodes latest block
func getLatestBlock(ctx context.Context, testnet *e2e.Testnet) (*types.Block, error) {
	var earliestBlock *types.Block
	for _, node := range testnet.Nodes {
		// skip nodes that don't have state or haven't started yet
		if node.Stateless() {
			continue
		}
		if !node.HasStarted {
			continue
		}

		client, err := node.Client()
		if err != nil {
			return nil, err
		}

		wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		result, err := client.Block(wctx, nil)
		if err != nil {
			return nil, err
		}

		if result.Block != nil && (earliestBlock == nil || earliestBlock.Height > result.Block.Height) {
			earliestBlock = result.Block
		}
	}
	return earliestBlock, nil
}
