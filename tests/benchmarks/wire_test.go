package benchmarks

import (
	"bytes"
	"testing"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tmsp/types"
)

func BenchmarkRequestWire(b *testing.B) {
	b.StopTimer()
	var bz = make([]byte, 1024)
	copy(bz, []byte{1, 9, 0x01, 1, 6, 34, 34, 34, 34, 34, 34})
	var buf = bytes.NewBuffer(bz)
	var req types.Request
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		{
			buf = bytes.NewBuffer(bz)
			var n int
			var err error
			wire.ReadBinaryPtrLengthPrefixed(&req, buf, 0, &n, &err)
			if err != nil {
				Exit(err.Error())
				return
			}
		}
		{
			buf = bytes.NewBuffer(bz)
			var n int
			var err error
			wire.WriteBinaryLengthPrefixed(struct{ types.Request }{req}, buf, &n, &err)
			if err != nil {
				Exit(err.Error())
				return
			}

		}
	}

}
