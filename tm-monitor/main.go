package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	cmn "github.com/tendermint/go-common"
	logger "github.com/tendermint/go-logger"
	log15 "github.com/tendermint/log15"
)

var version = "0.2.0"

var log = logger.New()

func main() {
	var listenAddr string
	var verbose, noton bool

	flag.StringVar(&listenAddr, "listen-addr", "tcp://0.0.0.0:46670", "HTTP and Websocket server listen address")
	flag.BoolVar(&verbose, "v", false, "verbose logging")
	flag.BoolVar(&noton, "no-ton", false, "Do not show ton (table of nodes)")

	flag.Usage = func() {
		fmt.Println(`Tendermint monitor watches over one or more Tendermint core
applications, collecting and providing various statistics to the user.

Usage:
	tm-monitor [-v] [-no-ton] [-listen-addr="tcp://0.0.0.0:46670"] [endpoints]

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

	m := startMonitor(flag.Arg(0))

	startRPC(listenAddr, m)

	var ton *Ton
	if !noton {
		logToFile("tm-monitor.log", verbose)
		ton = NewTon(m)
		ton.Start()
	} else {
		logToStdout(verbose)
	}

	cmn.TrapSignal(func() {
		if !noton {
			ton.Stop()
		}
		m.Stop()
	})
}

func startMonitor(endpoints string) *Monitor {
	m := NewMonitor()

	for _, e := range strings.Split(endpoints, ",") {
		if err := m.Monitor(NewNode(e)); err != nil {
			log.Crit(err.Error())
			os.Exit(1)
		}
	}

	if err := m.Start(); err != nil {
		log.Crit(err.Error())
		os.Exit(1)
	}

	return m
}

func logToStdout(verbose bool) {
	if verbose {
		log.SetHandler(logger.LvlFilterHandler(
			logger.LvlDebug,
			logger.BypassHandler(),
		))
	} else {
		log.SetHandler(logger.LvlFilterHandler(
			logger.LvlInfo,
			logger.BypassHandler(),
		))
	}
}

func logToFile(filename string, verbose bool) {
	if verbose {
		log.SetHandler(logger.LvlFilterHandler(
			logger.LvlDebug,
			log15.Must.FileHandler(filename, log15.LogfmtFormat()),
		))
	} else {
		log.SetHandler(logger.LvlFilterHandler(
			logger.LvlInfo,
			log15.Must.FileHandler(filename, log15.LogfmtFormat()),
		))
	}
}
