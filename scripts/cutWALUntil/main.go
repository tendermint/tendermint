/*
	cutWALUntil is a small utility for cutting a WAL until the given height
	(inclusively). Note it does not include last cs.EndHeightMessage.

	Usage:
			cutWALUntil <path-to-wal> height-to-stop <output-wal>
*/
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	cs "github.com/tendermint/tendermint/consensus"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("3 arguments required: <path-to-wal> <height-to-stop> <output-wal>")
		os.Exit(1)
	}

	var heightToStop uint64
	var err error
	if heightToStop, err = strconv.ParseUint(os.Args[2], 10, 64); err != nil {
		panic(fmt.Errorf("failed to parse height: %v", err))
	}

	in, err := os.Open(os.Args[1])
	if err != nil {
		panic(fmt.Errorf("failed to open input WAL file: %v", err))
	}
	defer in.Close()

	out, err := os.Create(os.Args[3])
	if err != nil {
		panic(fmt.Errorf("failed to open output WAL file: %v", err))
	}
	defer out.Close()

	enc := cs.NewWALEncoder(out)
	dec := cs.NewWALDecoder(in)

	for {
		msg, err := dec.Decode()
		if err == io.EOF {
			break
		} else if err != nil {
			panic(fmt.Errorf("failed to decode msg: %v", err))
		}

		if m, ok := msg.Msg.(cs.EndHeightMessage); ok {
			if m.Height == heightToStop {
				break
			}
		}

		err = enc.Encode(msg)
		if err != nil {
			panic(fmt.Errorf("failed to encode msg: %v", err))
		}
	}
}
