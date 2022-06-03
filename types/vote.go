package types

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/rs/zerolog"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

const (
	nilVoteStr string = "nil-Vote"
	// MaxVoteBytes is a maximum vote size (including amino overhead).
	MaxVoteBytesBLS12381 int64 = 241
	MaxVoteBytesEd25519  int64 = 209
)

func MaxVoteBytesForKeyType(keyType crypto.KeyType) int64 {
	switch keyType {
	case crypto.Ed25519:
		return MaxVoteBytesEd25519
	case crypto.BLS12381:
		return MaxVoteBytesBLS12381
	}
	return MaxVoteBytesBLS12381
}

var (
	ErrVoteUnexpectedStep             = errors.New("unexpected step")
	ErrVoteInvalidValidatorIndex      = errors.New("invalid validator index")
	ErrVoteInvalidValidatorAddress    = errors.New("invalid validator address")
	ErrVoteInvalidSignature           = errors.New("invalid signature")
	ErrVoteInvalidBlockHash           = errors.New("invalid block hash")
	ErrVoteNonDeterministicSignature  = errors.New("non-deterministic signature")
	ErrVoteNil                        = errors.New("nil vote")
	ErrVoteInvalidExtension           = errors.New("invalid vote extension")
	ErrVoteInvalidValidatorProTxHash  = errors.New("invalid validator pro_tx_hash")
	ErrVoteInvalidValidatorPubKeySize = errors.New("invalid validator public key size")
	ErrVoteInvalidBlockSignature      = errors.New("invalid block signature")
	ErrVoteInvalidStateSignature      = errors.New("invalid state signature")
	ErrVoteStateSignatureShouldBeNil  = errors.New("state signature when voting for nil block")
)

type ErrVoteConflictingVotes struct {
	VoteA *Vote
	VoteB *Vote
}

func (err *ErrVoteConflictingVotes) Error() string {
	return fmt.Sprintf("conflicting votes from validator %X", err.VoteA.ValidatorProTxHash)
}

func NewConflictingVoteError(vote1, vote2 *Vote) *ErrVoteConflictingVotes {
	return &ErrVoteConflictingVotes{
		VoteA: vote1,
		VoteB: vote2,
	}
}

// Address is hex bytes.
type Address = crypto.Address

type ProTxHash = crypto.ProTxHash

// Vote represents a prevote, precommit, or commit vote from validators for
// consensus.
type Vote struct {
	Type               tmproto.SignedMsgType `json:"type"`
	Height             int64                 `json:"height,string"`
	Round              int32                 `json:"round"`    // assume there will not be greater than 2^32 rounds
	BlockID            BlockID               `json:"block_id"` // zero if vote is nil.
	ValidatorProTxHash ProTxHash             `json:"validator_pro_tx_hash"`
	ValidatorIndex     int32                 `json:"validator_index"`
	BlockSignature     tmbytes.HexBytes      `json:"block_signature"`
	StateSignature     tmbytes.HexBytes      `json:"state_signature"`
	VoteExtensions     []VoteExtension       `json:"vote_extensions"`
}

// VoteExtension represents a vote extension data, with possible types: default or threshold recover
type VoteExtension struct {
	Type      tmproto.VoteExtensionType `json:"type"`
	Extension []byte                    `json:"extension"`
	Signature tmbytes.HexBytes          `json:"signature"`
}

// IsRecoverable returns true if a vote extension has threshold-recover type
func (v *VoteExtension) IsRecoverable() bool {
	return v.Type == tmproto.VoteExtensionType_THRESHOLD_RECOVER
}

// Clone returns a copy of current vote-extension
func (v *VoteExtension) Clone() VoteExtension {
	return VoteExtension{
		Type:      v.Type,
		Extension: v.Extension,
		Signature: v.Signature,
	}
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
	v := &Vote{
		Type:               pv.Type,
		Height:             pv.Height,
		Round:              pv.Round,
		BlockID:            *blockID,
		ValidatorProTxHash: pv.ValidatorProTxHash,
		ValidatorIndex:     pv.ValidatorIndex,
		BlockSignature:     pv.BlockSignature,
		StateSignature:     pv.StateSignature,
	}
	if len(pv.VoteExtensions) > 0 {
		v.VoteExtensions = make([]VoteExtension, len(pv.VoteExtensions))
		for i, ext := range pv.VoteExtensions {
			v.VoteExtensions[i] = VoteExtension{
				Type:      ext.Type,
				Extension: ext.Extension,
				Signature: ext.Signature,
			}
		}
	}
	return v, nil
}

