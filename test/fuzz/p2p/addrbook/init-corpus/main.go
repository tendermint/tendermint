package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/p2p"
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
		{ID: p2p.NodeIDFromPubKey(privKey.PubKey()), IP: net.IPv4(0, 0, 0, 0), Port: 0},
		{ID: p2p.NodeIDFromPubKey(privKey.PubKey()), IP: net.IPv4(127, 0, 0, 0), Port: 80},
		{ID: p2p.NodeIDFromPubKey(privKey.PubKey()), IP: net.IPv4(213, 87, 10, 200), Port: 8808},
		{ID: p2p.NodeIDFromPubKey(privKey.PubKey()), IP: net.IPv4(111, 111, 111, 111), Port: 26656},
		{ID: p2p.NodeIDFromPubKey(privKey.PubKey()), IP: net.ParseIP("2001:db8::68"), Port: 26656},
	}

	for i, addr := range addrs {
		outPath := filepath.Join(corpusDir, fmt.Sprintf("%d.json", i))
		f, err := os.Create(outPath)
		if err != nil {
			log.Printf("#%d: can't create %q: %v", i, outPath, err)
			continue
		}
		defer f.Close()

		err = json.NewEncoder(f).Encode(addr)
		if err == nil {
			log.Printf("Successfully wrote %q", outPath)
		} else {
			log.Printf("#%d: can't encode %v: %v", i, addr, err)
		}
	}
}
