// nolint: gosec
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/types"
)

func main() {
	baseDir := flag.String("base", ".", `where the "corpus" directory will live`)
	flag.Parse()

	initCorpus(*baseDir)
}

func initCorpus(baseDir string) {
	log.SetFlags(0)

	// create "corpus" directory
	corpusDir := filepath.Join(baseDir, "corpus")
	if err := os.MkdirAll(corpusDir, 0755); err != nil {
		log.Fatalf("Creating %q err: %v", corpusDir, err)
	}

	// create corpus
	privKey := ed25519.GenPrivKey()
	addrs := []*p2p.NetAddress{
		{ID: types.NodeIDFromPubKey(privKey.PubKey()), IP: net.IPv4(0, 0, 0, 0), Port: 0},
		{ID: types.NodeIDFromPubKey(privKey.PubKey()), IP: net.IPv4(127, 0, 0, 0), Port: 80},
		{ID: types.NodeIDFromPubKey(privKey.PubKey()), IP: net.IPv4(213, 87, 10, 200), Port: 8808},
		{ID: types.NodeIDFromPubKey(privKey.PubKey()), IP: net.IPv4(111, 111, 111, 111), Port: 26656},
		{ID: types.NodeIDFromPubKey(privKey.PubKey()), IP: net.ParseIP("2001:db8::68"), Port: 26656},
	}

	for i, addr := range addrs {
		filename := filepath.Join(corpusDir, fmt.Sprintf("%d.json", i))

		bz, err := json.Marshal(addr)
		if err != nil {
			log.Fatalf("can't marshal %v: %v", addr, err)
		}

		if err := ioutil.WriteFile(filename, bz, 0644); err != nil {
			log.Fatalf("can't write %v to %q: %v", addr, filename, err)
		}

		log.Printf("wrote %q", filename)
	}
}
