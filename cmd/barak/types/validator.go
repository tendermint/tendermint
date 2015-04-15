package types

import (
	acm "github.com/tendermint/tendermint/account"
)

type Validator struct {
	VotingPower uint64
	PubKey      acm.PubKey
}
