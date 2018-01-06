package types

import (
	"fmt"

	crypto "github.com/tendermint/go-crypto"
	data "github.com/tendermint/go-wire/data"
	"github.com/tendermint/tendermint/types"
)

// PrivValidator aliases types.PrivValidator
type PrivValidator = types.PrivValidator

// PrivKey implements Signer
type PrivKey crypto.PrivKey

// Sign - Implements Signer
func (pk PrivKey) Sign(msg []byte) (crypto.Signature, error) {
	return crypto.PrivKey(pk).Sign(msg), nil
}

// MarshalJSON
func (pk PrivKey) MarshalJSON() ([]byte, error) {
	return crypto.PrivKey(pk).MarshalJSON()
}

// UnmarshalJSON
func (pk *PrivKey) UnmarshalJSON(b []byte) error {
	cpk := new(crypto.PrivKey)
	if err := cpk.UnmarshalJSON(b); err != nil {
		return err
	}
	*pk = (PrivKey)(*cpk)
	return nil
}

//-----------------------------------------------------------------

var _ types.PrivValidator = (*PrivValidatorUnencrypted)(nil)

// PrivValidatorUnencrypted implements PrivValidator.
// It uses an in-memory crypto.PrivKey that is
// persisted to disk unencrypted.
type PrivValidatorUnencrypted struct {
	ID             types.ValidatorID `json:"id"`
	PrivKey        PrivKey           `json:"priv_key"`
	LastSignedInfo *LastSignedInfo   `json:"last_signed_info"`
}

// NewPrivValidatorUnencrypted returns an instance of PrivValidatorUnencrypted.
func NewPrivValidatorUnencrypted(priv crypto.PrivKey) *PrivValidatorUnencrypted {
	return &PrivValidatorUnencrypted{
		ID: types.ValidatorID{
			Address: priv.PubKey().Address(),
			PubKey:  priv.PubKey(),
		},
		PrivKey:        PrivKey(priv),
		LastSignedInfo: NewLastSignedInfo(),
	}
}

// String returns a string representation of the PrivValidatorUnencrypted
func (upv *PrivValidatorUnencrypted) String() string {
	return fmt.Sprintf("PrivValidator{%v %v}", upv.Address(), upv.LastSignedInfo.String())
}

func (upv *PrivValidatorUnencrypted) Address() data.Bytes {
	return upv.PrivKey.PubKey().Address()
}

func (upv *PrivValidatorUnencrypted) PubKey() crypto.PubKey {
	return upv.PrivKey.PubKey()
}

func (upv *PrivValidatorUnencrypted) SignVote(chainID string, vote *types.Vote) error {
	return upv.LastSignedInfo.SignVote(upv.PrivKey, chainID, vote)
}

func (upv *PrivValidatorUnencrypted) SignProposal(chainID string, proposal *types.Proposal) error {
	return upv.LastSignedInfo.SignProposal(upv.PrivKey, chainID, proposal)
}

func (upv *PrivValidatorUnencrypted) SignHeartbeat(chainID string, heartbeat *types.Heartbeat) error {
	var err error
	heartbeat.Signature, err = upv.PrivKey.Sign(types.SignBytes(chainID, heartbeat))
	return err
}

//-----------------------------------------------------------------

var _ types.PrivValidator = (*SocketPrivValidator)(nil)

// SocketPrivValidator implements PrivValidator.
// It uses a socket to request signatures.
type SocketPrivValidator struct {
	ID            types.ValidatorID
	SocketAddress string
}

// NewSocketPrivValidator returns an instance of SocketPrivValidator.
func NewSocketPrivValidator(addr string) *SocketPrivValidator {
	// conn :=
	return &SocketPrivValidator{
		SocketAddress: addr,
	}
}

func (us *SocketPrivValidator) Address() data.Bytes {
	return nil
}

func (us *SocketPrivValidator) PubKey() crypto.PubKey {
	return crypto.PubKey{}
}

func (us *SocketPrivValidator) SignVote(chainID string, vote *types.Vote) error {
	return nil
}

func (us *SocketPrivValidator) SignProposal(chainID string, proposal *types.Proposal) error {
	return nil
}

func (us *SocketPrivValidator) SignHeartbeat(chainID string, heartbeat *types.Heartbeat) error {
	return nil
}