// VoteBlockSignBytes returns the proto-encoding of the canonicalized Vote, for
// signing. Panics is the marshaling fails.
//
// The encoded Protobuf message is varint length-prefixed (using MarshalDelimited)
// for backwards-compatibility with the Amino encoding, due to e.g. hardware
// devices that rely on this encoding.
//
// See CanonicalizeVote
func VoteBlockSignBytes(chainID string, vote *tmproto.Vote) []byte {
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
func VoteExtensionSignBytes(chainID string, height int64, round int32, ext *tmproto.VoteExtension) []byte {
	pb := CanonicalizeVoteExtension(chainID, ext, height, round)
	bz, err := protoio.MarshalDelimited(&pb)
	if err != nil {
		panic(err)
	}
	return bz
}

// VoteExtensionRequestID returns vote extension request ID
func VoteExtensionRequestID(vote *tmproto.Vote) []byte {
	return voteHeightRoundRequestID("dpevote", vote.Height, vote.Round)
}

// VoteBlockSignID returns signID that should be signed for the block
func VoteBlockSignID(chainID string, vote *tmproto.Vote, quorumType btcjson.LLMQType, quorumHash []byte) []byte {
	signID := MakeBlockSignItem(chainID, vote, quorumType, quorumHash)
	return signID.ID
}

func (vote *Vote) Copy() *Vote {
	voteCopy := *vote
	return &voteCopy
}

// String returns a string representation of Vote.
//
// 1. validator index
// 2. first 6 bytes of validator proTxHash
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

	ext := bytes.Join(collectExtensions(vote.VoteExtensions, false), nil)
	return fmt.Sprintf("Vote{%v:%X %v/%02d/%v(%v) %X %X %X %X}",
		vote.ValidatorIndex,
		tmbytes.Fingerprint(vote.ValidatorProTxHash),
		vote.Height,
		vote.Round,
		vote.Type,
		typeString,
		tmbytes.Fingerprint(vote.BlockID.Hash),
		tmbytes.Fingerprint(vote.BlockSignature),
		tmbytes.Fingerprint(vote.StateSignature),
		tmbytes.Fingerprint(ext),
	)
}

// VerifyWithExtension performs the same verification as Verify, but
// additionally checks whether the vote extension signature corresponds to the
// given chain ID and public key. We only verify vote extension signatures for
// precommits.
func (vote *Vote) VerifyWithExtension(
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	pubKey crypto.PubKey,
	proTxHash ProTxHash,
	stateID StateID,
) error {
	quorumSigns, err := MakeQuorumSigns(chainID, quorumType, quorumHash, vote.ToProto(), stateID)
	if err != nil {
		return err
	}
	err = vote.verifyBasic(proTxHash, pubKey, quorumSigns)
	if err != nil {
		return err
	}
	// We only verify vote extension signatures for precommits.
	if vote.Type == tmproto.PrecommitType {
		// TODO: Remove extension signature nil check to enforce vote extension
		//       signing once we resolve https://github.com/tendermint/tendermint/issues/8272
		for i, extSignItem := range quorumSigns.VoteExts {
			sign := vote.VoteExtensions[i].Signature
			if sign != nil && !pubKey.VerifySignatureDigest(extSignItem.ID, sign) {
				return ErrVoteInvalidSignature
			}
		}
	}
	return nil
}

func (vote *Vote) Verify(
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash []byte,
	pubKey crypto.PubKey,
	proTxHash crypto.ProTxHash,
	stateID StateID,
) error {
	quorumSigns, err := MakeQuorumSigns(chainID, quorumType, quorumHash, vote.ToProto(), stateID)
	if err != nil {
		return err
	}
	return vote.verifyBasic(proTxHash, pubKey, quorumSigns)
}

