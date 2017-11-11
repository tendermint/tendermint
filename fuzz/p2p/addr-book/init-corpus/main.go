package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/tendermint/tendermint/p2p"
)

func main() {
	root := flag.String("root", ".", `where the "corpus" directory will live`)
	flag.Parse()

	initCorpa(*root)
}

func initCorpa(baseDir string) {
	log.SetFlags(0)
	corpusDir := filepath.Join(baseDir, "corpus")
	if err := os.MkdirAll(corpusDir, 0755); err != nil {
		log.Fatalf("Creating %q err: %v", corpusDir, err)
	}

	corpa := []*p2p.NetAddress{
		{IP: net.IPv4(0, 0, 0, 0), Port: 0},
		{IP: net.IPv4(127, 0, 0, 0), Port: 80},
		{IP: net.IPv4(213, 87, 10, 200), Port: 8808},
		{IP: net.IPv4(111, 111, 111, 111), Port: 46658},
	}

	for i, corpus := range corpa {
		outPath := filepath.Join(corpusDir, fmt.Sprintf("%d.json", i))
		f, err := os.Create(outPath)
		if err != nil {
			log.Printf("#%d: path:(%q) err: %v", i, outPath, err)
			continue
		}
		err = json.NewEncoder(f).Encode(corpus)
		f.Close()
		if err == nil {
			log.Printf("Successfully wrote %q", outPath)
		} else {
			log.Printf("JSON.Encode: #%d: path:(%q) %v", i, outPath, err)
		}
	}
}
