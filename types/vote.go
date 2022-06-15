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
	absentVoteStr string = "Vote{absent}"
	nilVoteStr    string = "nil"

	// The maximum supported number of bytes in a vote extension.
	MaxVoteExtensionSize int = 1024 * 1024
)

var (
	ErrVoteUnexpectedStep            = errors.New("unexpected step")
	ErrVoteInvalidValidatorIndex     = errors.New("invalid validator index")
	ErrVoteInvalidValidatorAddress   = errors.New("invalid validator address")
	ErrVoteInvalidSignature          = errors.New("invalid signature")
	ErrVoteInvalidBlockHash          = errors.New("invalid block hash")
	ErrVoteNonDeterministicSignature = errors.New("non-deterministic signature")
	ErrVoteNil                       = errors.New("nil vote")
	ErrVoteExtensionAbsent           = errors.New("vote extension absent")
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

// Vote represents a prevote, precommit, or commit vote from validators for
// consensus.
type Vote struct {
	Type               tmproto.SignedMsgType `json:"type"`
	Height             int64                 `json:"height,string"`
	Round              int32                 `json:"round"`    // assume there will not be greater than 2_147_483_647 rounds
	BlockID            BlockID               `json:"block_id"` // zero if vote is nil.
	Timestamp          time.Time             `json:"timestamp"`
	ValidatorAddress   Address               `json:"validator_address"`
	ValidatorIndex     int32                 `json:"validator_index"`
	Signature          []byte                `json:"signature"`
	Extension          []byte                `json:"extension"`
	ExtensionSignature []byte                `json:"extension_signature"`
}

// VoteFromProto attempts to convert the given serialization (Protobuf) type to
// our Vote domain type. No validation is performed on the resulting vote -
// this is left up to the caller to decide whether to call ValidateBasic or
// ValidateWithExtension.
func VoteFromProto(pv *tmproto.Vote) (*Vote, error) {
	blockID, err := BlockIDFromProto(&pv.BlockID)
	if err != nil {
		return nil, err
	}

	return &Vote{
		Type:               pv.Type,
		Height:             pv.Height,
		Round:              pv.Round,
		BlockID:            *blockID,
		Timestamp:          pv.Timestamp,
		ValidatorAddress:   pv.ValidatorAddress,
		ValidatorIndex:     pv.ValidatorIndex,
		Signature:          pv.Signature,
		Extension:          pv.Extension,
		ExtensionSignature: pv.ExtensionSignature,
	}, nil
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
	}
}

// StripExtension removes any extension data from the vote. Useful if the
// chain has not enabled vote extensions.
// Returns true if extension data was present before stripping and false otherwise.
func (vote *Vote) StripExtension() bool {
	stripped := len(vote.Extension) > 0 || len(vote.ExtensionSignature) > 0
	vote.Extension = nil
	vote.ExtensionSignature = nil
	return stripped
}

// ExtendedCommitSig attempts to construct an ExtendedCommitSig from this vote.
// Panics if either the vote extension signature is missing or if the block ID
// is not either empty or complete.
func (vote *Vote) ExtendedCommitSig() ExtendedCommitSig {
	if vote == nil {
		return NewExtendedCommitSigAbsent()
	}

	return ExtendedCommitSig{
		CommitSig:          vote.CommitSig(),
		Extension:          vote.Extension,
		ExtensionSignature: vote.ExtensionSignature,
	}
}

// VoteSignBytes returns the proto-encoding of the canonicalized Vote, for
// signing. Panics if the marshaling fails.
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

// VoteExtensionSignBytes returns the proto-encoding of the canonicalized vote
// extension for signing. Panics if the marshaling fails.
//
// Similar to VoteSignBytes, the encoded Protobuf message is varint
// length-prefixed for backwards-compatibility with the Amino encoding.
func VoteExtensionSignBytes(chainID string, vote *tmproto.Vote) []byte {
	pb := CanonicalizeVoteExtension(chainID, vote)
	bz, err := protoio.MarshalDelimited(&pb)
	if err != nil {
		panic(err)
	}

	return bz
}

func (vote *Vote) Copy() *Vote {
	voteCopy := *vote
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
		return absentVoteStr
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

	var blockHashString string
	if len(vote.BlockID.Hash) > 0 {
		blockHashString = fmt.Sprintf("%X", tmbytes.Fingerprint(vote.BlockID.Hash))
	} else {
		blockHashString = nilVoteStr
	}

	return fmt.Sprintf("Vote{%v:%X %v/%d %s %s %X %d @ %s}",
		vote.ValidatorIndex,
		tmbytes.Fingerprint(vote.ValidatorAddress),
		vote.Height,
		vote.Round,
		typeString,
		blockHashString,
		tmbytes.Fingerprint(vote.Signature),
		len(vote.Extension),
		CanonicalTime(vote.Timestamp),
	)
}

