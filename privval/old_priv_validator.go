package privval

import (
	"io/ioutil"
	"os"

	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/types"
)

// OldFilePV is the old version of the FilePV, pre v0.28.0.
type OldFilePV struct {
	Address       types.Address  `json:"address"`
	PubKey        crypto.PubKey  `json:"pub_key"`
	LastHeight    int64          `json:"last_height"`
	LastRound     int            `json:"last_round"`
	LastStep      int8           `json:"last_step"`
	LastSignature []byte         `json:"last_signature,omitempty"`
	LastSignBytes cmn.HexBytes   `json:"last_signbytes,omitempty"`
	PrivKey       crypto.PrivKey `json:"priv_key"`

	filePath string
}

// LoadOldFilePV loads an OldFilePV from the filePath.
func LoadOldFilePV(filePath string) (*OldFilePV, error) {
	pvJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	pv := &OldFilePV{}
	err = cdc.UnmarshalJSON(pvJSONBytes, &pv)
	if err != nil {
		return nil, err
	}

	// overwrite pubkey and address for convenience
	pv.PubKey = pv.PrivKey.PubKey()
	pv.Address = pv.PubKey.Address()

	pv.filePath = filePath
	return pv, nil
}

// Upgrade convets the OldFilePV to the new FilePV, separating the immutable and mutable components,
// and persisting them to the keyFilePath and stateFilePath, respectively.
// It renames the original file by adding ".bak".
func (oldFilePV *OldFilePV) Upgrade(keyFilePath, stateFilePath string) *FilePV {
	privKey := oldFilePV.PrivKey
	pvKey := FilePVKey{
		PrivKey:  privKey,
		PubKey:   privKey.PubKey(),
		Address:  privKey.PubKey().Address(),
		filePath: keyFilePath,
	}

	pvState := FilePVLastSignState{
		Height:    oldFilePV.LastHeight,
		Round:     oldFilePV.LastRound,
		Step:      oldFilePV.LastStep,
		Signature: oldFilePV.LastSignature,
		SignBytes: oldFilePV.LastSignBytes,
		filePath:  stateFilePath,
	}

	// Save the new PV files
	pv := &FilePV{
		Key:           pvKey,
		LastSignState: pvState,
	}
	pv.Save()

	// Rename the old PV file
	err := os.Rename(oldFilePV.filePath, oldFilePV.filePath+".bak")
	if err != nil {
		panic(err)
	}
	return pv
}
