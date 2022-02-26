// Program estream is a manual testing tool for polling the event stream
// of a running Tendermint consensus node.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/tendermint/tendermint/rpc/client/eventstream"
	rpcclient "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/rpc/coretypes"
)

var (
	query      = flag.String("query", "", "Filter query")
	batchSize  = flag.Int("batch", 0, "Batch size")
	resumeFrom = flag.String("resume", "", "Resume cursor")
	numItems   = flag.Int("count", 0, "Number of items to read (0 to stream)")
	waitTime   = flag.Duration("poll", 0, "Long poll interval")
	rpcAddr    = flag.String("addr", "http://localhost:26657", "RPC service address")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options]

Connect to the Tendermint node whose RPC service is at -addr, and poll for events
matching the specified -query. If no query is given, all events are fetched.
The resulting event data are written to stdout as JSON.

Use -resume to pick up polling from a previously-reported event cursor.
Use -count to stop polling after a certain number of events has been reported.
Use -batch to override the default request batch size.
Use -poll to override the default long-polling interval.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	cli, err := rpcclient.New(*rpcAddr)
	if err != nil {
		log.Fatalf("RPC client: %v", err)
	}
	stream := eventstream.New(cli, *query, &eventstream.StreamOptions{
		BatchSize:  *batchSize,
		ResumeFrom: *resumeFrom,
		WaitTime:   *waitTime,
	})

	// Shut down cleanly on SIGINT.  Don't attempt clean shutdown for other
	// fatal signals.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var nr int
	if err := stream.Run(ctx, func(itm *coretypes.EventItem) error {
		nr++
		bits, err := json.Marshal(itm)
		if err != nil {
			return err
		}
		fmt.Println(string(bits))
		if *numItems > 0 && nr >= *numItems {
			return eventstream.ErrStopRunning
		}
		return nil
	}); err != nil {
		log.Fatalf("Stream failed: %v", err)
	}
}
