package consensus

import (
	"errors"
	"fmt"

	cstypes "github.com/tendermint/tendermint/internal/consensus/types"
	"github.com/tendermint/tendermint/internal/jsontypes"
	"github.com/tendermint/tendermint/libs/bits"
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// Message defines an interface that the consensus domain types implement. When
// a proto message is received on a consensus p2p Channel, it is wrapped and then
// converted to a Message via MsgFromProto.
type Message interface {
	ValidateBasic() error

	jsontypes.Tagged
}

func init() {
	jsontypes.MustRegister(&NewRoundStepMessage{})
	jsontypes.MustRegister(&NewValidBlockMessage{})
	jsontypes.MustRegister(&ProposalMessage{})
	jsontypes.MustRegister(&ProposalPOLMessage{})
	jsontypes.MustRegister(&BlockPartMessage{})
	jsontypes.MustRegister(&VoteMessage{})
	jsontypes.MustRegister(&HasVoteMessage{})
	jsontypes.MustRegister(&VoteSetMaj23Message{})
	jsontypes.MustRegister(&VoteSetBitsMessage{})
}

// NewRoundStepMessage is sent for every step taken in the ConsensusState.
// For every height/round/step transition
type NewRoundStepMessage struct {
	Height                int64 `json:",string"`
	Round                 int32
	Step                  cstypes.RoundStepType
	SecondsSinceStartTime int64 `json:",string"`
	LastCommitRound       int32
}

func (*NewRoundStepMessage) TypeTag() string { return "tendermint/NewRoundStepMessage" }

// ValidateBasic performs basic validation.
func (m *NewRoundStepMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if !m.Step.IsValid() {
		return errors.New("invalid Step")
	}

	// NOTE: SecondsSinceStartTime may be negative

	// LastCommitRound will be -1 for the initial height, but we don't know what height this is
	// since it can be specified in genesis. The reactor will have to validate this via
	// ValidateHeight().
	if m.LastCommitRound < -1 {
		return errors.New("invalid LastCommitRound (cannot be < -1)")
	}

	return nil
}

// ValidateHeight validates the height given the chain's initial height.
func (m *NewRoundStepMessage) ValidateHeight(initialHeight int64) error {
	if m.Height < initialHeight {
		return fmt.Errorf("invalid Height %v (lower than initial height %v)",
			m.Height, initialHeight)
	}
	if m.Height == initialHeight && m.LastCommitRound != -1 {
		return fmt.Errorf("invalid LastCommitRound %v (must be -1 for initial height %v)",
			m.LastCommitRound, initialHeight)
	}
	if m.Height > initialHeight && m.LastCommitRound < 0 {
		return fmt.Errorf("LastCommitRound can only be negative for initial height %v",
			initialHeight)
	}
	return nil
}

// String returns a string representation.
func (m *NewRoundStepMessage) String() string {
	return fmt.Sprintf("[NewRoundStep H:%v R:%v S:%v LCR:%v]",
		m.Height, m.Round, m.Step, m.LastCommitRound)
}

// NewValidBlockMessage is sent when a validator observes a valid block B in some round r,
// i.e., there is a Proposal for block B and 2/3+ prevotes for the block B in the round r.
// In case the block is also committed, then IsCommit flag is set to true.
type NewValidBlockMessage struct {
	Height             int64 `json:",string"`
	Round              int32
	BlockPartSetHeader types.PartSetHeader
	BlockParts         *bits.BitArray
	IsCommit           bool
}

func (*NewValidBlockMessage) TypeTag() string { return "tendermint/NewValidBlockMessage" }

// ValidateBasic performs basic validation.
func (m *NewValidBlockMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if err := m.BlockPartSetHeader.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockPartSetHeader: %w", err)
	}
	if m.BlockParts.Size() == 0 {
		return errors.New("empty blockParts")
	}
	if m.BlockParts.Size() != int(m.BlockPartSetHeader.Total) {
		return fmt.Errorf("blockParts bit array size %d not equal to BlockPartSetHeader.Total %d",
			m.BlockParts.Size(),
			m.BlockPartSetHeader.Total)
	}
	if m.BlockParts.Size() > int(types.MaxBlockPartsCount) {
		return fmt.Errorf("blockParts bit array is too big: %d, max: %d", m.BlockParts.Size(), types.MaxBlockPartsCount)
	}
	return nil
}

