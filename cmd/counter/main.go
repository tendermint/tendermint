package main

import (
	"flag"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/example/counter"
	"github.com/tendermint/tmsp/server"
	"github.com/tendermint/tmsp/types"
)

func main() {

	addrPtr := flag.String("addr", "tcp://0.0.0.0:46658", "Listen address")
	grpcPtr := flag.String("tmsp", "socket", "TMSP server: socket | grpc")
	serialPtr := flag.Bool("serial", false, "Enforce incrementing (serial) txs")
	flag.Parse()
	app := counter.NewCounterApplication(*serialPtr)

	// Start the listener
	switch *grpcPtr {
	case "socket":
		_, err := server.NewServer(*addrPtr, app)
		if err != nil {
			Exit(err.Error())
		}
	case "grpc":
		_, err := server.NewGRPCServer(*addrPtr, types.NewGRPCApplication(app))
		if err != nil {
			Exit(err.Error())
		}
	default:
		Exit(Fmt("Unknown server type %s", *grpcPtr))
	}

	// Wait forever
	TrapSignal(func() {
		// Cleanup
	})

}
