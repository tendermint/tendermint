package main

import (
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"net"

	"github.com/tendermint/tendermint/p2p"
)

var addrBook = p2p.NewAddrBook("./testdata/addrbook.json", true)

// random octet
func ro() byte { return byte(rand.Intn(256)) }

func randAddress() *p2p.NetAddress {
	return &p2p.NetAddress{
		IP:   net.IPv4(ro(), ro(), ro(), ro()),
		Port: uint16(rand.Intn(1 << 16)),
	}
}

func main() {
	srcStr := flag.String("src", "", "the JSON for the source")
	biasVar := flag.Int("bias", 412, "the bias to use in picking the address")
	addrStr := flag.String("addr", "", "the JSON for the address")
	flag.Parse()

	srcAddr := new(p2p.NetAddress)
	if err := json.Unmarshal([]byte(*srcStr), srcAddr); err != nil {
		log.Fatal(err)
	}
	addr := new(p2p.NetAddress)
	if err := json.Unmarshal([]byte(*addrStr), addr); err != nil {
		log.Fatal(err)
	}
	addrBook.AddAddress(addr, srcAddr)
	bias := rand.Intn(*biasVar)
	if addr := addrBook.PickAddress(bias); addr == nil {
		panic("picked a nil address")
	}
}
