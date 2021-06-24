package pex

import (
	"testing"

	"github.com/tendermint/tendermint/types"
)

func BenchmarkAddrBook_hash(b *testing.B) {
	book := &addrBook{
		ourAddrs:          make(map[string]struct{}),
		privateIDs:        make(map[types.NodeID]struct{}),
		addrLookup:        make(map[types.NodeID]*knownAddress),
		badPeers:          make(map[types.NodeID]*knownAddress),
		filePath:          "",
		routabilityStrict: true,
	}
	book.init()
	msg := []byte(`foobar`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = book.hash(msg)
	}
}
