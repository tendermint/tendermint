package types

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmproto "github.com/tendermint/tendermint/proto/types"
)

const (
	// MaxVoteBytes is a maximum vote size (including amino overhead).
	MaxVoteBytes int64  = 211
	nilVoteStr   string = "nil-Vote"
)

var (
	ErrVoteUnexpectedStep            = errors.New("unexpected step")
	ErrVoteInvalidValidatorIndex     = errors.New("invalid validator index")
	ErrVoteInvalidValidatorAddress   = errors.New("invalid validator address")
	ErrVoteInvalidSignature          = errors.New("invalid signature")
	ErrVoteInvalidBlockHash          = errors.New("invalid block hash")
	ErrVoteNonDeterministicSignature = errors.New("non-deterministic signature")
	ErrVoteNil                       = errors.New("nil vote")
)

type ErrVoteConflictingVotes struct {
	*DuplicateVoteEvidence
}

func (err *ErrVoteConflictingVotes) Error() string {
	return fmt.Sprintf("Conflicting votes from validator %v", err.VoteA.ValidatorAddress)
}

func NewConflictingVoteError(val *Validator, vote1, vote2 *Vote) *ErrVoteConflictingVotes {
	return &ErrVoteConflictingVotes{
		NewDuplicateVoteEvidence(vote1, vote2),
	}
}

// Address is hex bytes.
type Address = crypto.Address

// Vote represents a prevote, precommit, or commit vote from validators for
// consensus.
type Vote struct {
	Type             tmproto.SignedMsgType `json:"type"`
	Height           int64                 `json:"height"`
	Round            int32                 `json:"round"`    // assume there will not be greater than 2_147_483_647 rounds
	BlockID          BlockID               `json:"block_id"` // zero if vote is nil.
	Timestamp        time.Time             `json:"timestamp"`
	ValidatorAddress Address               `json:"validator_address"`
	// assume there will not be greater than 2_147_483_647 validators
	ValidatorIndex uint32 `json:"validator_index"`
	Signature      []byte `json:"signature"`
}

// CommitSig converts the Vote to a CommitSig.
func (vote *Vote) CommitSig() CommitSig {
	if vote == nil {
		return NewCommitSigAbsent()
	}

	var blockIDFlag tmproto.BlockIDFlag
	switch {
	case vote.BlockID.IsComplete():
		blockIDFlag = tmproto.BlockIDFlagCommit
	case vote.BlockID.IsZero():
		blockIDFlag = tmproto.BlockIDFlagNil
	default:
		panic(fmt.Sprintf("Invalid vote %v - expected BlockID to be either empty or complete", vote))
	}

	return CommitSig{
		BlockIDFlag:      blockIDFlag,
		ValidatorAddress: vote.ValidatorAddress,
		Timestamp:        vote.Timestamp,
		Signature:        vote.Signature,
	}
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
		return nilVoteStr
	}

	var typeString string
	switch vote.Type {
	case tmproto.PrevoteType:
		typeString = "Prevote"
	case tmproto.PrecommitType:
		typeString = "Precommit"
	default:
		panic("Unknown vote type")
	}

	return fmt.Sprintf("Vote{%v:%X %v/%02d/%v(%v) %X %X @ %s}",
		vote.ValidatorIndex,
		tmbytes.Fingerprint(vote.ValidatorAddress),
		vote.Height,
		vote.Round,
		vote.Type,
		typeString,
		tmbytes.Fingerprint(vote.BlockID.Hash),
		tmbytes.Fingerprint(vote.Signature),
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
		return errors.New("invalid Type")
	}
	if vote.Height < 0 {
		return errors.New("negative Height")
	}
	if vote.Round < 0 {
		return errors.New("negative Round")
	}

	// NOTE: Timestamp validation is subtle and handled elsewhere.

	if err := vote.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %v", err)
	}
	// BlockID.ValidateBasic would not err if we for instance have an empty hash but a
	// non-empty PartsSetHeader:
	if !vote.BlockID.IsZero() && !vote.BlockID.IsComplete() {
		return fmt.Errorf("blockID must be either empty or complete, got: %v", vote.BlockID)
	}
	if len(vote.ValidatorAddress) != crypto.AddressSize {
		return fmt.Errorf("expected ValidatorAddress size to be %d bytes, got %d bytes",
			crypto.AddressSize,
			len(vote.ValidatorAddress),
		)
	}
	if len(vote.Signature) == 0 {
		return errors.New("signature is missing")
	}
	if len(vote.Signature) > MaxSignatureSize {
		return fmt.Errorf("signature is too big (max: %d)", MaxSignatureSize)
	}
	return nil
}

// ToProto converts the handwritten type to proto generated type
// return type, nil if everything converts safely, otherwise nil, error
func (vote *Vote) ToProto() *tmproto.Vote {

	protoVote := tmproto.Vote{
		Type:   vote.Type,
		Height: vote.Height,
		Round:  vote.Round,
		BlockID: tmproto.BlockID{
			Hash: vote.BlockID.Hash,
			PartsHeader: tmproto.PartSetHeader{
				Total: vote.BlockID.PartsHeader.Total,
				Hash:  vote.BlockID.PartsHeader.Hash,
			},
		},
		Timestamp:        vote.Timestamp,
		ValidatorAddress: vote.ValidatorAddress,
		ValidatorIndex:   vote.ValidatorIndex,
		Signature:        vote.Signature,
	}

	return &protoVote
}

//FromProto converts a proto generetad type to a handwritten type
// return type, nil if everything converts safely, otherwise nil, error
func (vote *Vote) FromProto(pv tmproto.Vote) error {
	vote.Type = pv.Type
	vote.Height = pv.Height
	vote.Round = pv.Round
	vote.BlockID = BlockID{
		Hash: pv.BlockID.Hash,
		PartsHeader: PartSetHeader{
			Total: pv.BlockID.PartsHeader.GetTotal(),
			Hash:  pv.BlockID.PartsHeader.GetHash(),
		},
	}
	vote.Timestamp = pv.Timestamp
	vote.ValidatorAddress = pv.ValidatorAddress
	vote.ValidatorIndex = pv.ValidatorIndex
	vote.Signature = pv.Signature

	if err := vote.ValidateBasic(); err != nil {
		return err
	}

	return nil
}
