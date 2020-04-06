package encoding

import (
	"fmt"
	"testing"

	fuzz "github.com/google/gofuzz"

	"github.com/tendermint/tendermint/crypto/ed25519"
)

func TestFuzzPubKeyToProto10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	pk := &ed25519.PubKey{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(pk)
		pkp, err := PubKeyToProto(pk)
		fmt.Println(err, "1")
		_, err = PubKeyFromProto(pkp)
		fmt.Println(err, "2")
	}
}
