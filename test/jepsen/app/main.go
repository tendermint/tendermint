package app

import (
	"flag"
	"fmt"
	"os"

	"github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
)

var (
	logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))

	dbName string
	laddr  string
)

func init() {
	flag.StringVar(&dbName, "dbname", "", "database name")
	flag.StringVar(&laddr, "laddr", "unix://data.sock", "listen address")
}

func main() {
	flag.Parse()

	app := NewMerkleEyesApp(dbName, 0)
	srv, err := server.NewServer(laddr, "socket", app)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't create server: %v", err)
		os.Exit(-1)
	}
	srv.SetLogger(logger.With("module", "abci-server"))

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "can't start server: %v", err)
		os.Exit(-1)
	}

	// Stop upon receiving SIGTERM or CTRL-C.
	tmos.TrapSignal(logger, func() {
		// Cleanup
		srv.Stop()
		app.CloseDB()
	})

	// Run forever.
	select {}
}
