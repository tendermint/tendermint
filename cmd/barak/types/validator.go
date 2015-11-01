package types

import (
	"github.com/tendermint/go-crypto"
)

type Validator struct {
	VotingPower int64
	PubKey      crypto.PubKey
}
