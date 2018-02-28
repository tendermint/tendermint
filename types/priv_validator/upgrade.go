package types

import (
	"encoding/json"
	"io/ioutil"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

type PrivValidatorV1 struct {
	Address       cmn.HexBytes     `json:"address"`
	PubKey        crypto.PubKey    `json:"pub_key"`
	LastHeight    int64            `json:"last_height"`
	LastRound     int              `json:"last_round"`
	LastStep      int8             `json:"last_step"`
	LastSignature crypto.Signature `json:"last_signature,omitempty"` // so we dont lose signatures
	LastSignBytes cmn.HexBytes     `json:"last_signbytes,omitempty"` // so we dont lose signatures
	PrivKey       crypto.PrivKey   `json:"priv_key"`
}

func UpgradePrivValidator(filePath string) (*PrivValidatorJSON, error) {
	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	pv := new(PrivValidatorV1)
	err = json.Unmarshal(b, pv)
	if err != nil {
		return nil, err
	}

	pvNew := &PrivValidatorJSON{
		PrivValidatorUnencrypted: &PrivValidatorUnencrypted{
			ID: types.ValidatorID{
				Address: pv.Address,
				PubKey:  pv.PubKey,
			},
			PrivKey: PrivKey(pv.PrivKey),
			LastSignedInfo: &LastSignedInfo{
				Height:    pv.LastHeight,
				Round:     pv.LastRound,
				Step:      pv.LastStep,
				SignBytes: pv.LastSignBytes,
				Signature: pv.LastSignature,
			},
		},
	}

	b, err = json.MarshalIndent(pvNew, "", "  ")
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile(filePath, b, 0600)
	return pvNew, err
}
