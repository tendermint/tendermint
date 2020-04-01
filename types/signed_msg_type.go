package types

import (
	prototypes "github.com/tendermint/tendermint/proto/types"
)

// IsVoteTypeValid returns true if t is a valid vote type.
func IsVoteTypeValid(t prototypes.SignedMsgType) bool {
	switch t {
	case prototypes.PrevoteType, prototypes.PrecommitType:
		return true
	default:
		return false
	}
}
