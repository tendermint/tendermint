package bench

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	monitor "github.com/tendermint/tools/tm-monitor"
)

var version = "0.1.0.pre"

func main() {
	var listenAddr string
	var connections, duration, txsRate int

	flag.StringVar(&listenAddr, "listen-addr", "tcp://0.0.0.0:46670", "HTTP and Websocket server listen address")
	flag.IntVar(&duration, "T", 10, "Exit after the specified amount of time in seconds")
	flag.IntVar(&txsRate, "r", 1000, "Txs per second to send in a connection")

	flag.Usage = func() {
		fmt.Println(`Tendermint bench.

Usage:
	tm-bench [-listen-addr="tcp://0.0.0.0:46670"] [-T 10] [-r 1000] [endpoints]

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

	fmt.Printf("Running %ds test @ %s\n", duration, flag.Arg(0))

	m := startMonitor(flag.Arg(0))

	transacters := make([]*Transacter, len(endpoints))
	for _, e := range strings.Split(endpoints, ",") {
		t := &transacter{e, txsRate}
		if err := t.Start(); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		append(transacters, t)
	}

	select {
	case time.After(duration * time.Second):
		for _, t := range transacters {
			t.Stop()
		}
		collectAndPrintResults(m)
		m.Stop()
	}

	cmn.TrapSignal(func() {
		for _, t := range transacters {
			t.Stop()
		}
		collectAndPrintResults(m)
		m.Stop()
	})
}

func startMonitor(endpoints string) *monitor.Monitor {
	m := monitor.NewMonitor()

	for _, e := range strings.Split(endpoints, ",") {
		if err := m.Monitor(monitor.NewNode(e)); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}

	if err := m.Start(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	return m
}

func collectAndPrintResults(m *Monitor) {
	fmt.Printf("%d", m.Network.Height)
}