// String returns a string representation.
func (m *NewValidBlockMessage) String() string {
	return fmt.Sprintf("[ValidBlockMessage H:%v R:%v BP:%v BA:%v IsCommit:%v]",
		m.Height, m.Round, m.BlockPartSetHeader, m.BlockParts, m.IsCommit)
}

// ProposalMessage is sent when a new block is proposed.
type ProposalMessage struct {
	Proposal *types.Proposal
}

func (*ProposalMessage) TypeTag() string { return "tendermint/Proposal" }

// ValidateBasic performs basic validation.
func (m *ProposalMessage) ValidateBasic() error {
	return m.Proposal.ValidateBasic()
}

// String returns a string representation.
func (m *ProposalMessage) String() string {
	return fmt.Sprintf("[Proposal %v]", m.Proposal)
}

// ProposalPOLMessage is sent when a previous proposal is re-proposed.
type ProposalPOLMessage struct {
	Height           int64 `json:",string"`
	ProposalPOLRound int32
	ProposalPOL      *bits.BitArray
}

func (*ProposalPOLMessage) TypeTag() string { return "tendermint/ProposalPOL" }

// ValidateBasic performs basic validation.
func (m *ProposalPOLMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.ProposalPOLRound < 0 {
		return errors.New("negative ProposalPOLRound")
	}
	if m.ProposalPOL.Size() == 0 {
		return errors.New("empty ProposalPOL bit array")
	}
	if m.ProposalPOL.Size() > types.MaxVotesCount {
		return fmt.Errorf("proposalPOL bit array is too big: %d, max: %d", m.ProposalPOL.Size(), types.MaxVotesCount)
	}
	return nil
}

// String returns a string representation.
func (m *ProposalPOLMessage) String() string {
	return fmt.Sprintf("[ProposalPOL H:%v POLR:%v POL:%v]", m.Height, m.ProposalPOLRound, m.ProposalPOL)
}

// BlockPartMessage is sent when gossipping a piece of the proposed block.
type BlockPartMessage struct {
	Height int64 `json:",string"`
	Round  int32
	Part   *types.Part
}

func (*BlockPartMessage) TypeTag() string { return "tendermint/BlockPart" }

// ValidateBasic performs basic validation.
func (m *BlockPartMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if err := m.Part.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong Part: %w", err)
	}
	return nil
}

// String returns a string representation.
func (m *BlockPartMessage) String() string {
	return fmt.Sprintf("[BlockPart H:%v R:%v P:%v]", m.Height, m.Round, m.Part)
}

// VoteMessage is sent when voting for a proposal (or lack thereof).
type VoteMessage struct {
	Vote *types.Vote
}

func (*VoteMessage) TypeTag() string { return "tendermint/Vote" }

// ValidateBasic checks whether the vote within the message is well-formed.
func (m *VoteMessage) ValidateBasic() error {
	return m.Vote.ValidateBasic()
}

// String returns a string representation.
func (m *VoteMessage) String() string {
	return fmt.Sprintf("[Vote %v]", m.Vote)
}

// HasVoteMessage is sent to indicate that a particular vote has been received.
type HasVoteMessage struct {
	Height int64 `json:",string"`
	Round  int32
	Type   tmproto.SignedMsgType
	Index  int32
}

func (*HasVoteMessage) TypeTag() string { return "tendermint/HasVote" }

// ValidateBasic performs basic validation.
func (m *HasVoteMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("invalid Type")
	}
	if m.Index < 0 {
		return errors.New("negative Index")
	}
	return nil
}

