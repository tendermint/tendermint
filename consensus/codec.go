package consensus

import (
	"fmt"

	"github.com/pkg/errors"
	amino "github.com/tendermint/go-amino"

	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/libs/bits"
	tmcons "github.com/tendermint/tendermint/proto/consensus"
	"github.com/tendermint/tendermint/types"
)

var cdc = amino.NewCodec()

func init() {
	RegisterMessages(cdc)
	RegisterWALMessages(cdc)
	types.RegisterBlockAmino(cdc)
}

func MsgToProto(msg Message) (*tmcons.Message, error) {
	if msg == nil {
		return nil, errors.New("consensus: message is nil")
	}
	var pb tmcons.Message

	switch msg := msg.(type) {
	case *NewRoundStepMessage:
		pb = tmcons.Message{
			Sum: &tmcons.Message_NewRoundStep{
				NewRoundStep: &tmcons.NewRoundStep{
					Height:                msg.Height,
					Round:                 msg.Round,
					Step:                  uint8(msg.Step),
					SecondsSinceStartTime: int64(msg.SecondsSinceStartTime),
					LastCommitRound:       msg.LastCommitRound,
				},
			},
		}
	case *NewValidBlockMessage:
		pbPartsHeader := msg.BlockPartsHeader.ToProto()
		pbBits := msg.BlockParts.ToProto()
		pb = tmcons.Message{
			Sum: &tmcons.Message_NewValidBlock{
				NewValidBlock: &tmcons.NewValidBlock{
					Height:           msg.Height,
					Round:            msg.Round,
					BlockPartsHeader: pbPartsHeader,
					BlockParts:       pbBits,
					IsCommit:         msg.IsCommit,
				},
			},
		}
	case *ProposalMessage:
		pbP := msg.Proposal.ToProto()
		pb = tmcons.Message{
			Sum: &tmcons.Message_Proposal{
				Proposal: &tmcons.Proposal{
					Proposal: *pbP,
				},
			},
		}
	case *ProposalPOLMessage:
		pbBits := msg.ProposalPOL.ToProto()
		pb = tmcons.Message{
			Sum: &tmcons.Message_ProposalPol{
				ProposalPol: &tmcons.ProposalPOL{
					Height:           msg.Height,
					ProposalPolRound: msg.ProposalPOLRound,
					ProposalPol:      *pbBits,
				},
			},
		}
	case *BlockPartMessage:
		parts, err := msg.Part.ToProto()
		if err != nil {
			return nil, fmt.Errorf("msg to proto error: %w", err)
		}
		pb = tmcons.Message{
			Sum: &tmcons.Message_BlockPart{
				BlockPart: &tmcons.BlockPart{
					Height: msg.Height,
					Round:  msg.Round,
					Part:   *parts,
				},
			},
		}
	case *VoteMessage:
		vote := msg.Vote.ToProto()
		pb = tmcons.Message{
			Sum: &tmcons.Message_Vote{
				Vote: &tmcons.Vote{
					Vote: vote,
				},
			},
		}
	case *HasVoteMessage:
		pb = tmcons.Message{
			Sum: &tmcons.Message_HasVote{
				HasVote: &tmcons.HasVote{
					Height: msg.Height,
					Round:  msg.Round,
					Type:   msg.Type,
					Index:  msg.Index,
				},
			},
		}
	case *VoteSetMaj23Message:
		bi := msg.BlockID.ToProto()
		pb = tmcons.Message{
			Sum: &tmcons.Message_VoteSetMaj23{
				VoteSetMaj23: &tmcons.VoteSetMaj23{
					Height:  msg.Height,
					Round:   msg.Round,
					Type:    msg.Type,
					BlockID: bi, // nil pointer deference
				},
			},
		}
	case *VoteSetBitsMessage:
		bi := msg.BlockID.ToProto()
		bits := msg.Votes.ToProto()

		if bits == nil {
			pb = tmcons.Message{
				Sum: &tmcons.Message_VoteSetBits{
					VoteSetBits: &tmcons.VoteSetBits{
						Height:  msg.Height,
						Round:   msg.Round,
						Type:    msg.Type,
						BlockID: bi,
					},
				},
			}
		}

		if bits != nil {
			pb = tmcons.Message{
				Sum: &tmcons.Message_VoteSetBits{
					VoteSetBits: &tmcons.VoteSetBits{
						Height:  msg.Height,
						Round:   msg.Round,
						Type:    msg.Type,
						BlockID: bi,
						Votes:   *bits,
					},
				},
			}
		}
	default:
		return nil, errors.New("consensus: message not recognized")
	}

	return &pb, nil
}
func MsgFromProto(msg tmcons.Message) (*Message, error) {
	var pb Message

	switch msg := msg.Sum.(type) {
	case *tmcons.Message_NewRoundStep:
		pb = &NewRoundStepMessage{
			Height:                msg.NewRoundStep.Height,
			Round:                 msg.NewRoundStep.Round,
			Step:                  cstypes.RoundStepType(msg.NewRoundStep.Step),
			SecondsSinceStartTime: int(msg.NewRoundStep.SecondsSinceStartTime),
			LastCommitRound:       msg.NewRoundStep.LastCommitRound,
		}
	case *tmcons.Message_NewValidBlock:
		pbPartsHeader := new(types.PartSetHeader)
		pbPartsHeader.FromProto(msg.NewValidBlock.BlockPartsHeader)

		pbBits := new(bits.BitArray)
		pbBits.FromProto(msg.NewValidBlock.BlockParts)

		pb = &NewValidBlockMessage{
			Height:           msg.NewValidBlock.Height,
			Round:            msg.NewValidBlock.Round,
			BlockPartsHeader: *pbPartsHeader,
			BlockParts:       pbBits,
			IsCommit:         msg.NewValidBlock.IsCommit,
		}
	case *tmcons.Message_Proposal:
		pbP := new(types.Proposal)
		pbP.FromProto(&msg.Proposal.Proposal)

		pb = &ProposalMessage{
			Proposal: pbP,
		}
	case *tmcons.Message_ProposalPol:
		pbBits := new(bits.BitArray)
		pbBits.FromProto(&msg.ProposalPol.ProposalPol)
		pb = &ProposalPOLMessage{
			Height:           msg.ProposalPol.Height,
			ProposalPOLRound: msg.ProposalPol.ProposalPolRound,
			ProposalPOL:      pbBits,
		}
	case *tmcons.Message_BlockPart:
		parts, err := msg.Part.ToProto()
		if err != nil {
			return nil, fmt.Errorf("msg to proto error: %w", err)
		}
		pb = tmcons.Message{
			Sum: &tmcons.Message_BlockPart{
				BlockPart: &tmcons.BlockPart{
					Height: msg.Height,
					Round:  msg.Round,
					Part:   *parts,
				},
			},
		}
	case *tmcons.Message_Vote:
		vote := msg.Vote.ToProto()
		pb = tmcons.Message{
			Sum: &tmcons.Message_Vote{
				Vote: &tmcons.Vote{
					Vote: vote,
				},
			},
		}
	case *HasVoteMessage:
		pb = tmcons.Message{
			Sum: &tmcons.Message_HasVote{
				HasVote: &tmcons.HasVote{
					Height: msg.Height,
					Round:  msg.Round,
					Type:   msg.Type,
					Index:  msg.Index,
				},
			},
		}
	case *tmcons.Message_VoteSetMaj23:
		bi := msg.BlockID.ToProto()
		pb = tmcons.Message{
			Sum: &tmcons.Message_VoteSetMaj23{
				VoteSetMaj23: &tmcons.VoteSetMaj23{
					Height:  msg.Height,
					Round:   msg.Round,
					Type:    msg.Type,
					BlockID: bi, // nil pointer deference
				},
			},
		}
	case *tmcons.Message_VoteSetBits:
		bi := msg.BlockID.ToProto()
		bits := msg.Votes.ToProto()

		if bits == nil {
			pb = tmcons.Message{
				Sum: &tmcons.Message_VoteSetBits{
					VoteSetBits: &tmcons.VoteSetBits{
						Height:  msg.Height,
						Round:   msg.Round,
						Type:    msg.Type,
						BlockID: bi,
					},
				},
			}
		}

		if bits != nil {
			pb = tmcons.Message{
				Sum: &tmcons.Message_VoteSetBits{
					VoteSetBits: &tmcons.VoteSetBits{
						Height:  msg.Height,
						Round:   msg.Round,
						Type:    msg.Type,
						BlockID: bi,
						Votes:   *bits,
					},
				},
			}
		}
	default:
		return nil, errors.New("consensus: message not recognized")
	}

	return &pb, nil
}
