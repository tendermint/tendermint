package types

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/protoio"
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
	ErrVoteInvalidValidatorProTxHash  = errors.New("invalid validator pro_tx_hash")
	ErrVoteInvalidValidatorPubKeySize = errors.New("invalid validator public key size")
	ErrVoteInvalidBlockSignature      = errors.New("invalid block signature")
	ErrVoteInvalidStateSignature      = errors.New("invalid state signature")
	ErrVoteInvalidBlockHash           = errors.New("invalid block hash")
	ErrVoteNonDeterministicSignature  = errors.New("non-deterministic signature")
	ErrVoteNil                        = errors.New("nil vote")
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
	Height             int64                 `json:"height"`
	Round              int32                 `json:"round"`    // assume there will not be greater than 2^32 rounds
	BlockID            BlockID               `json:"block_id"` // zero if vote is nil.
	StateID            StateID               `json:"state_id"`
	ValidatorProTxHash ProTxHash             `json:"validator_pro_tx_hash"`
	ValidatorIndex     int32                 `json:"validator_index"`
	BlockSignature     []byte                `json:"block_signature"`
	StateSignature     []byte                `json:"state_signature"`
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

// VoteBlockSignId returns signId that should be signed for the block
func VoteBlockSignId(chainID string, vote *tmproto.Vote, quorumType btcjson.LLMQType, quorumHash []byte) []byte {
	blockSignBytes := VoteBlockSignBytes(chainID, vote)

	blockMessageHash := crypto.Sha256(blockSignBytes)

	blockRequestId := VoteBlockRequestIdProto(vote)

	blockSignId := crypto.SignId(quorumType, bls12381.ReverseBytes(quorumHash), bls12381.ReverseBytes(blockRequestId), bls12381.ReverseBytes(blockMessageHash))

	return blockSignId
}

// VoteStateSignBytes returns the proto-encoding of the canonicalized last app hash state, for
// signing. Panics is the marshaling fails.
//
// The encoded Protobuf message is varint length-prefixed (using MarshalDelimited)
// for backwards-compatibility with the Amino encoding, due to e.g. hardware
// devices that rely on this encoding.
//
// See CanonicalizeVote
func VoteStateSignBytes(chainID string, vote *tmproto.Vote) []byte {
	pb := CanonicalizeStateVote(vote)
	bz, err := protoio.MarshalDelimited(&pb)
	if err != nil {
		panic(err)
	}

	return bz
}

// VoteStateSignId returns signId that should be signed for the state
func VoteStateSignId(chainID string, vote *tmproto.Vote, quorumType btcjson.LLMQType, quorumHash []byte) []byte {
	stateSignBytes := VoteStateSignBytes(chainID, vote)

	if stateSignBytes == nil {
		return nil
	}

	stateMessageHash := crypto.Sha256(stateSignBytes)

	stateRequestId := VoteStateRequestIdProto(vote)

	stateSignId := crypto.SignId(quorumType, bls12381.ReverseBytes(quorumHash), bls12381.ReverseBytes(stateRequestId), bls12381.ReverseBytes(stateMessageHash))

	return stateSignId
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
// 9. timestamp
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

	return fmt.Sprintf("Vote{%v:%X %v/%02d/%v(%v) %X %X %X %X}",
		vote.ValidatorIndex,
		tmbytes.Fingerprint(vote.ValidatorProTxHash),
		vote.Height,
		vote.Round,
		vote.Type,
		typeString,
		tmbytes.Fingerprint(vote.BlockID.Hash),
		tmbytes.Fingerprint(vote.BlockSignature),
		tmbytes.Fingerprint(vote.StateID.LastAppHash),
		tmbytes.Fingerprint(vote.StateSignature),
	)
}

func VoteBlockRequestId(vote *Vote) []byte {
	requestIdMessage := []byte("dpbvote")
	heightByteArray := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightByteArray, uint64(vote.Height))
	roundByteArray := make([]byte, 4)
	binary.LittleEndian.PutUint32(roundByteArray, uint32(vote.Round))

	requestIdMessage = append(requestIdMessage, heightByteArray...)
	requestIdMessage = append(requestIdMessage, roundByteArray...)

	return crypto.Sha256(requestIdMessage)
}

func VoteBlockRequestIdProto(vote *tmproto.Vote) []byte {
	requestIdMessage := []byte("dpbvote")
	heightByteArray := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightByteArray, uint64(vote.Height))
	roundByteArray := make([]byte, 4)
	binary.LittleEndian.PutUint32(roundByteArray, uint32(vote.Round))

	requestIdMessage = append(requestIdMessage, heightByteArray...)
	requestIdMessage = append(requestIdMessage, roundByteArray...)

	return crypto.Sha256(requestIdMessage)
}

