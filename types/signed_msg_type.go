package types

import "github.com/tendermint/tendermint/types/proto"

// // SignedMsgType is a type of signed message in the consensus.
// type SignedMsgType byte

// const (
// 	// Votes
// 	PrevoteType   SignedMsgType = 0x01
// 	PrecommitType SignedMsgType = 0x02

// 	// Proposals
// 	ProposalType SignedMsgType = 0x20
// )

// IsVoteTypeValid returns true if t is a valid vote type.
func IsVoteTypeValid(t proto.SignedMsgType) bool {
	switch t {
	case proto.SIGNED_MSG_TYPE_PREVOTE_TYPE, proto.SIGNED_MSG_TYPE_PRECOMMIT_TYPE:
		return true
	default:
		return false
	}
}
