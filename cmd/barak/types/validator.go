package types

import (
	acm "github.com/eris-ltd/tendermint/account"
)

type Validator struct {
	VotingPower int64
	PubKey      acm.PubKey
}
