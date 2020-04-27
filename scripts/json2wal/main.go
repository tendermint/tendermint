/*
	json2wal converts JSON file to binary WAL file.

	Usage:
			json2wal <path-to-JSON>  <path-to-wal>
*/

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	amino "github.com/tendermint/go-amino"

	cs "github.com/tendermint/tendermint/consensus"
	tmencode "github.com/tendermint/tendermint/libs/encode"
	tmcons "github.com/tendermint/tendermint/proto/consensus"
	"github.com/tendermint/tendermint/types"
)

var cdc = amino.NewCodec()

func init() {
	types.RegisterBlockAmino(cdc)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "missing arguments: Usage:json2wal <path-to-JSON>  <path-to-wal>")
		os.Exit(1)
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(fmt.Errorf("failed to open WAL file: %v", err))
	}
	defer f.Close()

	walFile, err := os.OpenFile(os.Args[2], os.O_EXCL|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		panic(fmt.Errorf("failed to open WAL file: %v", err))
	}
	defer walFile.Close()

	// the length of tendermint/wal/MsgInfo in the wal.json may exceed the defaultBufSize(4096) of bufio
	// because of the byte array in BlockPart
	// leading to unmarshal error: unexpected end of JSON input
	br := bufio.NewReaderSize(f, int(2*types.BlockPartSizeBytes))
	dec := cs.NewWALEncoder(walFile)

	for {
		msgJSON, _, err := br.ReadLine()
		if err == io.EOF {
			break
		} else if err != nil {
			panic(fmt.Errorf("failed to read file: %v", err))
		}
		// ignore the ENDHEIGHT in json.File
		if strings.HasPrefix(string(msgJSON), "ENDHEIGHT") {
			continue
		}

		// var msg tmcons.TimedWALMessage
		pb, err := tmencode.UnmarshalJSON(msgJSON)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal json: %v", err))
		}

		if msg, ok := pb.(*tmcons.TimedWALMessage); ok {
			wal, err := cs.WALFromProto(msg.Msg)
			if err != nil {
				panic("error on transforming WAL message from proto")
			}
			walMsg := cs.TimedWALMessage{
				Time: msg.Time,
				Msg:  wal,
			}

			err = dec.Encode(&walMsg)
			if err != nil {
				panic(fmt.Errorf("failed to encode msg: %v", err))
			}
		} else {
			panic("failed to identify message")
		}
	}
}
