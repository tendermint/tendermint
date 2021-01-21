// nolint: gosec
package addr

import (
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/pex"
)

var addrBook = pex.NewAddrBook("./testdata/addrbook.json", true)

func Fuzz(data []byte) int {
	addr := new(p2p.NetAddress)
	if err := json.Unmarshal(data, addr); err != nil {
		return -1
	}

	// Fuzz AddAddress.
	err := addrBook.AddAddress(addr, addr)
	if err != nil {
		return 0
	}

	// Also, make sure PickAddress always returns a non-nil address.
	bias := rand.Intn(100)
	if p := addrBook.PickAddress(bias); p == nil {
		panic(fmt.Sprintf("picked a nil address (bias: %d, addrBook size: %v)",
			bias, addrBook.Size()))
	}

	return 1
}
