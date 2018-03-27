package types

import (
	"fmt"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

//-----------------------------------------------------------------

var _ types.PrivValidator2 = (*PrivValidatorUnencrypted)(nil)

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
	addr, err := upv.Address()
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf("PrivValidator{%v %v}", addr, upv.LastSignedInfo.String())
}

func (upv *PrivValidatorUnencrypted) Address() (cmn.HexBytes, error) {
	return upv.PrivKey.PubKey().Address(), nil
}

func (upv *PrivValidatorUnencrypted) PubKey() (crypto.PubKey, error) {
	return upv.PrivKey.PubKey(), nil
}

func (upv *PrivValidatorUnencrypted) SignVote(chainID string, vote *types.Vote) error {
	return upv.LastSignedInfo.SignVote(upv.PrivKey, chainID, vote)
}

func (upv *PrivValidatorUnencrypted) SignProposal(chainID string, proposal *types.Proposal) error {
	return upv.LastSignedInfo.SignProposal(upv.PrivKey, chainID, proposal)
}

func (upv *PrivValidatorUnencrypted) SignHeartbeat(chainID string, heartbeat *types.Heartbeat) error {
	var err error
	heartbeat.Signature, err = upv.PrivKey.Sign(heartbeat.SignBytes(chainID))
	return err
}
