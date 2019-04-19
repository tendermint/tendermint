package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/golang/protobuf/ptypes"
	tspb "github.com/golang/protobuf/ptypes/timestamp"

	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
)

const (
	// MaxVoteBytes is a maximum vote size (including amino overhead).
	MaxVoteBytes int64 = 223
)

var (
	ErrVoteUnexpectedStep            = errors.New("Unexpected step")
	ErrVoteInvalidValidatorIndex     = errors.New("Invalid validator index")
	ErrVoteInvalidValidatorAddress   = errors.New("Invalid validator address")
	ErrVoteInvalidSignature          = errors.New("Invalid signature")
	ErrVoteInvalidBlockHash          = errors.New("Invalid block hash")
	ErrVoteNonDeterministicSignature = errors.New("Non-deterministic signature")
	ErrVoteNil                       = errors.New("Nil vote")
)

type ErrVoteConflictingVotes struct {
	*DuplicateVoteEvidence
}

func (err *ErrVoteConflictingVotes) Error() string {
	return fmt.Sprintf("Conflicting votes from validator %v", err.PubKey.Address())
}

func NewConflictingVoteError(val *Validator, voteA, voteB *Vote) *ErrVoteConflictingVotes {
	return &ErrVoteConflictingVotes{
		&DuplicateVoteEvidence{
			PubKey: val.PubKey,
			VoteA:  voteA,
			VoteB:  voteB,
		},
	}
}

// Address is hex bytes.
type Address = crypto.Address

// Vote represents a prevote, precommit, or commit vote from validators for
// consensus.
type Vote struct {
	Type             SignedMsgType `json:"type"`
	Height           int64         `json:"height"`
	Round            int           `json:"round"`
	BlockID          BlockID       `json:"block_id"` // zero if vote is nil.
	Timestamp        time.Time     `json:"timestamp"`
	ValidatorAddress Address       `json:"validator_address"`
	ValidatorIndex   int           `json:"validator_index"`
	Signature        []byte        `json:"signature"`
}

// CommitSig converts the Vote to a CommitSig.
// If the Vote is nil, the CommitSig will be nil.
func (vote *Vote) CommitSig() *CommitSig {
	if vote == nil {
		return nil
	}
	cs := CommitSig(*vote)
	return &cs
}

func (vote *Vote) SignBytes(chainID string) []byte {
	bz, err := cdc.MarshalBinaryLengthPrefixed(CanonicalizeVote(chainID, vote))
	if err != nil {
		panic(err)
	}
	return bz
}

func (vote *Vote) Copy() *Vote {
	voteCopy := *vote
	return &voteCopy
}

func (vote *Vote) String() string {
	if vote == nil {
		return "nil-Vote"
	}
	var typeString string
	switch vote.Type {
	case PrevoteType:
		typeString = "Prevote"
	case PrecommitType:
		typeString = "Precommit"
	default:
		cmn.PanicSanity("Unknown vote type")
	}

	return fmt.Sprintf("Vote{%v:%X %v/%02d/%v(%v) %X %X @ %s}",
		vote.ValidatorIndex,
		cmn.Fingerprint(vote.ValidatorAddress),
		vote.Height,
		vote.Round,
		vote.Type,
		typeString,
		cmn.Fingerprint(vote.BlockID.Hash),
		cmn.Fingerprint(vote.Signature),
		CanonicalTime(vote.Timestamp),
	)
}

func (vote *Vote) Verify(chainID string, pubKey crypto.PubKey) error {
	if !bytes.Equal(pubKey.Address(), vote.ValidatorAddress) {
		return ErrVoteInvalidValidatorAddress
	}

	if !pubKey.VerifyBytes(vote.SignBytes(chainID), vote.Signature) {
		return ErrVoteInvalidSignature
	}
	return nil
}

