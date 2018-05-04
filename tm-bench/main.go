package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-kit/kit/log/term"
	metrics "github.com/rcrowley/go-metrics"

	"text/tabwriter"

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
	var duration, txsRate, connections int
	var verbose bool
	var outputFormat string

	flag.IntVar(&connections, "c", 1, "Connections to keep open per endpoint")
	flag.IntVar(&duration, "T", 10, "Exit after the specified amount of time in seconds")
	flag.IntVar(&txsRate, "r", 1000, "Txs per second to send in a connection")
	flag.StringVar(&outputFormat, "output-format", "plain", "Output format: plain or json")
	flag.BoolVar(&verbose, "v", false, "Verbose output")

	flag.Usage = func() {
		fmt.Println(`Tendermint blockchain benchmarking tool.

Usage:
	tm-bench [-c 1] [-T 10] [-r 1000] [endpoints] [-output-format <plain|json>]

Examples:
	tm-bench localhost:46657`)
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

	endpoints := strings.Split(flag.Arg(0), ",")

	client := tmrpc.NewHTTP(endpoints[0], "/websocket")

	minHeight := latestBlockHeight(client)
	logger.Info("Latest block height", "h", minHeight)

	// record time start
	timeStart := time.Now()
	logger.Info("Time started", "t", timeStart)

	transacters := startTransacters(endpoints, connections, txsRate)

	select {
	case <-time.After(time.Duration(duration) * time.Second):
		for _, t := range transacters {
			t.Stop()
		}
		timeStop := time.Now()
		logger.Info("Time stopped", "t", timeStop)

		stats := calculateStatistics(client, minHeight, timeStart, timeStop)

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

func calculateStatistics(client tmrpc.Client, minHeight int64, timeStart, timeStop time.Time) *statistics {
	stats := &statistics{
		BlocksThroughput: metrics.NewHistogram(metrics.NewUniformSample(1000)),
		TxsThroughput:    metrics.NewHistogram(metrics.NewUniformSample(1000)),
	}

	// get blocks between minHeight and last height
	info, err := client.BlockchainInfo(minHeight, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	numBlocksPerSec := make(map[int64]int64)
	numTxsPerSec := make(map[int64]int64)
	for _, blockMeta := range info.BlockMetas {
		// check if block was created before timeStop
		if blockMeta.Header.Time.After(timeStop) {
			break
		}

		sec := secondsSinceTimeStart(timeStart, blockMeta.Header.Time)

		// increase number of blocks for that second
		if _, ok := numBlocksPerSec[sec]; !ok {
			numBlocksPerSec[sec] = 0
		}
		numBlocksPerSec[sec]++

		// increase number of txs for that second
		if _, ok := numTxsPerSec[sec]; !ok {
			numTxsPerSec[sec] = 0
		}
		numTxsPerSec[sec] += blockMeta.Header.NumTxs
	}

	for _, n := range numBlocksPerSec {
		stats.BlocksThroughput.Update(n)
	}

	for _, n := range numTxsPerSec {
		stats.TxsThroughput.Update(n)
	}

	return stats
}

func secondsSinceTimeStart(timeStart, timePassed time.Time) int64 {
	return int64(timePassed.Sub(timeStart).Seconds())
}

func startTransacters(endpoints []string, connections int, txsRate int) []*transacter {
	transacters := make([]*transacter, len(endpoints))

	for i, e := range endpoints {
		t := newTransacter(e, connections, txsRate)
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
