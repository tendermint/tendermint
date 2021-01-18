package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/proto"

	bc "github.com/tendermint/tendermint/blockchain"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blockchain"
	types "github.com/tendermint/tendermint/proto/tendermint/types"
)

// Creates "corpus" directory in the current path and writes corpus data.

func main() {
	// create "corpus" dir if not exists
	corpusDir := filepath.Join(".", "corpus")
	if err := os.Mkdir(corpusDir, 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("can't create %s: %v", corpusDir, err)
	}

	// write corpus
	messages := []struct {
		filename string
		msg      proto.Message
	}{
		{"status-request", &bcproto.StatusRequest{}},
		{"status-response", &bcproto.StatusResponse{Base: 0, Height: 200}},
		{"block-request", &bcproto.BlockRequest{Height: 100}},
		{"block-response", &bcproto.BlockResponse{Block: &types.Block{}}},
		{"no-block-response", &bcproto.NoBlockResponse{Height: 50}},
	}

	for _, msg := range messages {
		bz, err := bc.EncodeMsg(msg.msg)
		if err != nil {
			log.Fatalf("can't encode %v: %v", msg, err)
		}

		filename := filepath.Join(corpusDir, msg.filename)

		err = ioutil.WriteFile(filename, bz, 0644)
		if err != nil {
			log.Fatalf("can't write msg bytes to %s: %v", filename, err)
		}

		log.Printf("wrote %s", filename)
	}
}
