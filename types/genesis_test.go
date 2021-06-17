//nolint:lll
package types

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmjson "github.com/tendermint/tendermint/libs/json"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func TestGenesisBad(t *testing.T) {
	// test some bad ones from raw json
	testCases := [][]byte{
		{},              // empty
		{1, 1, 1, 1, 1}, // junk
		[]byte(`{}`),    // empty
		[]byte(`{"chain_id":"mychain","validators":[{}]}`),   // invalid validator
		[]byte(`{"chain_id":"chain","initial_height":"-1"}`), // negative initial height
		// missing pub_key type
		[]byte(
			`{"threshold_public_key": {"type": "tendermint/PubKeyBLS12381","value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"},"validators":[{"pub_key":{"value":"AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="},"power":"10","name":"","pro_tx_hash":"51BF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C45F"}]}`,
		),
		// missing threshold_public_key
		[]byte(
			`{"validators":[{"pub_key":{"value":"AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="},"power":"10","name":"","pro_tx_hash":"51BF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C45F"}]}`,
		),
		// missing threshold_public_key key type
		[]byte(
			`{"threshold_public_key": {"value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"},"validators":[{"pub_key":{"value":"AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="},"power":"10","name":"","pro_tx_hash":"51BF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C45F"}]}`,
		),
		// missing chain_id
		[]byte(
			`{"validators":[` +
				`{"pub_key":{` +
				`"type": "tendermint/PubKeyBLS12381","value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"` +
				`},"power":"10","name":"","pro_tx_hash":"51BF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C45F"}` +
				`]}`,
		),
		// too big chain_id
		[]byte(
			`{"chain_id": "Lorem ipsum dolor sit amet, consectetuer adipiscing", "validators": [` +
				`{"pub_key":{` +
				`"type": "tendermint/PubKeyBLS12381","value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"` +
				`},"power":"10","name":"","pro_tx_hash":"51BF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C45F"}` +
				`]}`,
		),
		// wrong address
		[]byte(
			`{"chain_id":"mychain", "validators":[` +
				`{"address": "A", "pub_key":{` +
				`"type": "tendermint/PubKeyBLS12381","value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"` +
				`},"power":"10","name":"","pro_tx_hash":"51BF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C45F"}` +
				`]}`,
		),
		// missing pro_tx_hash
		[]byte(
			`{"chain_id":"mychain", "validators":[` +
				`{"address": "A", "pub_key":{` +
				`"type":"tendermint/PubKeyEd25519","value":"AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="` +
				`},"power":"10","name":""}` +
				`]}`,
		),
		// missing quorum_hash
		[]byte(
			`{
			"genesis_time": "0001-01-01T00:00:00Z",
			"chain_id": "test-chain-QDKdJr",
			"initial_height": "1000",
            "initial_core_chain_locked_height": 3000,
			"consensus_params": null,
			"validators": [{
				"pub_key":{"type": "tendermint/PubKeyBLS12381","value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"},
				"power":"100",
				"name":"",
				"pro_tx_hash":"51BF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C45F"
			}],
			"threshold_public_key": {
				"type": "tendermint/PubKeyBLS12381",
				"value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"
			}
		}`),
	}

	for _, testCase := range testCases {
		_, err := GenesisDocFromJSON(testCase)
		assert.Error(t, err, "expected error for empty genDoc json")
	}
}

func TestGenesisGood(t *testing.T) {
	// test a good one by raw json
	genDocBytes := []byte(
		`{
			"genesis_time": "0001-01-01T00:00:00Z",
			"chain_id": "test-chain-QDKdJr",
			"initial_height": "1000",
            "initial_core_chain_locked_height": 3000,
			"consensus_params": null,
			"validators": [{
				"pub_key":{"type": "tendermint/PubKeyBLS12381","value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"},
				"power":"100",
				"name":"",
				"pro_tx_hash":"51BF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C45F"
			}],
			"threshold_public_key": {
				"type": "tendermint/PubKeyBLS12381",
				"value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"
			},
			"quorum_hash":"43FF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C4CC",
			"app_hash":"",
			"app_state":{"account_owner": "Bob"}
		}`,
	)
	_, err := GenesisDocFromJSON(genDocBytes)
	assert.NoError(t, err, "expected no error for good genDoc json")

	pubkey := bls12381.GenPrivKey().PubKey()
	// create a base gendoc from struct
	baseGenDoc := &GenesisDoc{
		ChainID:            "abc",
		Validators:         []GenesisValidator{{pubkey.Address(), pubkey, 100, "myval", crypto.RandProTxHash()}},
		ThresholdPublicKey: pubkey,
		QuorumHash:         crypto.RandQuorumHash(),
	}
	genDocBytes, err = tmjson.Marshal(baseGenDoc)
	assert.NoError(t, err, "error marshalling genDoc")

	// test base gendoc and check consensus params were filled
	genDoc, err := GenesisDocFromJSON(genDocBytes)
	assert.NoError(t, err, "expected no error for valid genDoc json")
	assert.NotNil(t, genDoc.ConsensusParams, "expected consensus params to be filled in")

	// check validator's address is filled
	assert.NotNil(t, genDoc.Validators[0].Address, "expected validator's address to be filled in")

	// create json with consensus params filled
	genDocBytes, err = tmjson.Marshal(genDoc)
	assert.NoError(t, err, "error marshalling genDoc")
	genDoc, err = GenesisDocFromJSON(genDocBytes)
	assert.NoError(t, err, "expected no error for valid genDoc json")

	// test with invalid consensus params
	genDoc.ConsensusParams.Block.MaxBytes = 0
	genDocBytes, err = tmjson.Marshal(genDoc)
	assert.NoError(t, err, "error marshalling genDoc")
	_, err = GenesisDocFromJSON(genDocBytes)
	assert.Error(t, err, "expected error for genDoc json with block size of 0")

	// Genesis doc from raw json
	missingValidatorsTestCases := [][]byte{
		[]byte(`{"chain_id":"mychain"}`),                   // missing validators
		[]byte(`{"chain_id":"mychain","validators":[]}`),   // missing validators
		[]byte(`{"chain_id":"mychain","validators":null}`), // nil validator
		[]byte(`{"chain_id":"mychain"}`),                   // missing validators
	}

	for _, tc := range missingValidatorsTestCases {
		_, err := GenesisDocFromJSON(tc)
		assert.NoError(t, err)
	}
}

func TestGenesisSaveAs(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "genesis")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	genDoc := randomGenesisDoc()

	// save
	err = genDoc.SaveAs(tmpfile.Name())
	require.NoError(t, err)
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
	assert.EqualValues(t, genDoc2, genDoc)
	assert.Equal(t, genDoc2.Validators, genDoc.Validators)
}

func TestGenesisValidatorHash(t *testing.T) {
	genDoc := randomGenesisDoc()
	assert.NotEmpty(t, genDoc.ValidatorHash())
}

func randomGenesisDoc() *GenesisDoc {
	pubkey := bls12381.GenPrivKey().PubKey()
	return &GenesisDoc{
		GenesisTime:        tmtime.Now(),
		ChainID:            "abc",
		InitialHeight:      1000,
		Validators:         []GenesisValidator{{pubkey.Address(), pubkey, DefaultDashVotingPower, "myval", crypto.RandProTxHash()}},
		ConsensusParams:    DefaultConsensusParams(),
		ThresholdPublicKey: pubkey,
		QuorumHash:         crypto.RandQuorumHash(),
		AppHash:            []byte{1, 2, 3},
	}
}
