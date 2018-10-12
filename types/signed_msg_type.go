package types

type SignedMsgType byte

const (
	// Types of votes
	PrevoteType   SignedMsgType = 0x01
	PrecommitType SignedMsgType = 0x02
	// To separate (canonicalized) Vote and Proposal messages:
	ProposalType SignedMsgType = 0x03
)

func IsVoteTypeValid(type_ byte) bool {
	switch type_ {
	case byte(PrevoteType):
		return true
	case byte(PrecommitType):
		return true
	default:
		return false
	}
}
