package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

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