func (vote *Vote) verifyBasic(proTxHash ProTxHash, pubKey crypto.PubKey, quorumSigns QuorumSigns) error {
	if !bytes.Equal(proTxHash, vote.ValidatorProTxHash) {
		return ErrVoteInvalidValidatorProTxHash
	}
	if len(pubKey.Bytes()) != bls12381.PubKeySize {
		return ErrVoteInvalidValidatorPubKeySize
	}

	if !pubKey.VerifySignatureDigest(quorumSigns.Block.ID, vote.BlockSignature) {
		return fmt.Errorf(
			"%s proTxHash %s pubKey %v vote %v sign bytes %s block signature %s", ErrVoteInvalidBlockSignature.Error(),
			proTxHash.ShortString(), pubKey, vote, hex.EncodeToString([]byte{}), hex.EncodeToString(vote.BlockSignature))
	}

	// we must verify the stateID but only if the blockID isn't nil
	if vote.BlockID.Hash != nil {
		// fmt.Printf("state vote verify sign ID %s (%d - %s  - %s  - %s)\n", hex.EncodeToString(stateSignID), quorumType,
		//	hex.EncodeToString(quorumHash), hex.EncodeToString(stateRequestID), hex.EncodeToString(stateMessageHash))
		if !pubKey.VerifySignatureDigest(quorumSigns.State.ID, vote.StateSignature) {
			return ErrVoteInvalidStateSignature
		}
	} else if vote.StateSignature != nil {
		return ErrVoteStateSignatureShouldBeNil
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

	if len(vote.ValidatorProTxHash) != crypto.DefaultHashSize {
		return fmt.Errorf("expected ValidatorProTxHash size to be %d bytes, got %d bytes (%X)",
			crypto.DefaultHashSize,
			len(vote.ValidatorProTxHash),
			vote.ValidatorProTxHash.Bytes(),
		)
	}
	if vote.ValidatorIndex < 0 {
		return errors.New("negative ValidatorIndex")
	}
	if len(vote.BlockSignature) == 0 {
		return errors.New("block signature is missing")
	}

	if len(vote.BlockSignature) > SignatureSize {
		return fmt.Errorf("block signature is too big (max: %d)", SignatureSize)
	}

	if vote.BlockID.Hash != nil && len(vote.StateSignature) == 0 {
		return errors.New("state signature is missing for a block not voting nil")
	}

	if len(vote.StateSignature) > SignatureSize {
		return fmt.Errorf("state signature is too big (max: %d)", SignatureSize)
	}

	// We should only ever see vote extensions in precommits.
	if vote.Type != tmproto.PrecommitType {
		if len(vote.VoteExtensions) > 0 {
			return errors.New("unexpected vote extensions")
		}
	}

	return nil
}

// ValidateWithExtension performs the same validations as ValidateBasic, but
// additionally checks whether a vote extension signature is present. This
// function is used in places where vote extension signatures are expected.
func (vote *Vote) ValidateWithExtension() error {
	if err := vote.ValidateBasic(); err != nil {
		return err
	}

	// We should always see vote extension signatures in precommits
	if vote.Type == tmproto.PrecommitType {
		// TODO(thane): Remove extension length check once
		//              https://github.com/tendermint/tendermint/issues/8272 is
		//              resolved.
		for _, ext := range vote.VoteExtensions {
			if len(ext.Extension) > 0 && len(ext.Signature) == 0 {
				return errors.New("vote extension signature is missing")
			}
			if len(ext.Signature) > SignatureSize {
				return fmt.Errorf("vote extension signature is too big (max: %d)", SignatureSize)
			}
		}
	}

	return nil
}

// ToProto converts the handwritten type to proto generated type
// return type, nil if everything converts safely, otherwise nil, error
func (vote *Vote) ToProto() *tmproto.Vote {
	if vote == nil {
		return nil
	}
	exts := make([]*tmproto.VoteExtension, len(vote.VoteExtensions))
	for i, ext := range vote.VoteExtensions {
		exts[i] = &tmproto.VoteExtension{
			Type:      ext.Type,
			Extension: ext.Extension,
			Signature: ext.Signature,
		}
	}
	return &tmproto.Vote{
		Type:               vote.Type,
		Height:             vote.Height,
		Round:              vote.Round,
		BlockID:            vote.BlockID.ToProto(),
		ValidatorProTxHash: vote.ValidatorProTxHash,
		ValidatorIndex:     vote.ValidatorIndex,
		BlockSignature:     vote.BlockSignature,
		StateSignature:     vote.StateSignature,
		VoteExtensions:     exts,
	}
}

// MarshalZerologObject formats this object for logging purposes
func (vote *Vote) MarshalZerologObject(e *zerolog.Event) {
	if vote == nil {
		return
	}
	e.Str("vote", vote.String())
	e.Int64("height", vote.Height)
	e.Int32("round", vote.Round)
	e.Str("type", vote.Type.String())
	e.Str("block_key", vote.BlockID.String())
	e.Str("block_signature", vote.BlockSignature.ShortString())
	e.Str("state_signature", vote.StateSignature.ShortString())
	e.Str("val_proTxHash", vote.ValidatorProTxHash.ShortString())
	e.Int32("val_index", vote.ValidatorIndex)
	e.Bool("nil", vote.BlockID.IsNil())
}

func (vote *Vote) HasVoteMessage() *tmcons.HasVote {
	return &tmcons.HasVote{
		Height: vote.Height,
		Round:  vote.Round,
		Type:   vote.Type,
		Index:  vote.ValidatorIndex,
	}
}

func VoteBlockRequestIDProto(vote *tmproto.Vote) []byte {
	return voteHeightRoundRequestID("dpbvote", vote.Height, vote.Round)
}

func voteHeightRoundRequestID(prefix string, height int64, round int32) []byte {
	reqID := []byte(prefix)
	heightBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightBytes, uint64(height))
	roundBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(roundBytes, uint32(round))
	reqID = append(reqID, heightBytes...)
	reqID = append(reqID, roundBytes...)
	return crypto.Checksum(reqID)
}

