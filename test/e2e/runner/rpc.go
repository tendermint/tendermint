package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	rpctypes "github.com/tendermint/tendermint/rpc/core/types"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

// waitForHeight waits for the network to reach a certain height (or above),
// returning the highest height seen. Errors if the network is not making
// progress at all.
func waitForHeight(ctx context.Context, testnet *e2e.Testnet, height int64) (*types.Block, *types.BlockID, error) {
	var (
		err             error
		maxResult       *rpctypes.ResultBlock
		clients         = map[string]*rpchttp.HTTP{}
		lastIncrease    = time.Now()
		nodesAtHeight   = map[string]struct{}{}
		numRunningNodes int
	)
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

				wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				result, err := client.Block(wctx, nil)
				if err != nil {
					continue
				}
				if result.Block != nil && (maxResult == nil || result.Block.Height > maxResult.Block.Height) {
					maxResult = result
					lastIncrease = time.Now()
				}

				if maxResult != nil && maxResult.Block.Height >= height {
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

					// return once all nodes have reached
					// the target height.
					return maxResult.Block, &maxResult.BlockID, nil
				}
			}

			if len(clients) == 0 {
				return nil, nil, errors.New("unable to connect to any network nodes")
			}
			if time.Since(lastIncrease) >= time.Minute {
				if maxResult == nil {
					return nil, nil, errors.New("chain stalled at unknown height")
				}

				return nil, nil, fmt.Errorf("chain stalled at height %v [%d of %d nodes %+v]",
					maxResult.Block.Height,
					len(nodesAtHeight),
					numRunningNodes,
					nodesAtHeight)

			}
			timer.Reset(1 * time.Second)
		}
	}
}

// waitForNode waits for a node to become available and catch up to the given block height.
func waitForNode(ctx context.Context, node *e2e.Node, height int64) (*rpctypes.ResultStatus, error) {
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
			case err == nil && status.SyncInfo.LatestBlockHeight >= height:
				return status, nil
			case counter%100 == 0:
				switch {
				case err != nil:
					lastFailed = true
					logger.Error("node not yet ready",
						"iter", counter,
						"node", node.Name,
						"err", err,
						"target", height,
					)
				case status != nil:
					logger.Error("node not yet ready",
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
