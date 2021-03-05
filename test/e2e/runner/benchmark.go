package main

import (
	"context"
	"fmt"
	"math"
	"time"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

// Benchmark is a simple function for fetching, calculating and printing
// the following metrics:
// 1. Average block production time
// 2. Block interval standard deviation
// 3. Max block interval (slowest block)
// 4. Min block interval (fastest block)
//
// Metrics are based of the `benchmarkLength`, the amount of consecutive blocks
// sampled from in the testnet
func Benchmark(testnet *e2e.Testnet, benchmarkLength int64) error {
	block, _, err := waitForHeight(testnet, 0)
	if err != nil {
		return err
	}

	logger.Info("Beginning benchmark period...", "height", block.Height)

	// wait for the length of the benchmark period in blocks to pass. We allow 5 seconds for each block
	// which should be sufficient.
	waitingTime := time.Duration(benchmarkLength*5) * time.Second
	endHeight, err := waitForAllNodes(testnet, block.Height+benchmarkLength, waitingTime)
	if err != nil {
		return err
	}

	logger.Info("Ending benchmark period", "height", endHeight)

	// fetch a sample of blocks
	blocks, err := fetchBlockChainSample(testnet, benchmarkLength)
	if err != nil {
		return err
	}

	// slice into time intervals and collate data
	timeIntervals := splitIntoBlockIntervals(blocks)
	testnetStats := extractTestnetStats(timeIntervals)
	testnetStats.startHeight = blocks[0].Header.Height
	testnetStats.endHeight = blocks[len(blocks)-1].Header.Height

	// print and return
	logger.Info(testnetStats.String())
	return nil
}

type testnetStats struct {
	startHeight int64
	endHeight   int64

	// average time to produce a block
	mean time.Duration
	// standard deviation of block production
	std float64
	// longest time to produce a block
	max time.Duration
	// shortest time to produce a block
	min time.Duration
}

func (t *testnetStats) String() string {
	return fmt.Sprintf(`Benchmarked from height %v to %v
	Mean Block Interval: %v
	Standard Deviation: %f
	Max Block Interval: %v
	Min Block Interval: %v
	`,
		t.startHeight,
		t.endHeight,
		t.mean,
		t.std,
		t.max,
		t.min,
	)
}

// fetchBlockChainSample waits for `benchmarkLength` amount of blocks to pass, fetching
// all of the headers for these blocks from an archive node and returning it.
func fetchBlockChainSample(testnet *e2e.Testnet, benchmarkLength int64) ([]*types.BlockMeta, error) {
	var blocks []*types.BlockMeta

	// Find the first archive node
	archiveNode := testnet.ArchiveNodes()[0]
	c, err := archiveNode.Client()
	if err != nil {
		return nil, err
	}

	// find the latest height
	ctx := context.Background()
	s, err := c.Status(ctx)
	if err != nil {
		return nil, err
	}

	to := s.SyncInfo.LatestBlockHeight
	from := to - benchmarkLength + 1
	if from <= testnet.InitialHeight {
		return nil, fmt.Errorf("tesnet was unable to reach required height for benchmarking (latest height %d)", to)
	}

	// Fetch blocks
	for from < to {
		// fetch the blockchain metas. Currently we can only fetch 20 at a time
		resp, err := c.BlockchainInfo(ctx, from, min(from+19, to))
		if err != nil {
			return nil, err
		}

		blockMetas := resp.BlockMetas
		// we receive blocks in descending order so we have to add them in reverse
		for i := len(blockMetas) - 1; i >= 0; i-- {
			if blockMetas[i].Header.Height != from {
				return nil, fmt.Errorf("node gave us another header. Wanted %d, got %d",
					from,
					blockMetas[i].Header.Height,
				)
			}
			from++
			blocks = append(blocks, blockMetas[i])
		}
	}

	return blocks, nil
}

func splitIntoBlockIntervals(blocks []*types.BlockMeta) []time.Duration {
	intervals := make([]time.Duration, len(blocks)-1)
	lastTime := blocks[0].Header.Time
	for i, block := range blocks {
		// skip the first block
		if i == 0 {
			continue
		}

		intervals[i-1] = block.Header.Time.Sub(lastTime)
		lastTime = block.Header.Time
	}
	return intervals
}

func extractTestnetStats(intervals []time.Duration) testnetStats {
	var (
		sum, mean time.Duration
		std       float64
		max       = intervals[0]
		min       = intervals[0]
	)

	for _, interval := range intervals {
		sum += interval

		if interval > max {
			max = interval
		}

		if interval < min {
			min = interval
		}
	}
	mean = sum / time.Duration(len(intervals))

	for _, interval := range intervals {
		diff := (interval - mean).Seconds()
		std += math.Pow(diff, 2)
	}
	std = math.Sqrt(std / float64(len(intervals)))

	return testnetStats{
		mean: mean,
		std:  std,
		max:  max,
		min:  min,
	}
}

func min(a, b int64) int64 {
	if a > b {
		return b
	}
	return a
}
