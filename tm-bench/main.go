package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/go-kit/kit/log/term"
	metrics "github.com/rcrowley/go-metrics"

	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmlibs/log"
	"github.com/tendermint/tools/tm-monitor/monitor"
)

var version = "0.1.0"

var logger = log.NewNopLogger()

type statistics struct {
	BlockTimeSample    metrics.Histogram
	TxThroughputSample metrics.Histogram
	BlockLatency       metrics.Histogram
}

func main() {
	var duration, txsRate, connections int
	var verbose bool

	flag.IntVar(&connections, "c", 1, "Connections to keep open per endpoint")
	flag.IntVar(&duration, "T", 10, "Exit after the specified amount of time in seconds")
	flag.IntVar(&txsRate, "r", 1000, "Txs per second to send in a connection")
	flag.BoolVar(&verbose, "v", false, "Verbose output")

	flag.Usage = func() {
		fmt.Println(`Tendermint blockchain benchmarking tool.

Usage:
	tm-bench [-c 1] [-T 10] [-r 1000] [endpoints]

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
	}

	fmt.Printf("Running %ds test @ %s\n", duration, flag.Arg(0))

	endpoints := strings.Split(flag.Arg(0), ",")

	blockCh := make(chan tmtypes.Header, 100)
	blockLatencyCh := make(chan float64, 100)

	nodes := startNodes(endpoints, blockCh, blockLatencyCh)

	transacters := startTransacters(endpoints, connections, txsRate)

	stats := &statistics{
		BlockTimeSample:    metrics.NewHistogram(metrics.NewUniformSample(1000)),
		TxThroughputSample: metrics.NewHistogram(metrics.NewUniformSample(1000)),
		BlockLatency:       metrics.NewHistogram(metrics.NewUniformSample(1000)),
	}

	lastBlockHeight := -1

	durationTimer := time.After(time.Duration(duration) * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	var blocks, txs int
	for {
		select {
		case b := <-blockCh:
			if lastBlockHeight < b.Height {
				blocks++
				txs += b.NumTxs
				lastBlockHeight = b.Height
			}
		case l := <-blockLatencyCh:
			stats.BlockLatency.Update(int64(l))
		case <-ticker.C:
			stats.BlockTimeSample.Update(int64(blocks))
			stats.TxThroughputSample.Update(int64(txs))
			blocks = 0
			txs = 0
		case <-durationTimer:
			for _, t := range transacters {
				t.Stop()
			}

			printStatistics(stats)

			for _, n := range nodes {
				n.Stop()
			}
			return
		}
	}
}

func startNodes(endpoints []string, blockCh chan<- tmtypes.Header, blockLatencyCh chan<- float64) []*monitor.Node {
	nodes := make([]*monitor.Node, len(endpoints))

	for i, e := range endpoints {
		n := monitor.NewNode(e)
		n.SetLogger(logger.With("node", e))
		n.SendBlocksTo(blockCh)
		n.SendBlockLatenciesTo(blockLatencyCh)
		if err := n.Start(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		nodes[i] = n
	}

	return nodes
}

func startTransacters(endpoints []string, connections int, txsRate int) []*transacter {
	transacters := make([]*transacter, len(endpoints))

	for i, e := range endpoints {
		t := newTransacter(e, connections, txsRate)
		t.SetLogger(logger)
		if err := t.Start(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		transacters[i] = t
	}

	return transacters
}

func printStatistics(stats *statistics) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 5, ' ', 0)
	fmt.Fprintln(w, "Stats\tAvg\tStdev\tMax\t")
	fmt.Fprintln(w, fmt.Sprintf("Block latency\t%.2fms\t%.2fms\t%dms\t",
		stats.BlockLatency.Mean()/1000000.0,
		stats.BlockLatency.StdDev()/1000000.0,
		stats.BlockLatency.Max()/1000000))
	fmt.Fprintln(w, fmt.Sprintf("Blocks/sec\t%.3f\t%.3f\t%d\t",
		stats.BlockTimeSample.Mean(),
		stats.BlockTimeSample.StdDev(),
		stats.BlockTimeSample.Max()))
	fmt.Fprintln(w, fmt.Sprintf("Txs/sec\t%.0f\t%.0f\t%d\t",
		stats.TxThroughputSample.Mean(),
		stats.TxThroughputSample.StdDev(),
		stats.TxThroughputSample.Max()))
	w.Flush()
}
