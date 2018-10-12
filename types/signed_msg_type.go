package types

// SignedMsgType is a type of signed message in the consensus.
type SignedMsgType byte

const (
	// Votes
	PrevoteType   SignedMsgType = 0x01
	PrecommitType SignedMsgType = 0x02

	// Proposals
	ProposalType SignedMsgType = 0x20

	// Heartbeat
	HeartbeatType SignedMsgType = 0x30
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