func (vote *Vote) verifyAndReturnProto(chainID string, pubKey crypto.PubKey) (*tmproto.Vote, error) {
	if !bytes.Equal(pubKey.Address(), vote.ValidatorAddress) {
		return nil, ErrVoteInvalidValidatorAddress
	}
	v := vote.ToProto()
	if !pubKey.VerifySignature(VoteSignBytes(chainID, v), vote.Signature) {
		return nil, ErrVoteInvalidSignature
	}
	return v, nil
}

// Verify checks whether the signature associated with this vote corresponds to
// the given chain ID and public key. This function does not validate vote
// extension signatures - to do so, use VerifyWithExtension instead.
func (vote *Vote) Verify(chainID string, pubKey crypto.PubKey) error {
	_, err := vote.verifyAndReturnProto(chainID, pubKey)
	return err
}

// VerifyVoteAndExtension performs the same verification as Verify, but
// additionally checks whether the vote extension signature corresponds to the
// given chain ID and public key. We only verify vote extension signatures for
// precommits.
func (vote *Vote) VerifyVoteAndExtension(chainID string, pubKey crypto.PubKey) error {
	v, err := vote.verifyAndReturnProto(chainID, pubKey)
	if err != nil {
		return err
	}
	// We only verify vote extension signatures for non-nil precommits.
	if vote.Type == tmproto.PrecommitType && !ProtoBlockIDIsNil(&v.BlockID) {
		extSignBytes := VoteExtensionSignBytes(chainID, v)
		if !pubKey.VerifySignature(extSignBytes, vote.ExtensionSignature) {
			return ErrVoteInvalidSignature
		}
	}
	return nil
}

// VerifyExtension checks whether the vote extension signature corresponds to the
// given chain ID and public key.
func (vote *Vote) VerifyExtension(chainID string, pubKey crypto.PubKey) error {
	if vote.Type != tmproto.PrecommitType || vote.BlockID.IsNil() {
		return nil
	}
	v := vote.ToProto()
	extSignBytes := VoteExtensionSignBytes(chainID, v)
	if !pubKey.VerifySignature(extSignBytes, vote.ExtensionSignature) {
		return ErrVoteInvalidSignature
	}
	return nil
}

// ValidateBasic checks whether the vote is well-formed. It does not, however,
// check vote extensions - for vote validation with vote extension validation,
// use ValidateWithExtension.
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

	// We should only ever see vote extensions in non-nil precommits, otherwise
	// this is a violation of the specification.
	// https://github.com/tendermint/tendermint/issues/8487
	if vote.Type != tmproto.PrecommitType || (vote.Type == tmproto.PrecommitType && vote.BlockID.IsNil()) {
		if len(vote.Extension) > 0 {
			return errors.New("unexpected vote extension")
		}
		if len(vote.ExtensionSignature) > 0 {
			return errors.New("unexpected vote extension signature")
		}
	}

	if vote.Type == tmproto.PrecommitType && !vote.BlockID.IsNil() {
		if len(vote.ExtensionSignature) > MaxSignatureSize {
			return fmt.Errorf("vote extension signature is too big (max: %d)", MaxSignatureSize)
		}
		if len(vote.ExtensionSignature) == 0 && len(vote.Extension) != 0 {
			return fmt.Errorf("vote extension signature absent on vote with extension")
		}
	}

	return nil
}

// EnsureExtension checks for the presence of extensions signature data
// on precommit vote types.
func (vote *Vote) EnsureExtension() error {
	// We should always see vote extension signatures in non-nil precommits
	if vote.Type != tmproto.PrecommitType {
		return nil
	}
	if vote.BlockID.IsNil() {
		return nil
	}
	if len(vote.ExtensionSignature) > 0 {
		return nil
	}
	return ErrVoteExtensionAbsent
}

// ToProto converts the handwritten type to proto generated type
// return type, nil if everything converts safely, otherwise nil, error
func (vote *Vote) ToProto() *tmproto.Vote {
	if vote == nil {
		return nil
	}

	return &tmproto.Vote{
		Type:               vote.Type,
		Height:             vote.Height,
		Round:              vote.Round,
		BlockID:            vote.BlockID.ToProto(),
		Timestamp:          vote.Timestamp,
		ValidatorAddress:   vote.ValidatorAddress,
		ValidatorIndex:     vote.ValidatorIndex,
		Signature:          vote.Signature,
		Extension:          vote.Extension,
		ExtensionSignature: vote.ExtensionSignature,
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
