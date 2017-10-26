package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	crypto "github.com/tendermint/go-crypto"
)

func TestGenesis(t *testing.T) {

	// test some bad ones from raw json
	testCases := [][]byte{
		[]byte{},                                             // empty
		[]byte{1, 1, 1, 1, 1},                                // junk
		[]byte(`{}`),                                         // empty
		[]byte(`{"chain_id": "mychain"}`),                    // missing validators
		[]byte(`{"chain_id": "mychain", "validators": []`),   // missing validators
		[]byte(`{"chain_id": "mychain", "validators": [{}]`), // missing validators
		[]byte(`{"validators":[{"pub_key":
		{"type":"ed25519","data":"961EAB8752E51A03618502F55C2B6E09C38C65635C64CCF3173ED452CF86C957"},
		"power":10,"name":""}]}`), // missing chain_id
	}

	for _, testCase := range testCases {
		_, err := GenesisDocFromJSON(testCase)
		assert.Error(t, err, "expected error for empty genDoc json")
	}

	// test a good one by raw json
	genDocBytes := []byte(`{"genesis_time":"0001-01-01T00:00:00Z","chain_id":"test-chain-QDKdJr","consensus_params":null,"validators":[{"pub_key":{"type":"ed25519","data":"961EAB8752E51A03618502F55C2B6E09C38C65635C64CCF3173ED452CF86C957"},"power":10,"name":""}],"app_hash":"","app_options":{"account_owner": "Bob"}}`)
	_, err := GenesisDocFromJSON(genDocBytes)
	assert.NoError(t, err, "expected no error for good genDoc json")

	// create a base gendoc from struct
	baseGenDoc := &GenesisDoc{
		ChainID:    "abc",
		Validators: []GenesisValidator{{crypto.GenPrivKeyEd25519().PubKey(), 10, "myval"}},
	}
	genDocBytes, err = json.Marshal(baseGenDoc)
	assert.NoError(t, err, "error marshalling genDoc")

	// test base gendoc and check consensus params were filled
	genDoc, err := GenesisDocFromJSON(genDocBytes)
	assert.NoError(t, err, "expected no error for valid genDoc json")
	assert.NotNil(t, genDoc.ConsensusParams, "expected consensus params to be filled in")

	// create json with consensus params filled
	genDocBytes, err = json.Marshal(genDoc)
	assert.NoError(t, err, "error marshalling genDoc")
	genDoc, err = GenesisDocFromJSON(genDocBytes)
	assert.NoError(t, err, "expected no error for valid genDoc json")

	// test with invalid consensus params
	genDoc.ConsensusParams.BlockSizeParams.MaxBytes = 0
	genDocBytes, err = json.Marshal(genDoc)
	assert.NoError(t, err, "error marshalling genDoc")
	genDoc, err = GenesisDocFromJSON(genDocBytes)
	assert.Error(t, err, "expected error for genDoc json with block size of 0")
}
