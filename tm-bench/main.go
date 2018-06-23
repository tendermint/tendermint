package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/go-kit/kit/log/term"
	metrics "github.com/rcrowley/go-metrics"
	tmrpc "github.com/tendermint/tendermint/rpc/client"

	"github.com/tendermint/tmlibs/log"
)

var version = "0.3.0"

var logger = log.NewNopLogger()

type statistics struct {
	TxsThroughput    metrics.Histogram `json:"txs_per_sec"`
	BlocksThroughput metrics.Histogram `json:"blocks_per_sec"`
}

func main() {
	var duration, txsRate, connections, txSize int
	var verbose bool
	var outputFormat, broadcastTxMethod string

	flag.IntVar(&connections, "c", 1, "Connections to keep open per endpoint")
	flag.IntVar(&duration, "T", 10, "Exit after the specified amount of time in seconds")
	flag.IntVar(&txsRate, "r", 1000, "Txs per second to send in a connection")
	flag.IntVar(&txSize, "s", 250, "The size of a transaction in bytes.")
	flag.StringVar(&outputFormat, "output-format", "plain", "Output format: plain or json")
	flag.StringVar(&broadcastTxMethod, "broadcast-tx-method", "async", "Broadcast method: async (no guarantees; fastest), sync (ensures tx is checked) or commit (ensures tx is checked and committed; slowest)")
	flag.BoolVar(&verbose, "v", false, "Verbose output")

	flag.Usage = func() {
		fmt.Println(`Tendermint blockchain benchmarking tool.

Usage:
	tm-bench [-c 1] [-T 10] [-r 1000] [endpoints] [-output-format <plain|json> [-broadcast-tx-method <async|sync|commit>]]

Examples:
	tm-bench localhost:26657`)
		fmt.Println("Flags:")
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	if verbose {
		if outputFormat == "json" {
			fmt.Fprintln(os.Stderr, "Verbose mode not supported with json output.")
			os.Exit(1)
		}
		// Color errors red
		colorFn := func(keyvals ...interface{}) term.FgBgColor {
			for i := 1; i < len(keyvals); i += 2 {
				if _, ok := keyvals[i].(error); ok {
					return term.FgBgColor{Fg: term.White, Bg: term.Red}
				}
			}
			return term.FgBgColor{}
		}
		logger = log.NewTMLoggerWithColorFn(log.NewSyncWriter(os.Stdout), colorFn)

		fmt.Printf("Running %ds test @ %s\n", duration, flag.Arg(0))
	}

	if broadcastTxMethod != "async" &&
		broadcastTxMethod != "sync" &&
		broadcastTxMethod != "commit" {
		fmt.Fprintln(
			os.Stderr,
			"broadcast-tx-method should be either 'sync', 'async' or 'commit'.",
		)
		os.Exit(1)
	}

	var (
		endpoints     = strings.Split(flag.Arg(0), ",")
		client        = tmrpc.NewHTTP(endpoints[0], "/websocket")
		initialHeight = latestBlockHeight(client)
	)
	logger.Info("Latest block height", "h", initialHeight)

	// record time start
	timeStart := time.Now()
	logger.Info("Time started", "t", timeStart)

	transacters := startTransacters(
		endpoints,
		connections,
		txsRate,
		txSize,
		"broadcast_tx_"+broadcastTxMethod,
	)

	select {
	case <-time.After(time.Duration(duration) * time.Second):
		for _, t := range transacters {
			t.Stop()
		}

		timeStop := time.Now()
		logger.Info("Time stopped", "t", timeStop)

		stats, err := calculateStatistics(
			client,
			initialHeight,
			timeStart,
			timeStop,
			duration,
		)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		printStatistics(stats, outputFormat)

		return
	}
}

func latestBlockHeight(client tmrpc.Client) int64 {
	status, err := client.Status()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return status.SyncInfo.LatestBlockHeight
}

// calculateStatistics calculates the tx / second, and blocks / second based
// off of the number the transactions and number of blocks that occurred from
// the start block, and the end time.
func calculateStatistics(
	client tmrpc.Client,
	minHeight int64,
	timeStart, timeStop time.Time,
	duration int,
) (*statistics, error) {
	stats := &statistics{
		BlocksThroughput: metrics.NewHistogram(metrics.NewUniformSample(1000)),
		TxsThroughput:    metrics.NewHistogram(metrics.NewUniformSample(1000)),
	}

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

	var (
		numBlocksPerSec = make(map[int64]int64)
		numTxsPerSec    = make(map[int64]int64)
	)

	// because during some seconds blocks won't be created...
	for i := int64(0); i < int64(duration); i++ {
		numBlocksPerSec[i] = 0
		numTxsPerSec[i] = 0
	}

	// iterates from max height to min height
	for _, blockMeta := range blockMetas {
		// check if block was created after timeStart
		if blockMeta.Header.Time.Before(timeStart) {
			break
		}

		// check if block was created before timeStop
		if blockMeta.Header.Time.After(timeStop) {
			continue
		}
		sec := secondsSinceTimeStart(timeStart, blockMeta.Header.Time)

		// increase number of blocks for that second
		numBlocksPerSec[sec]++

		// increase number of txs for that second
		numTxsPerSec[sec] += blockMeta.Header.NumTxs
	}

	for _, n := range numBlocksPerSec {
		stats.BlocksThroughput.Update(n)
	}

	for _, n := range numTxsPerSec {
		stats.TxsThroughput.Update(n)
	}

	return stats, nil
}

func secondsSinceTimeStart(timeStart, timePassed time.Time) int64 {
	return int64(math.Round(timePassed.Sub(timeStart).Seconds()))
}

func startTransacters(
	endpoints []string,
	connections,
	txsRate int,
	txSize int,
	broadcastTxMethod string,
) []*transacter {
	transacters := make([]*transacter, len(endpoints))

	for i, e := range endpoints {
		t := newTransacter(e, connections, txsRate, txSize, broadcastTxMethod)
		t.SetLogger(logger)
		if err := t.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		transacters[i] = t
	}

	return transacters
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
		fmt.Fprintln(w, "Stats\tAvg\tStdDev\tMax\t")
		fmt.Fprintln(w, fmt.Sprintf("Txs/sec\t%.0f\t%.0f\t%d\t",
			stats.TxsThroughput.Mean(),
			stats.TxsThroughput.StdDev(),
			stats.TxsThroughput.Max()))
		fmt.Fprintln(w, fmt.Sprintf("Blocks/sec\t%.3f\t%.3f\t%d\t",
			stats.BlocksThroughput.Mean(),
			stats.BlocksThroughput.StdDev(),
			stats.BlocksThroughput.Max()))
		w.Flush()
	}
}
