package addr

import (
	"encoding/json"
	"fmt"
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

func Fuzz(data []byte) int {
	srcAddr := new(p2p.NetAddress)
	if err := json.Unmarshal(data, srcAddr); err != nil {
		return -1
	}
	addr := randAddress()
	addrBook.AddAddress(addr, srcAddr)
	bias := rand.Intn(1000)
	if p := addrBook.PickAddress(bias); p == nil {
		blob, _ := json.Marshal(addr)
		panic(fmt.Sprintf("picked a nil address with bias: %d, netAddress: %s addr: %s", bias, data, blob))
	}
	return 0
}
