package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	cs "github.com/tendermint/tendermint/consensus"
)

func main() {
	var heightToStop uint64
	var err error
	if heightToStop, err = strconv.ParseUint(os.Args[2], 10, 64); err != nil {
		panic(fmt.Errorf("failed to parse height: %v (format: cutWALUntil in heightToStop out)", err))
	}

	in, err := os.Open(os.Args[1])
	if err != nil {
		panic(fmt.Errorf("failed to open WAL file: %v (format: cutWALUntil in heightToStop out)", err))
	}
	defer in.Close()

	out, err := os.Create(os.Args[3])
	if err != nil {
		panic(fmt.Errorf("failed to open WAL file: %v (format: cutWALUntil in heightToStop out)", err))
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
