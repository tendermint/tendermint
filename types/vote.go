package types

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

const (
	nilVoteStr string = "nil-Vote"
)

var (
	ErrVoteUnexpectedStep            = errors.New("unexpected step")
	ErrVoteInvalidValidatorIndex     = errors.New("invalid validator index")
	ErrVoteInvalidValidatorAddress   = errors.New("invalid validator address")
	ErrVoteInvalidSignature          = errors.New("invalid signature")
	ErrVoteInvalidBlockHash          = errors.New("invalid block hash")
	ErrVoteNonDeterministicSignature = errors.New("non-deterministic signature")
	ErrVoteNil                       = errors.New("nil vote")
	ErrVoteInvalidExtension          = errors.New("invalid vote extension")
)

type ErrVoteConflictingVotes struct {
	VoteA *Vote
	VoteB *Vote
}

func (err *ErrVoteConflictingVotes) Error() string {
	return fmt.Sprintf("conflicting votes from validator %X", err.VoteA.ValidatorAddress)
}

func NewConflictingVoteError(vote1, vote2 *Vote) *ErrVoteConflictingVotes {
	return &ErrVoteConflictingVotes{
		VoteA: vote1,
		VoteB: vote2,
	}
}

// Address is hex bytes.
type Address = crypto.Address

// VoteExtensionToSign is a subset of VoteExtension
// that is signed by the validators private key
type VoteExtensionToSign struct {
	AppDataToSign []byte `json:"app_data_to_sign"`
}

func (ext VoteExtensionToSign) ToProto() *tmproto.VoteExtensionToSign {
	if ext.IsEmpty() {
		return nil
	}
	return &tmproto.VoteExtensionToSign{
		AppDataToSign: ext.AppDataToSign,
	}
}

func VoteExtensionToSignFromProto(pext *tmproto.VoteExtensionToSign) VoteExtensionToSign {
	if pext == nil {
		return VoteExtensionToSign{}
	}
	return VoteExtensionToSign{
		AppDataToSign: pext.AppDataToSign,
	}
}

func (ext VoteExtensionToSign) IsEmpty() bool {
	return len(ext.AppDataToSign) == 0
}

// BytesPacked returns a bytes-packed representation for
// debugging and human identification. This function should
// not be used for any logical operations.
func (ext VoteExtensionToSign) BytesPacked() []byte {
	res := []byte{}
	res = append(res, ext.AppDataToSign...)
	return res
}

// ToVoteExtension constructs a VoteExtension from a VoteExtensionToSign
func (ext VoteExtensionToSign) ToVoteExtension() VoteExtension {
	return VoteExtension{
		AppDataToSign: ext.AppDataToSign,
	}
}

// VoteExtension is a set of data provided by the application
// that is additionally included in the vote
type VoteExtension struct {
	AppDataToSign             []byte `json:"app_data_to_sign"`
	AppDataSelfAuthenticating []byte `json:"app_data_self_authenticating"`
}

// ToSign constructs a VoteExtensionToSign from a VoteExtenstion
func (ext VoteExtension) ToSign() VoteExtensionToSign {
	return VoteExtensionToSign{
		AppDataToSign: ext.AppDataToSign,
	}
}

// BytesPacked returns a bytes-packed representation for
// debugging and human identification. This function should
// not be used for any logical operations.
func (ext VoteExtension) BytesPacked() []byte {
	res := []byte{}
	res = append(res, ext.AppDataToSign...)
	res = append(res, ext.AppDataSelfAuthenticating...)
	return res
}

// Vote represents a prevote, precommit, or commit vote from validators for
// consensus.
type Vote struct {
	Type             tmproto.SignedMsgType `json:"type"`
	Height           int64                 `json:"height,string"`
	Round            int32                 `json:"round"`    // assume there will not be greater than 2_147_483_647 rounds
	BlockID          BlockID               `json:"block_id"` // zero if vote is nil.
	Timestamp        time.Time             `json:"timestamp"`
	ValidatorAddress Address               `json:"validator_address"`
	ValidatorIndex   int32                 `json:"validator_index"`
	Signature        []byte                `json:"signature"`
	VoteExtension    VoteExtension         `json:"vote_extension"`
}

// CommitSig converts the Vote to a CommitSig.
func (vote *Vote) CommitSig() CommitSig {
	if vote == nil {
		return NewCommitSigAbsent()
	}

	var blockIDFlag BlockIDFlag
	switch {
	case vote.BlockID.IsComplete():
		blockIDFlag = BlockIDFlagCommit
	case vote.BlockID.IsNil():
		blockIDFlag = BlockIDFlagNil
	default:
		panic(fmt.Sprintf("Invalid vote %v - expected BlockID to be either empty or complete", vote))
	}

	return CommitSig{
		BlockIDFlag:      blockIDFlag,
		ValidatorAddress: vote.ValidatorAddress,
		Timestamp:        vote.Timestamp,
		Signature:        vote.Signature,
		VoteExtension:    vote.VoteExtension.ToSign(),
	}
}

// VoteSignBytes returns the proto-encoding of the canonicalized Vote, for
// signing. Panics is the marshaling fails.
//
// The encoded Protobuf message is varint length-prefixed (using MarshalDelimited)
// for backwards-compatibility with the Amino encoding, due to e.g. hardware
// devices that rely on this encoding.
//
// See CanonicalizeVote
func VoteSignBytes(chainID string, vote *tmproto.Vote) []byte {
	pb := CanonicalizeVote(chainID, vote)
	bz, err := protoio.MarshalDelimited(&pb)
	if err != nil {
		panic(err)
	}

	return bz
}