func VoteStateRequestId(vote *Vote) []byte {
	requestIdMessage := []byte("dpsvote")
	heightByteArray := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightByteArray, uint64(vote.Height))
	roundByteArray := make([]byte, 4)
	binary.LittleEndian.PutUint32(roundByteArray, uint32(vote.Round))

	requestIdMessage = append(requestIdMessage, heightByteArray...)
	requestIdMessage = append(requestIdMessage, roundByteArray...)

	return crypto.Sha256(requestIdMessage)
}

func VoteStateRequestIdProto(vote *tmproto.Vote) []byte {
	requestIdMessage := []byte("dpsvote")
	heightByteArray := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightByteArray, uint64(vote.Height))
	roundByteArray := make([]byte, 4)
	binary.LittleEndian.PutUint32(roundByteArray, uint32(vote.Round))

	requestIdMessage = append(requestIdMessage, heightByteArray...)
	requestIdMessage = append(requestIdMessage, roundByteArray...)

	return crypto.Sha256(requestIdMessage)
}

func (vote *Vote) Verify(chainID string, quorumType btcjson.LLMQType, quorumHash []byte, pubKey crypto.PubKey, proTxHash crypto.ProTxHash) error {
	if !bytes.Equal(proTxHash, vote.ValidatorProTxHash) {
		return ErrVoteInvalidValidatorProTxHash
	}
	if len(pubKey.Bytes()) != bls12381.PubKeySize {
		return ErrVoteInvalidValidatorPubKeySize
	}
	v := vote.ToProto()
	voteBlockSignBytes := VoteBlockSignBytes(chainID, v)

	blockMessageHash := crypto.Sha256(voteBlockSignBytes)

	blockRequestId := VoteBlockRequestId(vote)

	signId := crypto.SignId(quorumType, bls12381.ReverseBytes(quorumHash), bls12381.ReverseBytes(blockRequestId), bls12381.ReverseBytes(blockMessageHash))

	// fmt.Printf("block vote verify sign Id %s (%d - %s  - %s  - %s)\n", hex.EncodeToString(signId), quorumType,
	//	hex.EncodeToString(quorumHash), hex.EncodeToString(blockRequestId), hex.EncodeToString(blockMessageHash))

	if !pubKey.VerifySignatureDigest(signId, vote.BlockSignature) {
		return fmt.Errorf("%s proTxHash %s pubKey %v vote %v sign bytes %s block signature %s", ErrVoteInvalidBlockSignature.Error(),
			proTxHash, pubKey, vote, hex.EncodeToString(voteBlockSignBytes), hex.EncodeToString(vote.BlockSignature))
	}

	// we must verify the stateID but only if the blockID isn't nil
	if vote.BlockID.Hash != nil {
		voteStateSignBytes := VoteStateSignBytes(chainID, v)
		stateMessageHash := crypto.Sha256(voteStateSignBytes)

		stateRequestId := VoteStateRequestId(vote)

		stateSignId := crypto.SignId(quorumType, bls12381.ReverseBytes(quorumHash), bls12381.ReverseBytes(stateRequestId), bls12381.ReverseBytes(stateMessageHash))

		// fmt.Printf("state vote verify sign Id %s (%d - %s  - %s  - %s)\n", hex.EncodeToString(stateSignId), quorumType,
		//	hex.EncodeToString(quorumHash), hex.EncodeToString(stateRequestId), hex.EncodeToString(stateMessageHash))

		if !pubKey.VerifySignatureDigest(stateSignId, vote.StateSignature) {
			return ErrVoteInvalidStateSignature
		}
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

	if err := vote.StateID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong StateID: %v", err)
	}

	// BlockID.ValidateBasic would not err if we for instance have an empty hash but a
	// non-empty PartsSetHeader:
	if !vote.BlockID.IsZero() && !vote.BlockID.IsComplete() {
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

	return nil
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
		StateID:            vote.StateID.ToProto(),
		ValidatorProTxHash: vote.ValidatorProTxHash,
		ValidatorIndex:     vote.ValidatorIndex,
		BlockSignature:     vote.BlockSignature,
		StateSignature:     vote.StateSignature,
	}
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

	stateID, err := StateIDFromProto(&pv.StateID)
	if err != nil {
		return nil, err
	}

	vote := new(Vote)
	vote.Type = pv.Type
	vote.Height = pv.Height
	vote.Round = pv.Round
	vote.BlockID = *blockID
	vote.StateID = *stateID
	vote.ValidatorProTxHash = pv.ValidatorProTxHash
	vote.ValidatorIndex = pv.ValidatorIndex
	vote.BlockSignature = pv.BlockSignature
	vote.StateSignature = pv.StateSignature

	return vote, vote.ValidateBasic()
}
