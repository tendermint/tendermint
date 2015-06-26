package types

import (
	acm "github.com/tendermint/tendermint/account"
)

type Validator struct {
	VotingPower int64
	PubKey      acm.PubKey
}
