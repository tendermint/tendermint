package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// This file contains implementation of StateID logic.

//--------------------------------------------------------------------------------

// StateID
// TODO: Move to separate file
type StateID struct {

	// Height of last committed block
	Height int64 `json:"height"`
	// LastAppHash used in last committed block
	LastAppHash tmbytes.HexBytes `json:"last_app_hash"`
}

// Copy returns new StateID that is equal to this one
func (stateID StateID) Copy() StateID {
	appHash := make([]byte, len(stateID.LastAppHash))
	if copy(appHash, stateID.LastAppHash) != len(stateID.LastAppHash) {
		panic("Cannot copy LastAppHash, this should never happen. Out of memory???")
	}

	return StateID{
		Height:      stateID.Height,
		LastAppHash: appHash,
	}
}

// Equals returns true if the StateID matches the given StateID
func (stateID StateID) Equals(other StateID) bool {
	return stateID.Height == other.Height &&
		bytes.Equal(stateID.LastAppHash, other.LastAppHash)
}

// Key returns a machine-readable string representation of the StateID
func (stateID StateID) Key() string {
	return strconv.FormatInt(stateID.Height, 36) + string(stateID.LastAppHash)
}

// ValidateBasic performs basic validation.
func (stateID StateID) ValidateBasic() error {
	// LastAppHash can be empty in case of genesis block.
	if err := ValidateAppHash(stateID.LastAppHash); err != nil {
		return fmt.Errorf("wrong app Hash")
	}

	if stateID.Height < 0 {
		return fmt.Errorf("stateID height is not valid: %d < 0", stateID.Height)
	}

	return nil
}

func (stateID StateID) Signable() Signable {
	return stateID
}

// SignBytes returns bytes that should be signed
func (stateID StateID) SignBytes(chainID string) []byte {

	return StateIDSignBytesProto(chainID, stateID.ToProto())
}

// SignID returns sign ID for provided state
func (stateID StateID) SignID(chainID string, quorumType btcjson.LLMQType, quorumHash []byte) []byte {
	return StateIDSignIDProto(chainID, stateID.ToProto(), quorumType, quorumHash)
}

func (stateID StateID) SignRequestID() []byte {
	return StateIDRequestIDProto(stateID.ToProto())
}

// String returns a human readable string representation of the StateID.
//
// 1. hash
//
func (stateID StateID) String() string {
	return fmt.Sprintf(`%d:%v`, stateID.Height, stateID.LastAppHash)
}

// ToProto converts StateID to protobuf
func (stateID StateID) ToProto() tmproto.StateID {
	return tmproto.StateID{
		LastAppHash: stateID.LastAppHash,
		Height:      stateID.Height,
	}
}

// WithHeight returns new copy of stateID with height set to provided value.
// It is a convenience method used in tests.
// Note that this is Last Height from state, so it will be (height-1) for Vote.
func (stateID StateID) WithHeight(height int64) StateID {
	ret := stateID.Copy()
	ret.Height = height

	return ret
}

// ******** PROTOBUF FUNCTIONS ********* //

// FromProto sets a protobuf BlockID to the given pointer.
// It returns an error if the block id is invalid.
func StateIDFromProto(sID *tmproto.StateID) (*StateID, error) {
	if sID == nil {
		return nil, errors.New("nil StateID")
	}

	stateID := new(StateID)

	stateID.LastAppHash = sID.LastAppHash
	stateID.Height = sID.Height

	return stateID, stateID.ValidateBasic()
}

// StateIDSignBytesProto returns bytes that should be signed for provided protobuf StateID
// See also: StateID.SignBytes()
// TODO why we don't simply use stateID.Marshal() ?
func StateIDSignBytesProto(chainID string, stateID tmproto.StateID) []byte {

	bz := make([]byte, 8)
	binary.LittleEndian.PutUint64(bz, uint64(stateID.Height))

	lastAppHash := make([]byte, len(stateID.LastAppHash))
	copy(lastAppHash, stateID.LastAppHash)
	bz = append(bz, lastAppHash...)
	return bz
}

// StateIDSignIDProto returns signID that should be used to sign the protobuf stateID
func StateIDSignIDProto(chainID string, stateID tmproto.StateID,
	quorumType btcjson.LLMQType, quorumHash []byte) []byte {

	stateSignBytes := StateIDSignBytesProto(chainID, stateID)

	if stateSignBytes == nil {
		return nil
	}

	stateMessageHash := crypto.Sha256(stateSignBytes)

	stateRequestID := StateIDRequestIDProto(stateID)

	stateSignID := crypto.SignID(
		quorumType,
		bls12381.ReverseBytes(quorumHash),
		bls12381.ReverseBytes(stateRequestID),
		bls12381.ReverseBytes(stateMessageHash),
	)

	return stateSignID
}

func StateIDRequestIDProto(stateID tmproto.StateID) []byte {
	requestIDMessage := []byte("dpsvote")
	heightByteArray := make([]byte, 8)

	binary.LittleEndian.PutUint64(heightByteArray, uint64(stateID.Height))

	requestIDMessage = append(requestIDMessage, heightByteArray...)

	return crypto.Sha256(requestIDMessage)
}
