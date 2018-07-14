package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"text/tabwriter"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	tmrpc "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
)

type statistics struct {
	TxsThroughput    metrics.Histogram `json:"txs_per_sec"`
	BlocksThroughput metrics.Histogram `json:"blocks_per_sec"`
}

// calculateStatistics calculates the tx / second, and blocks / second based
// off of the number the transactions and number of blocks that occurred from
// the start block, and the end time.
func calculateStatistics(
	client tmrpc.Client,
	minHeight int64,
	timeStart time.Time,
	duration int,
) (*statistics, error) {
	timeEnd := timeStart.Add(time.Duration(duration) * time.Second)

	stats := &statistics{
		BlocksThroughput: metrics.NewHistogram(metrics.NewUniformSample(1000)),
		TxsThroughput:    metrics.NewHistogram(metrics.NewUniformSample(1000)),
	}

	var (
		numBlocksPerSec = make(map[int64]int64)
		numTxsPerSec    = make(map[int64]int64)
	)

	// because during some seconds blocks won't be created...
	for i := int64(0); i < int64(duration); i++ {
		numBlocksPerSec[i] = 0
		numTxsPerSec[i] = 0
	}

	blockMetas, err := getBlockMetas(client, minHeight, timeStart, timeEnd)
	if err != nil {
		return nil, err
	}

	// iterates from max height to min height
	for _, blockMeta := range blockMetas {
		// check if block was created after timeStart
		if blockMeta.Header.Time.Before(timeStart) {
			break
		}

		// check if block was created before timeEnd
		if blockMeta.Header.Time.After(timeEnd) {
			continue
		}
		sec := secondsSinceTimeStart(timeStart, blockMeta.Header.Time)

		// increase number of blocks for that second
		numBlocksPerSec[sec]++

		// increase number of txs for that second
		numTxsPerSec[sec] += blockMeta.Header.NumTxs
		logger.Debug(fmt.Sprintf("%d txs at block height %d", blockMeta.Header.NumTxs, blockMeta.Header.Height))
	}

	for i := int64(0); i < int64(duration); i++ {
		stats.BlocksThroughput.Update(numBlocksPerSec[i])
		stats.TxsThroughput.Update(numTxsPerSec[i])
	}

	return stats, nil
}

func getBlockMetas(client tmrpc.Client, minHeight int64, timeStart, timeEnd time.Time) ([]*types.BlockMeta, error) {
	// get blocks between minHeight and last height
	// This returns max(minHeight,(last_height - 20)) to last_height
	info, err := client.BlockchainInfo(minHeight, 0)
	if err != nil {
		return nil, err
	}

	var (
		blockMetas = info.BlockMetas
		lastHeight = info.LastHeight
		diff       = lastHeight - minHeight
		offset     = len(blockMetas)
	)

	for offset < int(diff) {
		// get blocks between minHeight and last height
		info, err := client.BlockchainInfo(minHeight, lastHeight-int64(offset))
		if err != nil {
			return nil, err
		}
		blockMetas = append(blockMetas, info.BlockMetas...)
		offset = len(blockMetas)
	}

	return blockMetas, nil
}

func secondsSinceTimeStart(timeStart, timePassed time.Time) int64 {
	return int64(math.Round(timePassed.Sub(timeStart).Seconds()))
}

func printStatistics(stats *statistics, outputFormat string) {
	if outputFormat == "json" {
		result, err := json.Marshal(struct {
			TxsThroughput    float64 `json:"txs_per_sec_avg"`
			BlocksThroughput float64 `json:"blocks_per_sec_avg"`
		}{stats.TxsThroughput.Mean(), stats.BlocksThroughput.Mean()})

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(string(result))
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 5, ' ', 0)
		fmt.Fprintln(w, "Stats\tAvg\tStdDev\tMax\tTotal\t")
		fmt.Fprintln(
			w,
			fmt.Sprintf(
				"Txs/sec\t%.0f\t%.0f\t%d\t%d\t",
				stats.TxsThroughput.Mean(),
				stats.TxsThroughput.StdDev(),
				stats.TxsThroughput.Max(),
				stats.TxsThroughput.Sum(),
			),
		)
		fmt.Fprintln(
			w,
			fmt.Sprintf("Blocks/sec\t%.3f\t%.3f\t%d\t%d\t",
				stats.BlocksThroughput.Mean(),
				stats.BlocksThroughput.StdDev(),
				stats.BlocksThroughput.Max(),
				stats.BlocksThroughput.Sum(),
			),
		)
		w.Flush()
	}
}
