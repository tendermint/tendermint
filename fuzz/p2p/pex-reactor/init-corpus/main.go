package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/p2p"
)

func main() {
	rootDirVar := flag.String("root", ".", `the directory in which the "corpus", "testdata" directories are`)
	flag.Parse()
	rootDir := *rootDirVar
	if rootDir == "" {
		rootDir = "."
	}
	InitCorpus(rootDir)
}

func InitCorpus(rootDir string) {
	log.SetFlags(0)
	sizes := []int{0, 1, 2, 17, 5, 31}

	// Make the PRNG predictable
	rand.Seed(10)

	for _, n := range sizes {
		var addrs []*p2p.NetAddress
		for i := 0; i < n; i++ {
			addr := fmt.Sprintf("%v.%v.%v.%v:46656", rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%256)
			netAddr, _ := p2p.NewNetAddressString(addr)
			addrs = append(addrs, netAddr)
		}
		msg := wire.BinaryBytes(struct{ p2p.PexMessage }{&pexAddrsMessage{Addrs: addrs}})
		name := filepath.Join(rootDir, "corpus", fmt.Sprintf("%d", n))
		f, err := os.Create(name)
		if err == nil {
			f.Write(msg)
			if err := f.Close(); err == nil {
				log.Printf("Successfully generated corpus file: %q", name)
			} else {
				log.Printf("Failed to generate corpus file: %q err: %v", name, err)
			}
		} else {
			log.Printf("%q err: %v\n", name, err)
		}
	}
}

// Start of go-wire related code
// This go-wire code is copied from:
//  https://github.com/tendermint/tendermint/blob/128e2a1d9e88eeb0aa7645973940b23eb7a14803/p2p/pex_reactor.go#L315-L328
const (
	msgTypeRequest = byte(0x01)
	msgTypeAddrs   = byte(0x02)
)

var _ = wire.RegisterInterface(
	struct{ p2p.PexMessage }{},
	wire.ConcreteType{&pexRequestMessage{}, msgTypeRequest},
	wire.ConcreteType{&pexAddrsMessage{}, msgTypeAddrs},
)

type pexRequestMessage struct{}
type pexAddrsMessage struct {
	Addrs []*p2p.NetAddress
}

// End of go-wire related code
