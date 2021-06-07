package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"io/ioutil"
	"time"

	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
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
	Address   Address          `json:"address"`
	PubKey    crypto.PubKey    `json:"pub_key"`
	Power     int64            `json:"power"`
	Name      string           `json:"name"`
	ProTxHash crypto.ProTxHash `json:"pro_tx_hash"`
}

// GenesisDoc defines the initial conditions for a tendermint blockchain, in particular its validator set.
type GenesisDoc struct {
	GenesisTime                  time.Time                `json:"genesis_time"`
	ChainID                      string                   `json:"chain_id"`
	InitialHeight                int64                    `json:"initial_height"`
	InitialCoreChainLockedHeight uint32                   `json:"initial_core_chain_locked_height"`
	InitialProposalCoreChainLock *tmproto.CoreChainLock   `json:"initial_proposal_core_chain_lock"`
	ConsensusParams              *tmproto.ConsensusParams `json:"consensus_params,omitempty"`
	Validators                   []GenesisValidator       `json:"validators,omitempty"`
	ThresholdPublicKey           crypto.PubKey            `json:"threshold_public_key"`
	QuorumType                   btcjson.LLMQType         `json:"quorum_type"`
	QuorumHash                   crypto.QuorumHash        `json:"quorum_hash"`
	NodeProTxHash                *crypto.ProTxHash        `json:"node_pro_tx_hash"`
	AppHash                      tmbytes.HexBytes         `json:"app_hash"`
	AppState                     json.RawMessage          `json:"app_state,omitempty"`
}

// SaveAs is a utility method for saving GenensisDoc as a JSON file.
func (genDoc *GenesisDoc) SaveAs(file string) error {
	genDocBytes, err := tmjson.MarshalIndent(genDoc, "", "  ")
	if err != nil {
		return err
	}
	return tmos.WriteFile(file, genDocBytes, 0644)
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
	} else if err := ValidateConsensusParams(*genDoc.ConsensusParams); err != nil {
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
		if len(v.ProTxHash) != crypto.ProTxHashSize {
			return fmt.Errorf("validators must all contain a pro_tx_hash of size 32")
		}
	}

	if genDoc.Validators != nil && genDoc.ThresholdPublicKey == nil {
		return fmt.Errorf("the threshold public key must be set if there are validators (%d Validator(s))",
			len(genDoc.Validators))
	}
	if genDoc.Validators != nil && len(genDoc.ThresholdPublicKey.Bytes()) != bls12381.PubKeySize {
		return fmt.Errorf("the threshold public key must be 48 bytes for BLS")
	}
	if genDoc.Validators != nil && len(genDoc.QuorumHash.Bytes()) < crypto.SmallAppHashSize {
		return fmt.Errorf("the quorum hash must be at least 20 bytes long (%d Validator(s))", len(genDoc.Validators))
	}

	if genDoc.Validators != nil && genDoc.QuorumType == 0 {
		return fmt.Errorf("the quorum type must not be 0 (%d Validator(s))", len(genDoc.Validators))
	}

	if genDoc.NodeProTxHash != nil && len(*genDoc.NodeProTxHash) != crypto.DefaultHashSize {
		return fmt.Errorf("the node proTxHash must be 32 bytes if it is set (received %d)", len(*genDoc.NodeProTxHash))
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
	err := tmjson.Unmarshal(jsonBlob, &genDoc)
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
		return nil, fmt.Errorf("couldn't read GenesisDoc file: %w", err)
	}
	genDoc, err := GenesisDocFromJSON(jsonBlob)
	if err != nil {
		fmt.Printf("gendoc %v\n", genDoc)
		return nil, fmt.Errorf("error reading GenesisDoc at %s: %w", genDocFile, err)
	}
	return genDoc, nil
}
