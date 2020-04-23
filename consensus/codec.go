package consensus

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	amino "github.com/tendermint/go-amino"

	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/libs/bits"
	"github.com/tendermint/tendermint/p2p"
	tmcons "github.com/tendermint/tendermint/proto/consensus"
	tmproto "github.com/tendermint/tendermint/proto/types"
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
		return nil, fmt.Errorf("consensus: message not recognized, %T", msg)
	}

	return &pb, nil
}
func MsgFromProto(msg tmcons.Message) (Message, error) {
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
		parts := new(types.Part)
		if err := parts.FromProto(&msg.BlockPart.Part); err != nil {
			return nil, fmt.Errorf("blockpart msg to proto error: %w", err)
		}
		pb = &BlockPartMessage{
			Height: msg.BlockPart.Height,
			Round:  msg.BlockPart.Round,
			Part:   parts,
		}
	case *tmcons.Message_Vote:
		vote := new(types.Vote)
		if err := vote.FromProto(msg.Vote.Vote); err != nil {
			return nil, fmt.Errorf("vote msg to proto error: %w", err)
		}

		pb = &VoteMessage{
			Vote: vote,
		}
	case *tmcons.Message_HasVote:
		pb = &HasVoteMessage{
			Height: msg.HasVote.Height,
			Round:  msg.HasVote.Round,
			Type:   msg.HasVote.Type,
			Index:  msg.HasVote.Index,
		}
	case *tmcons.Message_VoteSetMaj23:
		bi := new(types.BlockID)
		if err := bi.FromProto(&msg.VoteSetMaj23.BlockID); err != nil {
			return nil, fmt.Errorf("voteSetMaj23 msg to proto error: %w", err)
		}
		pb = &VoteSetMaj23Message{
			Height:  msg.VoteSetMaj23.Height,
			Round:   msg.VoteSetMaj23.Round,
			Type:    msg.VoteSetMaj23.Type,
			BlockID: *bi, // nil pointer deference
		}
	case *tmcons.Message_VoteSetBits:
		bi := new(types.BlockID)
		if err := bi.FromProto(&msg.VoteSetBits.BlockID); err != nil {
			return nil, fmt.Errorf("voteSetBits msg to proto error: %w", err)
		}
		bits := new(bits.BitArray)
		bits.FromProto(&msg.VoteSetBits.Votes)

		pb = &VoteSetBitsMessage{
			Height:  msg.VoteSetBits.Height,
			Round:   msg.VoteSetBits.Round,
			Type:    msg.VoteSetBits.Type,
			BlockID: *bi,
			Votes:   bits,
		}
	default:
		return nil, fmt.Errorf("consensus: message not recognized: %T", msg)
	}

	if err := pb.ValidateBasic(); err != nil {
		return nil, err
	}

	return pb, nil
}

//MustEncode takes the reactors msg, makes it proto and marshals it
// this mimics `MustMarshalBinaryBare` in that is panics on error
func MustEncode(msg Message) []byte {
	pb, err := MsgToProto(msg)
	if err != nil {
		panic(err)
	}
	enc, err := proto.Marshal(pb)
	if err != nil {
		panic(err)
	}
	return enc
}

// WALToProto takes a consensus wal message and return a proto walMessage and error
func WALToProto(msg WALMessage) (*tmcons.WALMessage, error) {
	var pb tmcons.WALMessage

	switch msg := msg.(type) {
	case types.EventDataRoundState:
		pb = tmcons.WALMessage{
			Sum: &tmcons.WALMessage_EventDataRoundState{
				EventDataRoundState: &tmproto.EventDataRoundState{
					Height: msg.Height,
					Round:  msg.Round,
					Step:   msg.Step,
				},
			},
		}
	case msgInfo:
		consMsg, err := MsgToProto(msg.Msg)
		if err != nil {
			return nil, err
		}
		pb = tmcons.WALMessage{
			Sum: &tmcons.WALMessage_MsgInfo{
				MsgInfo: &tmcons.MsgInfo{
					Msg:    *consMsg,
					PeerId: string(msg.PeerID),
				},
			},
		}
	case timeoutInfo:
		pb = tmcons.WALMessage{
			Sum: &tmcons.WALMessage_TimeoutInfo{
				TimeoutInfo: &tmcons.TimeoutInfo{
					Duration: msg.Duration,
					Height:   msg.Height,
					Round:    msg.Round,
					Step:     uint8(msg.Step),
				},
			},
		}
	case EndHeightMessage:
		pb = tmcons.WALMessage{
			Sum: &tmcons.WALMessage_EndHeight{
				EndHeight: &tmcons.EndHeight{
					Height: msg.Height,
				},
			},
		}
	default:
		return nil, fmt.Errorf("to proto: wal message not recognized: %T", msg)
	}

	return &pb, nil
}

// WALFromProto takes a proto wal message and return a consensus walMessage and error
func WALFromProto(msg *tmcons.WALMessage) (WALMessage, error) {
	if msg == nil {
		return nil, errors.New("nil WAL message")
	}
	var pb WALMessage

	switch msg := msg.Sum.(type) {
	case *tmcons.WALMessage_EventDataRoundState:
		pb = types.EventDataRoundState{
			Height: msg.EventDataRoundState.Height,
			Round:  msg.EventDataRoundState.Round,
			Step:   msg.EventDataRoundState.Step,
		}
	case *tmcons.WALMessage_MsgInfo:
		walMsg, err := MsgFromProto(msg.MsgInfo.Msg)
		if err != nil {
			return nil, fmt.Errorf("msgInfo from proto error: %w", err)
		}
		pb = msgInfo{
			Msg:    walMsg,
			PeerID: p2p.ID(msg.MsgInfo.PeerId),
		}
	case *tmcons.WALMessage_TimeoutInfo:
		pb = timeoutInfo{
			Duration: msg.TimeoutInfo.Duration,
			Height:   msg.TimeoutInfo.Height,
			Round:    msg.TimeoutInfo.Round,
			Step:     cstypes.RoundStepType(msg.TimeoutInfo.Step),
		}
	case *tmcons.WALMessage_EndHeight:
		pb = EndHeightMessage{
			Height: msg.EndHeight.Height,
		}
	default:
		return nil, fmt.Errorf("from proto: wal message not recognized: %T", msg)
	}

	return pb, nil
}
