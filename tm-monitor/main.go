package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/term"
	cmn "github.com/tendermint/go-common"
	monitor "github.com/tendermint/tools/tm-monitor/monitor"
)

var version = "0.3.0.pre"

var logger = log.NewNopLogger()

func main() {
	var listenAddr string
	var noton bool

	flag.StringVar(&listenAddr, "listen-addr", "tcp://0.0.0.0:46670", "HTTP and Websocket server listen address")
	flag.BoolVar(&noton, "no-ton", false, "Do not show ton (table of nodes)")

	flag.Usage = func() {
		fmt.Println(`Tendermint monitor watches over one or more Tendermint core
applications, collecting and providing various statistics to the user.

Usage:
	tm-monitor [-no-ton] [-listen-addr="tcp://0.0.0.0:46670"] [endpoints]

Examples:
	# monitor single instance
	tm-monitor localhost:46657

	# monitor a few instances by providing comma-separated list of RPC endpoints
	tm-monitor host1:46657,host2:46657`)
		fmt.Println("Flags:")
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	if noton {
		// Color errors red
		colorFn := func(keyvals ...interface{}) term.FgBgColor {
			for i := 1; i < len(keyvals); i += 2 {
				if _, ok := keyvals[i].(error); ok {
					return term.FgBgColor{Fg: term.White, Bg: term.Red}
				}
			}
			return term.FgBgColor{}
		}

		logger = term.NewLogger(os.Stdout, log.NewLogfmtLogger, colorFn)
	}

	m := startMonitor(flag.Arg(0))

	startRPC(listenAddr, m)

	var ton *Ton
	if !noton {
		ton = NewTon(m)
		ton.Start()
	}

	cmn.TrapSignal(func() {
		if !noton {
			ton.Stop()
		}
		m.Stop()
	})
}

func startMonitor(endpoints string) *monitor.Monitor {
	m := monitor.NewMonitor()
	m.SetLogger(log.With(logger, "component", "monitor"))

	for _, e := range strings.Split(endpoints, ",") {
		n := monitor.NewNode(e)
		n.SetLogger(log.With(logger, "node", e))
		if err := m.Monitor(n); err != nil {
			panic(err)
		}
	}

	if err := m.Start(); err != nil {
		panic(err)
	}

	return m
}
