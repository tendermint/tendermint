package types

import (
	"testing"

	fuzz "github.com/google/gofuzz"
)

func TestFuzzHeaderProto_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	header := &Header{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(header)
		hp := header.ToProto()

		h := Header{}
		_ = h.FromProto(*hp)
	}
}