// String returns a string representation.
func (m *HasVoteMessage) String() string {
	return fmt.Sprintf("[HasVote VI:%v V:{%v/%02d/%v}]", m.Index, m.Height, m.Round, m.Type)
}

// VoteSetMaj23Message is sent to indicate that a given BlockID has seen +2/3 votes.
type VoteSetMaj23Message struct {
	Height  int64 `json:",string"`
	Round   int32
	Type    tmproto.SignedMsgType
	BlockID types.BlockID
}

func (*VoteSetMaj23Message) TypeTag() string { return "tendermint/VoteSetMaj23" }

// ValidateBasic performs basic validation.
func (m *VoteSetMaj23Message) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("invalid Type")
	}
	if err := m.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %w", err)
	}

	return nil
}

// String returns a string representation.
func (m *VoteSetMaj23Message) String() string {
	return fmt.Sprintf("[VSM23 %v/%02d/%v %v]", m.Height, m.Round, m.Type, m.BlockID)
}

// VoteSetBitsMessage is sent to communicate the bit-array of votes seen for the
// BlockID.
type VoteSetBitsMessage struct {
	Height  int64 `json:",string"`
	Round   int32
	Type    tmproto.SignedMsgType
	BlockID types.BlockID
	Votes   *bits.BitArray
}

func (*VoteSetBitsMessage) TypeTag() string { return "tendermint/VoteSetBits" }

// ValidateBasic performs basic validation.
func (m *VoteSetBitsMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("invalid Type")
	}
	if err := m.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %w", err)
	}

	// NOTE: Votes.Size() can be zero if the node does not have any
	if m.Votes.Size() > types.MaxVotesCount {
		return fmt.Errorf("votes bit array is too big: %d, max: %d", m.Votes.Size(), types.MaxVotesCount)
	}

	return nil
}

// String returns a string representation.
func (m *VoteSetBitsMessage) String() string {
	return fmt.Sprintf("[VSB %v/%02d/%v %v %v]", m.Height, m.Round, m.Type, m.BlockID, m.Votes)
}

// MsgToProto takes a consensus message type and returns the proto defined
// consensus message.
//
// TODO: This needs to be removed, but WALToProto depends on this.
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
					Step:                  uint32(msg.Step),
					SecondsSinceStartTime: msg.SecondsSinceStartTime,
					LastCommitRound:       msg.LastCommitRound,
				},
			},
		}
	case *NewValidBlockMessage:
		pbPartSetHeader := msg.BlockPartSetHeader.ToProto()
		pbBits := msg.BlockParts.ToProto()
		pb = tmcons.Message{
			Sum: &tmcons.Message_NewValidBlock{
				NewValidBlock: &tmcons.NewValidBlock{
					Height:             msg.Height,
					Round:              msg.Round,
					BlockPartSetHeader: pbPartSetHeader,
					BlockParts:         pbBits,
					IsCommit:           msg.IsCommit,
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
					BlockID: bi,
				},
			},
		}
	case *VoteSetBitsMessage:
		bi := msg.BlockID.ToProto()
		bits := msg.Votes.ToProto()

		vsb := &tmcons.Message_VoteSetBits{
			VoteSetBits: &tmcons.VoteSetBits{
				Height:  msg.Height,
				Round:   msg.Round,
				Type:    msg.Type,
				BlockID: bi,
			},
		}

		if bits != nil {
			vsb.VoteSetBits.Votes = *bits
		}

		pb = tmcons.Message{
			Sum: vsb,
		}

	default:
		return nil, fmt.Errorf("consensus: message not recognized: %T", msg)
	}

	return &pb, nil
}

