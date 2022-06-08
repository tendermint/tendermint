package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/internal/jsontypes"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
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
	PubKey    crypto.PubKey
	Power     int64
	Name      string
	ProTxHash crypto.ProTxHash
}

type genesisValidatorJSON struct {
	PubKey    json.RawMessage  `json:"pub_key"`
	Power     int64            `json:"power"`
	Name      string           `json:"name"`
	ProTxHash crypto.ProTxHash `json:"pro_tx_hash"`
}

func (g GenesisValidator) MarshalJSON() ([]byte, error) {
	pk, err := jsontypes.Marshal(g.PubKey)
	if err != nil {
		return nil, err
	}
	return json.Marshal(genesisValidatorJSON{
		ProTxHash: g.ProTxHash, PubKey: pk, Power: g.Power, Name: g.Name,
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
	g.Power = gv.Power
	g.Name = gv.Name
	g.ProTxHash = gv.ProTxHash
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

	// dash fields
	InitialCoreChainLockedHeight uint32                 `json:"initial_core_chain_locked_height"`
	InitialProposalCoreChainLock *tmproto.CoreChainLock `json:"initial_proposal_core_chain_lock"`
	ThresholdPublicKey           crypto.PubKey          `json:"threshold_public_key"`
	QuorumType                   btcjson.LLMQType       `json:"quorum_type"`
	QuorumHash                   crypto.QuorumHash      `json:"quorum_hash"`
}

type genesisDocJSON struct {
	GenesisTime     time.Time          `json:"genesis_time"`
	ChainID         string             `json:"chain_id"`
	InitialHeight   int64              `json:"initial_height,string"`
	ConsensusParams *ConsensusParams   `json:"consensus_params,omitempty"`
	Validators      []GenesisValidator `json:"validators,omitempty"`
	AppHash         tmbytes.HexBytes   `json:"app_hash"`
	AppState        json.RawMessage    `json:"app_state,omitempty"`

	// dash fields
	InitialCoreChainLockedHeight uint32                 `json:"initial_core_chain_locked_height"`
	InitialProposalCoreChainLock *tmproto.CoreChainLock `json:"initial_proposal_core_chain_lock"`
	ThresholdPublicKey           json.RawMessage        `json:"threshold_public_key"`
	QuorumType                   btcjson.LLMQType       `json:"quorum_type"`
	QuorumHash                   crypto.QuorumHash      `json:"quorum_hash"`
}

func (genDoc GenesisDoc) MarshalJSON() ([]byte, error) {
	tpk, err := jsontypes.Marshal(genDoc.ThresholdPublicKey)
	if err != nil {
		return nil, err
	}
	return json.Marshal(genesisDocJSON{
		GenesisTime:     genDoc.GenesisTime,
		ChainID:         genDoc.ChainID,
		InitialHeight:   genDoc.InitialHeight,
		ConsensusParams: genDoc.ConsensusParams,
		Validators:      genDoc.Validators,
		AppHash:         genDoc.AppHash,
		AppState:        genDoc.AppState,

		InitialCoreChainLockedHeight: genDoc.InitialCoreChainLockedHeight,
		InitialProposalCoreChainLock: genDoc.InitialProposalCoreChainLock,
		ThresholdPublicKey:           tpk,
		QuorumType:                   genDoc.QuorumType,
		QuorumHash:                   genDoc.QuorumHash,
	})
}

func (genDoc *GenesisDoc) UnmarshalJSON(data []byte) error {
	var g genesisDocJSON
	if err := json.Unmarshal(data, &g); err != nil {
		return err
	}
	if err := jsontypes.Unmarshal(g.ThresholdPublicKey, &genDoc.ThresholdPublicKey); err != nil {
		return err
	}
	genDoc.GenesisTime = g.GenesisTime
	genDoc.ChainID = g.ChainID
	genDoc.InitialHeight = g.InitialHeight
	genDoc.ConsensusParams = g.ConsensusParams
	genDoc.Validators = g.Validators
	genDoc.AppHash = g.AppHash
	genDoc.AppState = g.AppState
	genDoc.InitialCoreChainLockedHeight = g.InitialCoreChainLockedHeight
	genDoc.InitialProposalCoreChainLock = g.InitialProposalCoreChainLock
	genDoc.QuorumType = g.QuorumType
	genDoc.QuorumHash = g.QuorumHash
	return nil
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
	if genDoc.QuorumHash == nil {
		panic("quorum hash should not be nil")
	}
	return genDoc.QuorumHash
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

	if genDoc.QuorumType == 0 {
		genDoc.QuorumType = 100
	}

	if genDoc.InitialProposalCoreChainLock != nil &&
		genDoc.InitialProposalCoreChainLock.CoreBlockHeight <= genDoc.InitialCoreChainLockedHeight {
		return fmt.Errorf("if set the initial proposal core chain locked block height %d"+
			" must be superior to the initial core chain locked height %d",
			genDoc.InitialProposalCoreChainLock.CoreBlockHeight, genDoc.InitialCoreChainLockedHeight)
	}

	if genDoc.ConsensusParams == nil {
		genDoc.ConsensusParams = DefaultConsensusParams()
	}
	genDoc.ConsensusParams.Complete()

	if err := genDoc.ConsensusParams.ValidateConsensusParams(); err != nil {
		return err
	}

	lenVals := len(genDoc.Validators)

	for _, v := range genDoc.Validators {
		if v.Power == 0 {
			return fmt.Errorf("the genesis file cannot contain validators with no voting power: %v", v)
		}
		if len(v.ProTxHash) != crypto.ProTxHashSize {
			return fmt.Errorf("validators must all contain a pro_tx_hash of size 32")
		}
	}

	if lenVals > 0 && genDoc.ThresholdPublicKey == nil {
		return fmt.Errorf("the threshold public key must be set if there are validators (%d Validator(s))",
			len(genDoc.Validators))
	}
	if lenVals > 0 && len(genDoc.ThresholdPublicKey.Bytes()) != bls12381.PubKeySize {
		return fmt.Errorf("the threshold public key must be 48 bytes for BLS")
	}
	if lenVals > 0 && len(genDoc.QuorumHash.Bytes()) < crypto.SmallAppHashSize {
		return fmt.Errorf("the quorum hash must be at least %d bytes long (%d Validator(s))",
			crypto.SmallAppHashSize,
			len(genDoc.Validators))
	}

	if genDoc.QuorumType == 0 {
		return fmt.Errorf("the quorum type must not be 0 (%d Validator(s))", len(genDoc.Validators))
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
		fmt.Printf("gendoc %v\n", genDoc)
		return nil, fmt.Errorf("error reading GenesisDoc at %s: %w", genDocFile, err)
	}
	return genDoc, nil
}
