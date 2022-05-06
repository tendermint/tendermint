package sr25519

import (
	"github.com/tendermint/tendermint/internal/jsontypes"
)

const (
	PrivKeyName = "tendermint/PrivKeySr25519"
	PubKeyName  = "tendermint/PubKeySr25519"
)

func init() {
	jsontypes.MustRegister(PubKey{})
	jsontypes.MustRegister(PrivKey{})
}
