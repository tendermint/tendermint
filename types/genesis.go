package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/jsontypes"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmtime "github.com/tendermint/tendermint/libs/time"
)

const (
	// MaxChainIDLen is a maximum length of the chain ID.
	MaxChainIDLen = 50
)

//------------------------------------------------------------
// core types for a genesis definition
// NOTE: any changes to the genesis definition should
// be reflected in the documentation:
// docs/tendermint-core/using-tendermint.md

// GenesisValidator is an initial validator.
type GenesisValidator struct {
	Address Address
	PubKey  crypto.PubKey
	Power   int64
	Name    string
}

type genesisValidatorJSON struct {
	Address Address         `json:"address"`
	PubKey  json.RawMessage `json:"pub_key"`
	Power   int64           `json:"power,string"`
	Name    string          `json:"name"`
}

func (g GenesisValidator) MarshalJSON() ([]byte, error) {
	pk, err := jsontypes.Marshal(g.PubKey)
	if err != nil {
		return nil, err
	}
	return json.Marshal(genesisValidatorJSON{
		Address: g.Address, PubKey: pk, Power: g.Power, Name: g.Name,
	})
}

func (g *GenesisValidator) UnmarshalJSON(data []byte) error {
	var gv genesisValidatorJSON
	if err := json.Unmarshal(data, &gv); err != nil {
		return err
	}
	if err := jsontypes.Unmarshal(gv.PubKey, &g.PubKey); err != nil {
		return err
	}
	g.Address = gv.Address
	g.Power = gv.Power
	g.Name = gv.Name
	return nil
}

// GenesisDoc defines the initial conditions for a tendermint blockchain, in particular its validator set.
type GenesisDoc struct {
	GenesisTime     time.Time          `json:"genesis_time"`
	ChainID         string             `json:"chain_id"`
	InitialHeight   int64              `json:"initial_height,string"`
	ConsensusParams *ConsensusParams   `json:"consensus_params,omitempty"`
	Validators      []GenesisValidator `json:"validators,omitempty"`
	AppHash         tmbytes.HexBytes   `json:"app_hash"`
	AppState        json.RawMessage    `json:"app_state,omitempty"`
}

// SaveAs is a utility method for saving GenensisDoc as a JSON file.
func (genDoc *GenesisDoc) SaveAs(file string) error {
	genDocBytes, err := json.MarshalIndent(genDoc, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(file, genDocBytes, 0644) // nolint:gosec
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
		return errors.New("genesis doc must include non-empty chain_id")
	}
	if len(genDoc.ChainID) > MaxChainIDLen {
		return fmt.Errorf("chain_id in genesis doc is too long (max: %d)", MaxChainIDLen)
	}
	if genDoc.InitialHeight < 0 {
		return fmt.Errorf("initial_height cannot be negative (got %v)", genDoc.InitialHeight)
	}
	if genDoc.InitialHeight == 0 {
		genDoc.InitialHeight = 1
	}

	if genDoc.ConsensusParams == nil {
		genDoc.ConsensusParams = DefaultConsensusParams()
	}
	genDoc.ConsensusParams.Complete()

	if err := genDoc.ConsensusParams.ValidateConsensusParams(); err != nil {
		return err
	}

	for i, v := range genDoc.Validators {
		if v.Power == 0 {
			return fmt.Errorf("the genesis file cannot contain validators with no voting power: %v", v)
		}
		if len(v.Address) > 0 && !bytes.Equal(v.PubKey.Address(), v.Address) {
			return fmt.Errorf("incorrect address for validator %v in the genesis file, should be %v", v, v.PubKey.Address())
		}
		if len(v.Address) == 0 {
			genDoc.Validators[i].Address = v.PubKey.Address()
		}
	}

	if genDoc.GenesisTime.IsZero() {
		genDoc.GenesisTime = tmtime.Now()
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
	jsonBlob, err := os.ReadFile(genDocFile)
	if err != nil {
		return nil, fmt.Errorf("couldn't read GenesisDoc file: %w", err)
	}
	genDoc, err := GenesisDocFromJSON(jsonBlob)
	if err != nil {
		return nil, fmt.Errorf("error reading GenesisDoc at %s: %w", genDocFile, err)
	}
	return genDoc, nil
}