// MsgFromProto takes a consensus proto message and returns the native go type.
func MsgFromProto(msg *tmcons.Message) (Message, error) {
	if msg == nil {
		return nil, errors.New("consensus: nil message")
	}
	var pb Message

	switch msg := msg.Sum.(type) {
	case *tmcons.Message_NewRoundStep:
		rs, err := tmmath.SafeConvertUint8(int64(msg.NewRoundStep.Step))
		// deny message based on possible overflow
		if err != nil {
			return nil, fmt.Errorf("denying message due to possible overflow: %w", err)
		}
		pb = &NewRoundStepMessage{
			Height:                msg.NewRoundStep.Height,
			Round:                 msg.NewRoundStep.Round,
			Step:                  cstypes.RoundStepType(rs),
			SecondsSinceStartTime: msg.NewRoundStep.SecondsSinceStartTime,
			LastCommitRound:       msg.NewRoundStep.LastCommitRound,
		}
	case *tmcons.Message_NewValidBlock:
		pbPartSetHeader, err := types.PartSetHeaderFromProto(&msg.NewValidBlock.BlockPartSetHeader)
		if err != nil {
			return nil, fmt.Errorf("parts header to proto error: %w", err)
		}

		pbBits := new(bits.BitArray)
		err = pbBits.FromProto(msg.NewValidBlock.BlockParts)
		if err != nil {
			return nil, fmt.Errorf("parts to proto error: %w", err)
		}

		pb = &NewValidBlockMessage{
			Height:             msg.NewValidBlock.Height,
			Round:              msg.NewValidBlock.Round,
			BlockPartSetHeader: *pbPartSetHeader,
			BlockParts:         pbBits,
			IsCommit:           msg.NewValidBlock.IsCommit,
		}
	case *tmcons.Message_Proposal:
		pbP, err := types.ProposalFromProto(&msg.Proposal.Proposal)
		if err != nil {
			return nil, fmt.Errorf("proposal msg to proto error: %w", err)
		}

		pb = &ProposalMessage{
			Proposal: pbP,
		}
	case *tmcons.Message_ProposalPol:
		pbBits := new(bits.BitArray)
		err := pbBits.FromProto(&msg.ProposalPol.ProposalPol)
		if err != nil {
			return nil, fmt.Errorf("proposal PoL to proto error: %w", err)
		}
		pb = &ProposalPOLMessage{
			Height:           msg.ProposalPol.Height,
			ProposalPOLRound: msg.ProposalPol.ProposalPolRound,
			ProposalPOL:      pbBits,
		}
	case *tmcons.Message_BlockPart:
		parts, err := types.PartFromProto(&msg.BlockPart.Part)
		if err != nil {
			return nil, fmt.Errorf("blockpart msg to proto error: %w", err)
		}
		pb = &BlockPartMessage{
			Height: msg.BlockPart.Height,
			Round:  msg.BlockPart.Round,
			Part:   parts,
		}
	case *tmcons.Message_Vote:
		// Vote validation will be handled in the vote message ValidateBasic
		// call below.
		vote, err := types.VoteFromProto(msg.Vote.Vote)
		if err != nil {
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
		bi, err := types.BlockIDFromProto(&msg.VoteSetMaj23.BlockID)
		if err != nil {
			return nil, fmt.Errorf("voteSetMaj23 msg to proto error: %w", err)
		}
		pb = &VoteSetMaj23Message{
			Height:  msg.VoteSetMaj23.Height,
			Round:   msg.VoteSetMaj23.Round,
			Type:    msg.VoteSetMaj23.Type,
			BlockID: *bi,
		}
	case *tmcons.Message_VoteSetBits:
		bi, err := types.BlockIDFromProto(&msg.VoteSetBits.BlockID)
		if err != nil {
			return nil, fmt.Errorf("block ID to proto error: %w", err)
		}
		bits := new(bits.BitArray)
		err = bits.FromProto(&msg.VoteSetBits.Votes)
		if err != nil {
			return nil, fmt.Errorf("votes to proto error: %w", err)
		}

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

// WALToProto takes a WAL message and return a proto walMessage and error.
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

// WALFromProto takes a proto wal message and return a consensus walMessage and
// error.
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
			PeerID: types.NodeID(msg.MsgInfo.PeerID),
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
