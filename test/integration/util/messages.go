package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ds-test-framework/scheduler/types"
	"github.com/gogo/protobuf/proto"
	tmsg "github.com/tendermint/tendermint/proto/tendermint/consensus"
	prototypes "github.com/tendermint/tendermint/proto/tendermint/types"
	ttypes "github.com/tendermint/tendermint/types"
)

type MessageType string

var (
	ErrInvalidVote = errors.New("invalid message type to change vote")
)

const (
	NewRoundStep  MessageType = "NewRoundStep"
	NewValidBlock MessageType = "NewValidBlock"
	Proposal      MessageType = "Proposal"
	ProposalPol   MessageType = "ProposalPol"
	BlockPart     MessageType = "BlockPart"
	Vote          MessageType = "Vote"
	Prevote       MessageType = "Prevote"
	Precommit     MessageType = "Precommit"
	HasVote       MessageType = "HasVote"
	VoteSetMaj23  MessageType = "VoteSetMaj23"
	VoteSetBits   MessageType = "VoteSetBits"
	None          MessageType = "None"
)

type TMessageWrapper struct {
	ChannelID        uint16          `json:"chan_id"`
	MsgB             []byte          `json:"msg"`
	From             types.ReplicaID `json:"from"`
	To               types.ReplicaID `json:"to"`
	Type             MessageType     `json:"-"`
	Msg              *tmsg.Message   `json:"-"`
	SchedulerMessage *types.Message  `json:"-"`
}

func GetMessageFromSendEvent(event *types.Event, messagePool *types.MessageStore) (*TMessageWrapper, error) {
	if !event.IsMessageSend() {
		return nil, errors.New("event is not message send ")
	}
	messageID, _ := event.MessageID()
	message, ok := messagePool.Get(messageID)
	if !ok {
		return nil, errors.New("message not found in the pool")
	}

	tMsg, err := Unmarshal(message.Data)
	if err != nil {
		return nil, err
	}
	tMsg.SchedulerMessage = message
	return tMsg, nil
}

func Unmarshal(m []byte) (*TMessageWrapper, error) {
	var cMsg TMessageWrapper
	err := json.Unmarshal(m, &cMsg)
	if err != nil {
		return &cMsg, err
	}
	chid := cMsg.ChannelID
	if chid < 0x20 || chid > 0x23 {
		cMsg.Type = "None"
		return &cMsg, nil
	}

	msg := proto.Clone(new(tmsg.Message))
	msg.Reset()

	if err := proto.Unmarshal(cMsg.MsgB, msg); err != nil {
		// log.Debug("Error unmarshalling")
		cMsg.Type = None
		cMsg.Msg = nil
		return &cMsg, nil
	}

	tMsg := msg.(*tmsg.Message)
	cMsg.Msg = tMsg

	switch tMsg.Sum.(type) {
	case *tmsg.Message_NewRoundStep:
		cMsg.Type = NewRoundStep
	case *tmsg.Message_NewValidBlock:
		cMsg.Type = NewValidBlock
	case *tmsg.Message_Proposal:
		cMsg.Type = Proposal
	case *tmsg.Message_ProposalPol:
		cMsg.Type = ProposalPol
	case *tmsg.Message_BlockPart:
		cMsg.Type = BlockPart
	case *tmsg.Message_Vote:
		v := tMsg.GetVote()
		if v == nil {
			cMsg.Type = Vote
			break
		}
		switch v.Vote.Type {
		case prototypes.PrevoteType:
			cMsg.Type = Prevote
		case prototypes.PrecommitType:
			cMsg.Type = Precommit
		default:
			cMsg.Type = Vote
		}
	case *tmsg.Message_HasVote:
		cMsg.Type = HasVote
	case *tmsg.Message_VoteSetMaj23:
		cMsg.Type = VoteSetMaj23
	case *tmsg.Message_VoteSetBits:
		cMsg.Type = VoteSetBits
	default:
		cMsg.Type = None
	}

	// log.Debug(fmt.Sprintf("Received message from: %s, with contents: %s", cMsg.From, cMsg.Msg.String()))
	return &cMsg, err
}

