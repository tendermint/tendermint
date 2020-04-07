package encoding

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/crypto/ed25519"
)

func TestFuzzPubKeyProto10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	pk := &ed25519.PubKey{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(pk)
		pkp, err := PubKeyToProto(*pk)
		assert.NoError(t, err)
		_, err = PubKeyFromProto(pkp)
		assert.NoError(t, err)
	}
}
func TestFuzzPrivKeyProto10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	pk := &ed25519.PrivKey{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(pk)
		pkp, err := PrivKeyToProto(*pk)
		_, err = PrivKeyFromProto(pkp)
		assert.NoError(t, err)
	}
}
