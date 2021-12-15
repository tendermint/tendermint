package consensus

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
)

// Wrap implements the p2p Wrapper interface and wraps a consensus proto message.
func (m *Message) Wrap(pb proto.Message) error {
	switch msg := pb.(type) {
	case *NewRoundStep:
		m.Sum = &Message_NewRoundStep{NewRoundStep: msg}

	case *NewValidBlock:
		m.Sum = &Message_NewValidBlock{NewValidBlock: msg}

	case *Proposal:
		m.Sum = &Message_Proposal{Proposal: msg}

	case *ProposalPOL:
		m.Sum = &Message_ProposalPol{ProposalPol: msg}

	case *BlockPart:
		m.Sum = &Message_BlockPart{BlockPart: msg}

	case *Vote:
		m.Sum = &Message_Vote{Vote: msg}

	case *HasVote:
		m.Sum = &Message_HasVote{HasVote: msg}

	case *VoteSetMaj23:
		m.Sum = &Message_VoteSetMaj23{VoteSetMaj23: msg}

	case *VoteSetBits:
		m.Sum = &Message_VoteSetBits{VoteSetBits: msg}

	default:
		return fmt.Errorf("unknown message: %T", msg)
	}

	return nil
}

// Unwrap implements the p2p Wrapper interface and unwraps a wrapped consensus
// proto message.
func (m *Message) Unwrap() (proto.Message, error) {
	switch msg := m.Sum.(type) {
	case *Message_NewRoundStep:
		return m.GetNewRoundStep(), nil

	case *Message_NewValidBlock:
		return m.GetNewValidBlock(), nil

	case *Message_Proposal:
		return m.GetProposal(), nil

	case *Message_ProposalPol:
		return m.GetProposalPol(), nil

	case *Message_BlockPart:
		return m.GetBlockPart(), nil

	case *Message_Vote:
		return m.GetVote(), nil

	case *Message_HasVote:
		return m.GetHasVote(), nil

	case *Message_VoteSetMaj23:
		return m.GetVoteSetMaj23(), nil

	case *Message_VoteSetBits:
		return m.GetVoteSetBits(), nil

	default:
		return nil, fmt.Errorf("unknown message: %T", msg)
	}
}