func makeSignID(signBytes, reqID []byte, quorumType btcjson.LLMQType, quorumHash []byte) []byte {
	msgHash := crypto.Checksum(signBytes)
	return crypto.SignID(
		quorumType,
		tmbytes.Reverse(quorumHash),
		tmbytes.Reverse(reqID),
		tmbytes.Reverse(msgHash),
	)
}

// ThresholdVoteSigns holds all created signatures, block, state and for each recovered vote-extensions
type ThresholdVoteSigns struct {
	BlockSign    []byte
	StateSign    []byte
	VoteExtSigns [][]byte
}

// QuorumVoteSigns is used to combine threshold signatures and quorum-hash that were used
type QuorumVoteSigns struct {
	ThresholdVoteSigns
	QuorumHash []byte
}

// ProtoToVoteExtensions ...
func ProtoToVoteExtensions(protoExts []*tmproto.VoteExtension) []VoteExtension {
	exts := make([]VoteExtension, len(protoExts))
	for i, ext := range protoExts {
		exts[i] = VoteExtension{
			Type:      ext.Type,
			Extension: ext.Extension,
			Signature: ext.Signature,
		}
	}
	return exts
}

func recoverableVoteExtensionIndexes(votes []*Vote) []int {
	var indexes []int
	for _, vote := range votes {
		if vote == nil {
			continue
		}
		for i, ext := range vote.VoteExtensions {
			if ext.IsRecoverable() {
				indexes = append(indexes, i)
			}
		}
		if len(indexes) > 0 {
			return indexes
		}
	}
	return nil
}

func collectExtensions(voteExtensions []VoteExtension, onlyRecoverable bool) [][]byte {
	exts := make([][]byte, len(voteExtensions))
	for i, ext := range voteExtensions {
		if onlyRecoverable {
			if !ext.IsRecoverable() {
				continue
			}
		}
		exts[i] = ext.Extension
	}
	return exts
}

// GetFirstVote returns the first not nil vote from a list, otherwise error
func GetFirstVote(votes []*Vote) (*Vote, error) {
	for _, vote := range votes {
		if vote != nil {
			return vote, nil
		}
	}
	return nil, errors.New("unable to get a vote, votes list is empty")
}
