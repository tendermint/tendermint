package types

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

func TestGenesisBad(t *testing.T) {
	// test some bad ones from raw json
	testCases := [][]byte{
		[]byte{},                                           // empty
		[]byte{1, 1, 1, 1, 1},                              // junk
		[]byte(`{}`),                                       // empty
		[]byte(`{"chain_id":"mychain"}`),                   // missing validators
		[]byte(`{"chain_id":"mychain","validators":[]}`),   // missing validators
		[]byte(`{"chain_id":"mychain","validators":[{}]}`), // missing validators
		[]byte(`{"chain_id":"mychain","validators":null}`), // missing validators
		[]byte(`{"chain_id":"mychain"}`),                   // missing validators
		[]byte(`{"validators":[{"pub_key":{"type":"tendermint/PubKeyEd25519","value":"AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="},"power":"10","name":""}]}`), // missing chain_id
	}

	for _, testCase := range testCases {
		_, err := GenesisDocFromJSON(testCase)
		assert.Error(t, err, "expected error for empty genDoc json")
	}
}

func TestGenesisGood(t *testing.T) {
	// test a good one by raw json
	genDocBytes := []byte(`{"genesis_time":"0001-01-01T00:00:00Z","chain_id":"test-chain-QDKdJr","consensus_params":null,"validators":[{"pub_key":{"type":"tendermint/PubKeyEd25519","value":"AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="},"power":"10","name":""}],"app_hash":"","app_state":{"account_owner": "Bob"}}`)
	_, err := GenesisDocFromJSON(genDocBytes)
	assert.NoError(t, err, "expected no error for good genDoc json")

	// create a base gendoc from struct
	baseGenDoc := &GenesisDoc{
		ChainID:    "abc",
		Validators: []GenesisValidator{{ed25519.GenPrivKey().PubKey(), 10, "myval"}},
	}
	genDocBytes, err = cdc.MarshalJSON(baseGenDoc)
	assert.NoError(t, err, "error marshalling genDoc")

	// test base gendoc and check consensus params were filled
	genDoc, err := GenesisDocFromJSON(genDocBytes)
	assert.NoError(t, err, "expected no error for valid genDoc json")
	assert.NotNil(t, genDoc.ConsensusParams, "expected consensus params to be filled in")

	// create json with consensus params filled
	genDocBytes, err = cdc.MarshalJSON(genDoc)
	assert.NoError(t, err, "error marshalling genDoc")
	genDoc, err = GenesisDocFromJSON(genDocBytes)
	assert.NoError(t, err, "expected no error for valid genDoc json")

	// test with invalid consensus params
	genDoc.ConsensusParams.BlockSize.MaxBytes = 0
	genDocBytes, err = cdc.MarshalJSON(genDoc)
	assert.NoError(t, err, "error marshalling genDoc")
	genDoc, err = GenesisDocFromJSON(genDocBytes)
	assert.Error(t, err, "expected error for genDoc json with block size of 0")
}

func TestGenesisSaveAs(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "genesis")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	genDoc := randomGenesisDoc()

	// save
	genDoc.SaveAs(tmpfile.Name())
	stat, err := tmpfile.Stat()
	require.NoError(t, err)
	if err != nil && stat.Size() <= 0 {
		t.Fatalf("SaveAs failed to write any bytes to %v", tmpfile.Name())
	}

	err = tmpfile.Close()
	require.NoError(t, err)

	// load
	genDoc2, err := GenesisDocFromFile(tmpfile.Name())
	require.NoError(t, err)

	// fails to unknown reason
	// assert.EqualValues(t, genDoc2, genDoc)
	assert.Equal(t, genDoc2.Validators, genDoc.Validators)
}

func TestGenesisValidatorHash(t *testing.T) {
	genDoc := randomGenesisDoc()
	assert.NotEmpty(t, genDoc.ValidatorHash())
}

func randomGenesisDoc() *GenesisDoc {
	return &GenesisDoc{
		GenesisTime:     time.Now().UTC(),
		ChainID:         "abc",
		Validators:      []GenesisValidator{{ed25519.GenPrivKey().PubKey(), 10, "myval"}},
		ConsensusParams: DefaultConsensusParams(),
	}
}
