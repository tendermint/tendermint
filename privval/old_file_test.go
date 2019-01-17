package privval_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/privval"
)

const oldPrivvalContent = `{
  "address": "1D8089FAFDFAE4A637F3D616E17B92905FA2D91D",
  "pub_key": {
    "type": "tendermint/PubKeyEd25519",
    "value": "r3Yg2AhDZ745CNTpavsGU+mRZ8WpRXqoJuyqjN8mJq0="
  },
  "last_height": "5",
  "last_round": "0",
  "last_step": 3,
  "last_signature": "CTr7b9ZQlrJJf+12rPl5t/YSCUc/KqV7jQogCfFJA24e7hof69X6OMT7eFLVQHyodPjD/QTA298XHV5ejxInDQ==",
  "last_signbytes": "750802110500000000000000220B08B398F3E00510F48DA6402A480A20FC258973076512999C3E6839A22E9FBDB1B77CF993E8A9955412A41A59D4CAD312240A20C971B286ACB8AAA6FCA0365EB0A660B189EDC08B46B5AF2995DEFA51A28D215B10013211746573742D636861696E2D533245415533",
  "priv_key": {
    "type": "tendermint/PrivKeyEd25519",
    "value": "7MwvTGEWWjsYwjn2IpRb+GYsWi9nnFsw8jPLLY1UtP6vdiDYCENnvjkI1Olq+wZT6ZFnxalFeqgm7KqM3yYmrQ=="
  }
}`

func TestLoadAndUpgrade(t *testing.T) {

	oldFilePath := initTmpOldFile(t)
	defer os.Remove(oldFilePath)
	newStateFile, err := ioutil.TempFile("", "priv_validator_state*.json")
	defer os.Remove(newStateFile.Name())
	require.NoError(t, err)
	newKeyFile, err := ioutil.TempFile("", "priv_validator_key*.json")
	defer os.Remove(newKeyFile.Name())
	require.NoError(t, err)

	oldPV, err := privval.LoadOldFilePV(oldFilePath)
	assert.NoError(t, err)
	newPV := oldPV.Upgrade(newKeyFile.Name(), newStateFile.Name())

	assertEqualPV(t, oldPV, newPV)
	assert.NoError(t, err)
	upgradedPV := privval.LoadFilePV(newKeyFile.Name(), newStateFile.Name())
	assertEqualPV(t, oldPV, upgradedPV)
	oldPV, err = privval.LoadOldFilePV(oldFilePath + ".bak")
	require.NoError(t, err)
	assertEqualPV(t, oldPV, upgradedPV)
}

func assertEqualPV(t *testing.T, oldPV *privval.OldFilePV, newPV *privval.FilePV) {
	assert.Equal(t, oldPV.Address, newPV.Key.Address)
	assert.Equal(t, oldPV.Address, newPV.GetAddress())
	assert.Equal(t, oldPV.PubKey, newPV.Key.PubKey)
	assert.Equal(t, oldPV.PubKey, newPV.GetPubKey())
	assert.Equal(t, oldPV.PrivKey, newPV.Key.PrivKey)

	assert.Equal(t, oldPV.LastHeight, newPV.LastSignState.Height)
	assert.Equal(t, oldPV.LastRound, newPV.LastSignState.Round)
	assert.Equal(t, oldPV.LastSignature, newPV.LastSignState.Signature)
	assert.Equal(t, oldPV.LastSignBytes, newPV.LastSignState.SignBytes)
	assert.Equal(t, oldPV.LastStep, newPV.LastSignState.Step)
}

func initTmpOldFile(t *testing.T) string {
	tmpfile, err := ioutil.TempFile("", "priv_validator_*.json")
	require.NoError(t, err)
	t.Logf("created test file %s", tmpfile.Name())
	_, err = tmpfile.WriteString(oldPrivvalContent)
	require.NoError(t, err)

	return tmpfile.Name()
}
