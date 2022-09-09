package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// This file contains implementation of StateID logic.

//--------------------------------------------------------------------------------

// StateID
// TODO: Move to separate file
type StateID struct {
	// Height of current block (the one containing state ID signature)
	Height int64 `json:"height"`
	// AppHash used in current block (the one containing state ID signature)
	AppHash tmbytes.HexBytes `json:"last_app_hash"`
}

// Copy returns new StateID that is equal to this one
func (stateID StateID) Copy() StateID {
	appHash := make([]byte, len(stateID.AppHash))
	if copy(appHash, stateID.AppHash) != len(stateID.AppHash) {
		panic("Cannot copy LastAppHash, this should never happen. Out of memory???")
	}

	return StateID{
		Height:  stateID.Height,
		AppHash: appHash,
	}
}

// Equals returns true if the StateID matches the given StateID
func (stateID StateID) Equals(other StateID) bool {
	return stateID.Height == other.Height &&
		bytes.Equal(stateID.AppHash, other.AppHash)
}

// Key returns a machine-readable string representation of the StateID
func (stateID StateID) Key() string {
	return strconv.FormatInt(stateID.Height, 36) + string(stateID.AppHash)
}

// ValidateBasic performs basic validation.
func (stateID StateID) ValidateBasic() error {
	// LastAppHash can be empty in case of genesis block.
	if err := ValidateAppHash(stateID.AppHash); err != nil {
		return fmt.Errorf("wrong app Hash")
	}

	if stateID.Height < 0 {
		return fmt.Errorf("stateID height is not valid: %d < 0", stateID.Height)
	}

	return nil
}

// SignBytes returns bytes that should be signed
// TODO why we don't simply use stateID.Marshal() ?
func (stateID StateID) SignBytes(chainID string) []byte {
	bz := make([]byte, 8)
	binary.LittleEndian.PutUint64(bz, uint64(stateID.Height))

	appHash := stateID.AppHash
	if len(appHash) == 0 {
		appHash = make([]byte, crypto.DefaultAppHashSize)
	}

	bz = append(bz, appHash...)
	return bz
}

// SignID returns signing session data that will be signed to get threshold signature share.
// See DIP-0007
func (stateID StateID) SignID(chainID string, quorumType btcjson.LLMQType, quorumHash []byte) []byte {

	stateSignBytes := stateID.SignBytes(chainID)

	if stateSignBytes == nil {
		return nil
	}

	stateMessageHash := sha256.Sum256(stateSignBytes)

	stateRequestID := stateID.SignRequestID()

	stateSignID := crypto.SignID(
		quorumType,
		tmbytes.Reverse(quorumHash),
		tmbytes.Reverse(stateRequestID),
		tmbytes.Reverse(stateMessageHash[:]),
	)

	return stateSignID

}

func (stateID StateID) SignRequestID() []byte {
	requestIDMessage := []byte("dpsvote")
	heightByteArray := make([]byte, 8)

	binary.LittleEndian.PutUint64(heightByteArray, uint64(stateID.Height))

	requestIDMessage = append(requestIDMessage, heightByteArray...)

	hash := sha256.Sum256(requestIDMessage)
	return hash[:]
}

// String returns a human readable string representation of the StateID.
//
// 1. hash
//
func (stateID StateID) String() string {
	return fmt.Sprintf(`%d:%v`, stateID.Height, stateID.AppHash)
}

// ToProto converts StateID to protobuf
func (stateID StateID) ToProto() tmproto.StateID {
	return tmproto.StateID{
		AppHash: stateID.AppHash,
		Height:  stateID.Height,
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

// FromProto sets a protobuf BlockID to the given pointer.
// It returns an error if the block id is invalid.
func StateIDFromProto(sID *tmproto.StateID) (*StateID, error) {
	if sID == nil {
		return nil, errors.New("nil StateID")
	}

	stateID := new(StateID)

	stateID.AppHash = sID.AppHash
	stateID.Height = sID.Height

	return stateID, stateID.ValidateBasic()
}
