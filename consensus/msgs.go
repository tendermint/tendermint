package consensus

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/libs/bits"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/p2p"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// MsgToProto takes a consensus message type and returns the proto defined consensus message.
//
// TODO: This needs to be removed, but WALToProto depends on this.
func MsgToProto(msg Message) (*tmcons.Message, error) {
	if msg == nil {
		return nil, errors.New("consensus: message is nil")
	}
	switch msg := msg.(type) {
	case *NewRoundStepMessage:
		m := &tmcons.NewRoundStep{
			Height:                msg.Height,
			Round:                 msg.Round,
			Step:                  uint32(msg.Step),
			SecondsSinceStartTime: msg.SecondsSinceStartTime,
			LastCommitRound:       msg.LastCommitRound,
		}
		return m.Wrap().(*tmcons.Message), nil

	case *NewValidBlockMessage:
		pbPartSetHeader := msg.BlockPartSetHeader.ToProto()
		pbBits := msg.BlockParts.ToProto()
		m := &tmcons.NewValidBlock{
			Height:             msg.Height,
			Round:              msg.Round,
			BlockPartSetHeader: pbPartSetHeader,
			BlockParts:         pbBits,
			IsCommit:           msg.IsCommit,
		}
		return m.Wrap().(*tmcons.Message), nil

	case *ProposalMessage:
		pbP := msg.Proposal.ToProto()
		m := &tmcons.Proposal{
			Proposal: *pbP,
		}
		return m.Wrap().(*tmcons.Message), nil

	case *ProposalPOLMessage:
		pbBits := msg.ProposalPOL.ToProto()
		m := &tmcons.ProposalPOL{
			Height:           msg.Height,
			ProposalPolRound: msg.ProposalPOLRound,
			ProposalPol:      *pbBits,
		}
		return m.Wrap().(*tmcons.Message), nil

	case *BlockPartMessage:
		parts, err := msg.Part.ToProto()
		if err != nil {
			return nil, fmt.Errorf("msg to proto error: %w", err)
		}
		m := &tmcons.BlockPart{
			Height: msg.Height,
			Round:  msg.Round,
			Part:   *parts,
		}
		return m.Wrap().(*tmcons.Message), nil

	case *VoteMessage:
		vote := msg.Vote.ToProto()
		m := &tmcons.Vote{
			Vote: vote,
		}
		return m.Wrap().(*tmcons.Message), nil

	case *HasVoteMessage:
		m := &tmcons.HasVote{
			Height: msg.Height,
			Round:  msg.Round,
			Type:   msg.Type,
			Index:  msg.Index,
		}
		return m.Wrap().(*tmcons.Message), nil

	case *VoteSetMaj23Message:
		bi := msg.BlockID.ToProto()
		m := &tmcons.VoteSetMaj23{
			Height:  msg.Height,
			Round:   msg.Round,
			Type:    msg.Type,
			BlockID: bi,
		}
		return m.Wrap().(*tmcons.Message), nil

	case *VoteSetBitsMessage:
		bi := msg.BlockID.ToProto()
		bits := msg.Votes.ToProto()

		m := &tmcons.VoteSetBits{
			Height:  msg.Height,
			Round:   msg.Round,
			Type:    msg.Type,
			BlockID: bi,
		}

		if bits != nil {
			m.Votes = *bits
		}

		return m.Wrap().(*tmcons.Message), nil

	default:
		return nil, fmt.Errorf("consensus: message not recognized: %T", msg)
	}
}

