package types

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/pkg/errors"

	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
)

//------------------------------------------------------------
// core types for a genesis definition

// GenesisValidator is an initial validator.
type GenesisValidator struct {
	PubKey crypto.PubKey `json:"pub_key"`
	Power  int64         `json:"power"`
	Name   string        `json:"name"`
}

// GenesisDoc defines the initial conditions for a tendermint blockchain, in particular its validator set.
type GenesisDoc struct {
	GenesisTime     time.Time          `json:"genesis_time"`
	ChainID         string             `json:"chain_id"`
	ConsensusParams *ConsensusParams   `json:"consensus_params,omitempty"`
	Validators      []GenesisValidator `json:"validators"`
	AppHash         cmn.HexBytes       `json:"app_hash"`
	AppStateJSON    json.RawMessage    `json:"app_state,omitempty"`
	AppOptions      json.RawMessage    `json:"app_options,omitempty"` // DEPRECATED
}

// AppState returns raw application state.
// TODO: replace with AppState field during next breaking release (0.18)
func (genDoc *GenesisDoc) AppState() json.RawMessage {
	if len(genDoc.AppOptions) > 0 {
		return genDoc.AppOptions
	}
	return genDoc.AppStateJSON
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
		vals[i] = NewValidator(v.PubKey, v.Power)
	}
	vset := NewValidatorSet(vals)
	return vset.Hash()
}

// ValidateAndComplete checks that all necessary fields are present
// and fills in defaults for optional fields left empty
func (genDoc *GenesisDoc) ValidateAndComplete() error {

	if genDoc.ChainID == "" {
		return errors.Errorf("Genesis doc must include non-empty chain_id")
	}

	if genDoc.ConsensusParams == nil {
		genDoc.ConsensusParams = DefaultConsensusParams()
	} else {
		if err := genDoc.ConsensusParams.Validate(); err != nil {
			return err
		}
	}

	if len(genDoc.Validators) == 0 {
		return errors.Errorf("The genesis file must have at least one validator")
	}

	for _, v := range genDoc.Validators {
		if v.Power == 0 {
			return errors.Errorf("The genesis file cannot contain validators with no voting power: %v", v)
		}
	}

	if genDoc.GenesisTime.IsZero() {
		genDoc.GenesisTime = time.Now()
	}

	return nil
}

//------------------------------------------------------------
// Make genesis state from file

// GenesisDocFromJSON unmarshalls JSON data into a GenesisDoc.
func GenesisDocFromJSON(jsonBlob []byte) (*GenesisDoc, error) {
	genDoc := GenesisDoc{}
	err := json.Unmarshal(jsonBlob, &genDoc)
	if err != nil {
		return nil, err
	}

	if err := genDoc.ValidateAndComplete(); err != nil {
		return nil, err
	}

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
		return nil, errors.Wrap(err, cmn.Fmt("Error reading GenesisDoc at %v", genDocFile))
	}
	return genDoc, nil
}
