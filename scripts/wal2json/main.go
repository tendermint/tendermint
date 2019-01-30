/*
	wal2json converts binary WAL file to JSON.

	Usage:
			wal2json <path-to-wal>
*/

package main

import (
	"fmt"
	"io"
	"os"

	amino "github.com/tendermint/go-amino"
	cs "github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/types"
)

var cdc = amino.NewCodec()

func init() {
	cs.RegisterConsensusMessages(cdc)
	cs.RegisterWALMessages(cdc)
	types.RegisterBlockAmino(cdc)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("missing one argument: <path-to-wal>")
		os.Exit(1)
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(fmt.Errorf("failed to open WAL file: %v", err))
	}
	defer f.Close()

	dec := cs.NewWALDecoder(f)
	for {
		msg, err := dec.Decode()
		if err == io.EOF {
			break
		} else if err != nil {
			panic(fmt.Errorf("failed to decode msg: %v", err))
		}

		json, err := cdc.MarshalJSON(msg)
		if err != nil {
			panic(fmt.Errorf("failed to marshal msg: %v", err))
		}

		_, err = os.Stdout.Write(json)
		if err == nil {
			_, err = os.Stdout.Write([]byte("\n"))
		}
		if err == nil {
			if end, ok := msg.Msg.(cs.EndHeightMessage); ok {
				_, err = os.Stdout.Write([]byte(fmt.Sprintf("ENDHEIGHT %d\n", end.Height))) // nolint: errcheck, gas
			}
		}
		if err != nil {
			fmt.Println("Failed to write message", err)
			os.Exit(1)
		}
	}
}