func (vote *Vote) Copy() *Vote {
	voteCopy := *vote
	voteCopy.VoteExtension = vote.VoteExtension.Copy()
	return &voteCopy
}

// String returns a string representation of Vote.
//
// 1. validator index
// 2. first 6 bytes of validator address
// 3. height
// 4. round,
// 5. type byte
// 6. type string
// 7. first 6 bytes of block hash
// 8. first 6 bytes of signature
// 9. first 6 bytes of vote extension
// 10. timestamp
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

	return fmt.Sprintf("Vote{%v:%X %v/%02d/%v(%v) %X %X %X @ %s}",
		vote.ValidatorIndex,
		tmbytes.Fingerprint(vote.ValidatorAddress),
		vote.Height,
		vote.Round,
		vote.Type,
		typeString,
		tmbytes.Fingerprint(vote.BlockID.Hash),
		tmbytes.Fingerprint(vote.Signature),
		tmbytes.Fingerprint(vote.VoteExtension.BytesPacked()),
		CanonicalTime(vote.Timestamp),
	)
}

func (vote *Vote) Verify(chainID string, pubKey crypto.PubKey) error {
	if !bytes.Equal(pubKey.Address(), vote.ValidatorAddress) {
		return ErrVoteInvalidValidatorAddress
	}
	v := vote.ToProto()
	if !pubKey.VerifySignature(VoteSignBytes(chainID, v), vote.Signature) {
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
		return fmt.Errorf("wrong BlockID: %w", err)
	}

	// BlockID.ValidateBasic would not err if we for instance have an empty hash but a
	// non-empty PartsSetHeader:
	if !vote.BlockID.IsNil() && !vote.BlockID.IsComplete() {
		return fmt.Errorf("blockID must be either empty or complete, got: %v", vote.BlockID)
	}

	if len(vote.ValidatorAddress) != crypto.AddressSize {
		return fmt.Errorf("expected ValidatorAddress size to be %d bytes, got %d bytes",
			crypto.AddressSize,
			len(vote.ValidatorAddress),
		)
	}
	if vote.ValidatorIndex < 0 {
		return errors.New("negative ValidatorIndex")
	}
	if len(vote.Signature) == 0 {
		return errors.New("signature is missing")
	}

	if len(vote.Signature) > MaxSignatureSize {
		return fmt.Errorf("signature is too big (max: %d)", MaxSignatureSize)
	}

	// XXX: add length verification for vote extension?

	return nil
}

func (ext VoteExtension) Copy() VoteExtension {
	res := VoteExtension{
		AppDataToSign:             ext.AppDataToSign,
		AppDataSelfAuthenticating: ext.AppDataSelfAuthenticating,
	}
	return res
}

func (ext VoteExtension) IsEmpty() bool {
	if len(ext.AppDataToSign) != 0 {
		return false
	}
	if len(ext.AppDataSelfAuthenticating) != 0 {
		return false
	}
	return true
}

func (ext VoteExtension) ToProto() *tmproto.VoteExtension {
	if ext.IsEmpty() {
		return nil
	}

	return &tmproto.VoteExtension{
		AppDataToSign:             ext.AppDataToSign,
		AppDataSelfAuthenticating: ext.AppDataSelfAuthenticating,
	}
}

// ToProto converts the handwritten type to proto generated type
// return type, nil if everything converts safely, otherwise nil, error
func (vote *Vote) ToProto() *tmproto.Vote {
	if vote == nil {
		return nil
	}

	return &tmproto.Vote{
		Type:             vote.Type,
		Height:           vote.Height,
		Round:            vote.Round,
		BlockID:          vote.BlockID.ToProto(),
		Timestamp:        vote.Timestamp,
		ValidatorAddress: vote.ValidatorAddress,
		ValidatorIndex:   vote.ValidatorIndex,
		Signature:        vote.Signature,
		VoteExtension:    vote.VoteExtension.ToProto(),
	}
}

func VotesToProto(votes []*Vote) []*tmproto.Vote {
	if votes == nil {
		return nil
	}

	res := make([]*tmproto.Vote, 0, len(votes))
	for _, vote := range votes {
		v := vote.ToProto()
		// protobuf crashes when serializing "repeated" fields with nil elements
		if v != nil {
			res = append(res, v)
		}
	}
	return res
}

func VoteExtensionFromProto(pext *tmproto.VoteExtension) VoteExtension {
	ext := VoteExtension{}
	if pext != nil {
		ext.AppDataToSign = pext.AppDataToSign
		ext.AppDataSelfAuthenticating = pext.AppDataSelfAuthenticating
	}
	return ext
}

// FromProto converts a proto generetad type to a handwritten type
// return type, nil if everything converts safely, otherwise nil, error
func VoteFromProto(pv *tmproto.Vote) (*Vote, error) {
	if pv == nil {
		return nil, errors.New("nil vote")
	}

	blockID, err := BlockIDFromProto(&pv.BlockID)
	if err != nil {
		return nil, err
	}

	vote := new(Vote)
	vote.Type = pv.Type
	vote.Height = pv.Height
	vote.Round = pv.Round
	vote.BlockID = *blockID
	vote.Timestamp = pv.Timestamp
	vote.ValidatorAddress = pv.ValidatorAddress
	vote.ValidatorIndex = pv.ValidatorIndex
	vote.Signature = pv.Signature
	vote.VoteExtension = VoteExtensionFromProto(pv.VoteExtension)

	return vote, vote.ValidateBasic()
}
