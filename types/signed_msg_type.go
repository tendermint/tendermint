package types

type SignedMsgType byte

const (
	// Types of votes
	PrevoteType   SignedMsgType = 0x01
	PrecommitType SignedMsgType = 0x02
	// To separate (canonicalized) Vote and Proposal messages:
	ProposalType SignedMsgType = 0x10
)

func IsVoteTypeValid(type_ SignedMsgType) bool {
	switch type_ {
	case PrevoteType:
		return true
	case PrecommitType:
		return true
	default:
		return false
	}
}
