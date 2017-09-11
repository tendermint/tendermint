package types

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
)

//------------------------------------------------------------
// core types for a genesis definition

// GenesisValidator is an initial validator.
type GenesisValidator struct {
	PubKey crypto.PubKey `json:"pub_key"`
	Amount int64         `json:"amount"`
	Name   string        `json:"name"`
}

// GenesisDoc defines the initial conditions for a tendermint blockchain, in particular its validator set.
type GenesisDoc struct {
	GenesisTime time.Time          `json:"genesis_time"`
	ChainID     string             `json:"chain_id"`
	Validators  []GenesisValidator `json:"validators"`
	AppHash     data.Bytes         `json:"app_hash"`
}

// SaveAs is a utility method for saving GenensisDoc as a JSON file.
func (genDoc *GenesisDoc) SaveAs(file string) error {
	genDocBytes, err := json.Marshal(genDoc)
	if err != nil {
		return err
	}
	return cmn.WriteFile(file, genDocBytes, 0644)
}

// ValidatorHash returns the hash of the validator set contained in the GenesisDoc
func (genDoc *GenesisDoc) ValidatorHash() []byte {
	vals := make([]*Validator, len(genDoc.Validators))
	for i, v := range genDoc.Validators {
		vals[i] = NewValidator(v.PubKey, v.Amount)
	}
	vset := NewValidatorSet(vals)
	return vset.Hash()
}

//------------------------------------------------------------
// Make genesis state from file

// GenesisDocFromJSON unmarshalls JSON data into a GenesisDoc.
func GenesisDocFromJSON(jsonBlob []byte) (*GenesisDoc, error) {
	genDoc := GenesisDoc{}
	err := json.Unmarshal(jsonBlob, &genDoc)
	return &genDoc, err
}

// GenesisDocFromFile reads JSON data from a file and unmarshalls it into a GenesisDoc.
func GenesisDocFromFile(genDocFile string) (*GenesisDoc, error) {
	jsonBlob, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		return nil, errors.Wrap(err, "Couldn't read GenesisDoc file")
	}
	genDoc, err := GenesisDocFromJSON(jsonBlob)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading GenesisDoc")
	}
	if genDoc.ChainID == "" {
		return nil, errors.Errorf("Genesis doc %v must include non-empty chain_id", genDocFile)
	}
	return genDoc, nil
}
