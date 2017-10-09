package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	cs "github.com/tendermint/tendermint/consensus"
)

func main() {
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

		json, err := json.Marshal(msg)
		if err != nil {
			panic(fmt.Errorf("failed to marshal msg: %v", err))
		}

		os.Stdout.Write(json)
		os.Stdout.Write([]byte("\n"))
	}
}