// ValidateBasic performs basic validation.
func (vote *Vote) ValidateBasic() error {
	if !IsVoteTypeValid(vote.Type) {
		return errors.New("Invalid Type")
	}
	if vote.Height < 0 {
		return errors.New("Negative Height")
	}
	if vote.Round < 0 {
		return errors.New("Negative Round")
	}

	// NOTE: Timestamp validation is subtle and handled elsewhere.

	if err := vote.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong BlockID: %v", err)
	}
	// BlockID.ValidateBasic would not err if we for instance have an empty hash but a
	// non-empty PartsSetHeader:
	if !vote.BlockID.IsZero() && !vote.BlockID.IsComplete() {
		return fmt.Errorf("BlockID must be either empty or complete, got: %v", vote.BlockID)
	}
	if len(vote.ValidatorAddress) != crypto.AddressSize {
		return fmt.Errorf("Expected ValidatorAddress size to be %d bytes, got %d bytes",
			crypto.AddressSize,
			len(vote.ValidatorAddress),
		)
	}
	if vote.ValidatorIndex < 0 {
		return errors.New("Negative ValidatorIndex")
	}
	if len(vote.Signature) == 0 {
		return errors.New("Signature is missing")
	}
	if len(vote.Signature) > MaxSignatureSize {
		return fmt.Errorf("Signature is too big (max: %d)", MaxSignatureSize)
	}
	return nil
}

func (vote Vote) toCustomType() VoteCustomType {
	return VoteCustomType{Vote: &vote}
}
func (vote Vote) Equal(other Vote) bool {
	proto := vote.toVoteType()
	return proto.Equal(other.toVoteType())
}

func (vote *Vote) toVoteType() *VoteType {
	proto := &VoteType{
		Type:             uint32(vote.Type),
		Height:           vote.Height,
		Round:            int64(vote.Round), // TODO
		BlockID:          &BlockIDType{Hash: vote.BlockID.Hash, PartSetHeader: &PartSetHeaderType{Total: int64(vote.BlockID.PartsHeader.Total), Hash: vote.BlockID.PartsHeader.Hash.Bytes()}},
		TimestampField:   &types.Timestamp{Seconds: int64(vote.Timestamp.Second()), Nanos: int32(vote.Timestamp.Nanosecond())},
		ValidatorAddress: vote.ValidatorAddress[:],
		ValidatorIndex:   int64(vote.ValidatorIndex),
		Signature:        vote.Signature,
	}
	return proto
}

func fromVoteType(voteType *VoteType) *Vote {
	vote := &Vote{}
	if voteType != nil {
		vote.Type = SignedMsgType(voteType.Type)

		vote.Height = voteType.Height
		vote.Round = int(voteType.Round) // FIXME
		vote.BlockID = BlockID{Hash: voteType.BlockID.Hash, PartsHeader: PartSetHeader{int(voteType.BlockID.PartSetHeader.Total), voteType.BlockID.PartSetHeader.Hash}}
		ti, err := ptypes.Timestamp(&tspb.Timestamp{Seconds: voteType.TimestampField.Seconds, Nanos: voteType.TimestampField.Nanos})
		if err != nil {
			panic(err)
		}
		vote.Timestamp = ti
		vote.ValidatorAddress = voteType.ValidatorAddress
		vote.ValidatorIndex = int(voteType.ValidatorIndex) // FIXME

		vote.Signature = voteType.Signature
	}

	return vote
}

func (vote *Vote) Size() int {
	return vote.toVoteType().Size()
}

func NewPopulatedVote(r randyTypes) *Vote {
	data := NewPopulatedVoteType(r, false)
	gt := fromVoteType(data)
	return gt
}

func (vote Vote) Marshal() ([]byte, error) {
	vt := vote.toVoteType()
	return proto.Marshal(vt)
}

func (vote *Vote) Unmarshal(data []byte) error {
	pr := &VoteType{}
	err := proto.Unmarshal(data, pr)
	if err != nil {
		return err
	}

	*vote = *fromVoteType(pr)
	return nil
}

func (vote Vote) MarshalJSON() ([]byte, error) {
	return json.Marshal(vote)
}

func (gt *Vote) UnmarshalJSON(data []byte) error {
	var v Vote
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	*gt = v
	return nil
}

func (gt Vote) Compare(other Vote) int {
	// todo
	return 1
}