func Marshal(msg *TMessageWrapper) ([]byte, error) {
	msgB, err := proto.Marshal(msg.Msg)
	if err != nil {
		return nil, err
	}
	msg.MsgB = msgB

	result, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func ExtractHR(msg *TMessageWrapper) (int, int) {
	switch msg.Type {
	case NewRoundStep:
		hrs := msg.Msg.GetNewRoundStep()
		return int(hrs.Height), int(hrs.Round)
	case Proposal:
		prop := msg.Msg.GetProposal()
		return int(prop.Proposal.Height), int(prop.Proposal.Round)
	case Prevote:
		vote := msg.Msg.GetVote()
		return int(vote.Vote.Height), int(vote.Vote.Round)
	case Precommit:
		vote := msg.Msg.GetVote()
		return int(vote.Vote.Height), int(vote.Vote.Round)
	case Vote:
		vote := msg.Msg.GetVote()
		return int(vote.Vote.Height), int(vote.Vote.Round)
	case NewValidBlock:
		block := msg.Msg.GetNewValidBlock()
		return int(block.Height), int(block.Round)
	case ProposalPol:
		pPol := msg.Msg.GetProposalPol()
		return int(pPol.Height), -1
	case VoteSetMaj23:
		vote := msg.Msg.GetVoteSetMaj23()
		return int(vote.Height), int(vote.Round)
	case VoteSetBits:
		vote := msg.Msg.GetVoteSetBits()
		return int(vote.Height), int(vote.Round)
	case BlockPart:
		blockPart := msg.Msg.GetBlockPart()
		return int(blockPart.Height), int(blockPart.Round)
	}
	return -1, -1
}

func ChangeVoteToNil(replica *types.Replica, voteMsg *TMessageWrapper) (*TMessageWrapper, error) {
	return ChangeVote(replica, voteMsg, &ttypes.BlockID{
		Hash:          nil,
		PartSetHeader: ttypes.PartSetHeader{},
	})
}

func ChangeVote(replica *types.Replica, tMsg *TMessageWrapper, blockID *ttypes.BlockID) (*TMessageWrapper, error) {
	privKey, err := GetPrivKey(replica)
	if err != nil {
		return nil, err
	}
	chainID, err := GetChainID(replica)
	if err != nil {
		return nil, err
	}

	if tMsg.Type != Prevote && tMsg.Type != Precommit {
		// Can't change vote of unknown type
		return tMsg, ErrInvalidVote
	}

	vote := tMsg.Msg.GetVote().Vote
	newVote := &ttypes.Vote{
		Type:             vote.Type,
		Height:           vote.Height,
		Round:            vote.Round,
		BlockID:          *blockID,
		Timestamp:        vote.Timestamp,
		ValidatorAddress: vote.ValidatorAddress,
		ValidatorIndex:   vote.ValidatorIndex,
	}
	signBytes := ttypes.VoteSignBytes(chainID, newVote.ToProto())

	sig, err := privKey.Sign(signBytes)
	if err != nil {
		return nil, fmt.Errorf("could not sign vote: %s", err)
	}

	newVote.Signature = sig

	tMsg.Msg = &tmsg.Message{
		Sum: &tmsg.Message_Vote{
			Vote: &tmsg.Vote{
				Vote: newVote.ToProto(),
			},
		},
	}

	return tMsg, nil
}

func ChangeVoteTime(replica *types.Replica, tMsg *TMessageWrapper, time time.Time) (*TMessageWrapper, error) {

	privKey, err := GetPrivKey(replica)
	if err != nil {
		return nil, err
	}
	chainID, err := GetChainID(replica)
	if err != nil {
		return nil, err
	}

	if tMsg.Type != Prevote && tMsg.Type != Precommit {
		// Can't change vote of unknown type
		return tMsg, nil
	}

	vote := tMsg.Msg.GetVote().Vote

	blockID, err := ttypes.BlockIDFromProto(&vote.BlockID)
	if err != nil {
		return nil, err
	}
	newVote := &ttypes.Vote{
		Type:             vote.Type,
		Height:           vote.Height,
		Round:            vote.Round,
		BlockID:          *blockID,
		Timestamp:        time,
		ValidatorAddress: vote.ValidatorAddress,
		ValidatorIndex:   vote.ValidatorIndex,
	}
	signBytes := ttypes.VoteSignBytes(chainID, newVote.ToProto())

	sig, err := privKey.Sign(signBytes)
	if err != nil {
		return nil, fmt.Errorf("could not sign vote: %s", err)
	}

	newVote.Signature = sig

	tMsg.Msg = &tmsg.Message{
		Sum: &tmsg.Message_Vote{
			Vote: &tmsg.Vote{
				Vote: newVote.ToProto(),
			},
		},
	}

	return tMsg, nil
}

func ChangeProposalBlockID(replica *types.Replica, pMsg *TMessageWrapper) (*TMessageWrapper, error) {
	privKey, err := GetPrivKey(replica)
	if err != nil {
		return nil, err
	}
	chainID, err := GetChainID(replica)
	if err != nil {
		return nil, err
	}
	propP := pMsg.Msg.GetProposal().Proposal
	prop, err := ttypes.ProposalFromProto(&propP)
	if err != nil {
		return nil, errors.New("failed converting proposal message")
	}
	newProp := &ttypes.Proposal{
		Type:     prop.Type,
		Height:   prop.Height,
		Round:    prop.Round,
		POLRound: prop.POLRound,
		BlockID: ttypes.BlockID{
			Hash:          nil,
			PartSetHeader: ttypes.PartSetHeader{},
		},
		Timestamp: prop.Timestamp,
	}
	signB := ttypes.ProposalSignBytes(chainID, newProp.ToProto())
	sig, err := privKey.Sign(signB)
	if err != nil {
		return nil, fmt.Errorf("could not sign proposal: %s", err)
	}
	newProp.Signature = sig
	pMsg.Msg = &tmsg.Message{
		Sum: &tmsg.Message_Proposal{
			Proposal: &tmsg.Proposal{
				Proposal: *newProp.ToProto(),
			},
		},
	}

	return pMsg, nil
}

func ChangeProposalLockedValue(replica *types.Replica, pMsg *TMessageWrapper) (*TMessageWrapper, error) {
	privKey, err := GetPrivKey(replica)
	if err != nil {
		return nil, err
	}
	chainID, err := GetChainID(replica)
	if err != nil {
		return nil, err
	}
	propP := pMsg.Msg.GetProposal().Proposal
	prop, err := ttypes.ProposalFromProto(&propP)
	if err != nil {
		return nil, errors.New("failed converting proposal message")
	}
	newProp := &ttypes.Proposal{
		Type:      prop.Type,
		Height:    prop.Height,
		Round:     prop.Round,
		POLRound:  -1,
		BlockID:   prop.BlockID,
		Timestamp: prop.Timestamp,
	}

	signB := ttypes.ProposalSignBytes(chainID, newProp.ToProto())
	sig, err := privKey.Sign(signB)
	if err != nil {
		return nil, fmt.Errorf("could not sign proposal: %s", err)
	}
	newProp.Signature = sig
	pMsg.Msg = &tmsg.Message{
		Sum: &tmsg.Message_Proposal{
			Proposal: &tmsg.Proposal{
				Proposal: *newProp.ToProto(),
			},
		},
	}

	return pMsg, nil
}

func GetProposalBlockIDS(msg *TMessageWrapper) (string, bool) {
	blockID, ok := GetProposalBlockID(msg)
	if !ok {
		return "", false
	}
	return blockID.Hash.String(), true
}

func GetProposalBlockID(msg *TMessageWrapper) (*ttypes.BlockID, bool) {
	if msg.Type != Proposal {
		return nil, false
	}
	prop := msg.Msg.GetProposal()
	blockID, err := ttypes.BlockIDFromProto(&prop.Proposal.BlockID)
	if err != nil {
		return nil, false
	}
	return blockID, true
}

func GetVoteTime(msg *TMessageWrapper) (time.Time, bool) {
	if msg.Type == Precommit || msg.Type == Prevote {
		return msg.Msg.GetVote().Vote.Timestamp, true
	}
	return time.Time{}, false
}

func GetVoteBlockIDS(msg *TMessageWrapper) (string, bool) {
	blockID, ok := GetVoteBlockID(msg)
	if !ok {
		return "", false
	}
	return blockID.Hash.String(), true
}

func GetVoteBlockID(msg *TMessageWrapper) (*ttypes.BlockID, bool) {
	if msg.Type != Prevote && msg.Type != Precommit {
		return nil, false
	}
	vote := msg.Msg.GetVote().Vote
	blockID, err := ttypes.BlockIDFromProto(&vote.BlockID)
	if err != nil {
		return nil, false
	}
	return blockID, true
}

func IsVoteFrom(msg *TMessageWrapper, replica *types.Replica) bool {
	privKey, err := GetPrivKey(replica)
	if err != nil {
		return false
	}
	replicaAddr := privKey.PubKey().Address()
	voteAddr := msg.Msg.GetVote().Vote.ValidatorAddress

	return bytes.Equal(replicaAddr.Bytes(), voteAddr)
}

func GetVoteValidator(msg *TMessageWrapper) ([]byte, bool) {
	if msg.Type != Prevote && msg.Type != Precommit {
		return []byte{}, false
	}
	return msg.Msg.GetVote().Vote.GetValidatorAddress(), true
}
