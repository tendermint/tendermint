package types

import tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

// IsVoteTypeValid returns true if t is a valid vote type.
func IsVoteTypeValid(t tmproto.SignedMsgType) bool {
	switch t {
	case tmproto.PrevoteType, tmproto.PrecommitType:
		return true
	default:
		return false
	}
}
