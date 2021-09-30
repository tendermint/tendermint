package p2p

import (
	"fmt"
	mrand "math/rand"

	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/types"
)

//------------------------------------------------

// nolint:gosec // G404: Use of weak random number generator
func CreateRoutableAddr() (addr string, netAddr *NetAddress) {
	for {
		var err error
		addr = fmt.Sprintf("%X@%v.%v.%v.%v:26656",
			tmrand.Bytes(20),
			mrand.Int()%256,
			mrand.Int()%256,
			mrand.Int()%256,
			mrand.Int()%256)
		netAddr, err = types.NewNetAddressString(addr)
		if err != nil {
			panic(err)
		}
		if netAddr.Routable() {
			break
		}
	}
	return
}