// MsgFromProto takes a consensus proto message and returns the native go type
func MsgFromProto(p *tmcons.Message) (Message, error) {
	if p == nil {
		return nil, errors.New("consensus: nil message")
	}
	var pb Message
	um, err := p.Unwrap()
	if err != nil {
		return nil, err
	}

	switch msg := um.(type) {
	case *tmcons.NewRoundStep:
		rs, err := tmmath.SafeConvertUint8(int64(msg.Step))
		// deny message based on possible overflow
		if err != nil {
			return nil, fmt.Errorf("denying message due to possible overflow: %w", err)
		}
		pb = &NewRoundStepMessage{
			Height:                msg.Height,
			Round:                 msg.Round,
			Step:                  cstypes.RoundStepType(rs),
			SecondsSinceStartTime: msg.SecondsSinceStartTime,
			LastCommitRound:       msg.LastCommitRound,
		}
	case *tmcons.NewValidBlock:
		pbPartSetHeader, err := types.PartSetHeaderFromProto(&msg.BlockPartSetHeader)
		if err != nil {
			return nil, fmt.Errorf("parts to proto error: %w", err)
		}

		pbBits := new(bits.BitArray)
		pbBits.FromProto(msg.BlockParts)

		pb = &NewValidBlockMessage{
			Height:             msg.Height,
			Round:              msg.Round,
			BlockPartSetHeader: *pbPartSetHeader,
			BlockParts:         pbBits,
			IsCommit:           msg.IsCommit,
		}
	case *tmcons.Proposal:
		pbP, err := types.ProposalFromProto(&msg.Proposal)
		if err != nil {
			return nil, fmt.Errorf("proposal msg to proto error: %w", err)
		}

		pb = &ProposalMessage{
			Proposal: pbP,
		}
	case *tmcons.ProposalPOL:
		pbBits := new(bits.BitArray)
		pbBits.FromProto(&msg.ProposalPol)
		pb = &ProposalPOLMessage{
			Height:           msg.Height,
			ProposalPOLRound: msg.ProposalPolRound,
			ProposalPOL:      pbBits,
		}
	case *tmcons.BlockPart:
		parts, err := types.PartFromProto(&msg.Part)
		if err != nil {
			return nil, fmt.Errorf("blockpart msg to proto error: %w", err)
		}
		pb = &BlockPartMessage{
			Height: msg.Height,
			Round:  msg.Round,
			Part:   parts,
		}
	case *tmcons.Vote:
		vote, err := types.VoteFromProto(msg.Vote)
		if err != nil {
			return nil, fmt.Errorf("vote msg to proto error: %w", err)
		}

		pb = &VoteMessage{
			Vote: vote,
		}
	case *tmcons.HasVote:
		pb = &HasVoteMessage{
			Height: msg.Height,
			Round:  msg.Round,
			Type:   msg.Type,
			Index:  msg.Index,
		}
	case *tmcons.VoteSetMaj23:
		bi, err := types.BlockIDFromProto(&msg.BlockID)
		if err != nil {
			return nil, fmt.Errorf("voteSetMaj23 msg to proto error: %w", err)
		}
		pb = &VoteSetMaj23Message{
			Height:  msg.Height,
			Round:   msg.Round,
			Type:    msg.Type,
			BlockID: *bi,
		}
	case *tmcons.VoteSetBits:
		bi, err := types.BlockIDFromProto(&msg.BlockID)
		if err != nil {
			return nil, fmt.Errorf("voteSetBits msg to proto error: %w", err)
		}
		bits := new(bits.BitArray)
		bits.FromProto(&msg.Votes)

		pb = &VoteSetBitsMessage{
			Height:  msg.Height,
			Round:   msg.Round,
			Type:    msg.Type,
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

// MustEncode takes the reactors msg, makes it proto and marshals it
// this mimics `MustMarshalBinaryBare` in that is panics on error
//
// Deprecated: Will be removed in v0.37.
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

// WALToProto takes a WAL message and return a proto walMessage and error
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
					PeerID: string(msg.PeerID),
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
					Step:     uint32(msg.Step),
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
		walMsg, err := MsgFromProto(&msg.MsgInfo.Msg)
		if err != nil {
			return nil, fmt.Errorf("msgInfo from proto error: %w", err)
		}
		pb = msgInfo{
			Msg:    walMsg,
			PeerID: p2p.ID(msg.MsgInfo.PeerID),
		}

	case *tmcons.WALMessage_TimeoutInfo:
		tis, err := tmmath.SafeConvertUint8(int64(msg.TimeoutInfo.Step))
		// deny message based on possible overflow
		if err != nil {
			return nil, fmt.Errorf("denying message due to possible overflow: %w", err)
		}
		pb = timeoutInfo{
			Duration: msg.TimeoutInfo.Duration,
			Height:   msg.TimeoutInfo.Height,
			Round:    msg.TimeoutInfo.Round,
			Step:     cstypes.RoundStepType(tis),
		}
		return pb, nil
	case *tmcons.WALMessage_EndHeight:
		pb := EndHeightMessage{
			Height: msg.EndHeight.Height,
		}
		return pb, nil
	default:
		return nil, fmt.Errorf("from proto: wal message not recognized: %T", msg)
	}
	return pb, nil
}
