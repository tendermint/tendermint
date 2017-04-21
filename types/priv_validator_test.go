package types

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	crypto "github.com/tendermint/go-crypto"
)

func TestLoadValidator(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// create some fixed values
	addrStr := "D028C9981F7A87F3093672BF0D5B0E2A1B3ED456"
	pubStr := "3B3069C422E19688B45CBFAE7BB009FC0FA1B1EA86593519318B7214853803C8"
	privStr := "27F82582AEFAE7AB151CFB01C48BB6C1A0DA78F9BDDA979A9F70A84D074EB07D3B3069C422E19688B45CBFAE7BB009FC0FA1B1EA86593519318B7214853803C8"
	addrBytes, _ := hex.DecodeString(addrStr)
	pubBytes, _ := hex.DecodeString(pubStr)
	privBytes, _ := hex.DecodeString(privStr)

	// prepend type byte
	pubKey, err := crypto.PubKeyFromBytes(append([]byte{1}, pubBytes...))
	require.Nil(err, "%+v", err)
	privKey, err := crypto.PrivKeyFromBytes(append([]byte{1}, privBytes...))
	require.Nil(err, "%+v", err)

	serialized := fmt.Sprintf(`{
  "address": "%s",
  "pub_key": {
    "type": "ed25519",
    "data": "%s"
  },
  "priv_key": {
    "type": "ed25519",
    "data": "%s"
  },
  "last_height": 0,
  "last_round": 0,
  "last_step": 0,
  "last_signature": null
}`, addrStr, pubStr, privStr)

	val := PrivValidator{}
	err = json.Unmarshal([]byte(serialized), &val)
	require.Nil(err, "%+v", err)

	// make sure the values match
	assert.EqualValues(addrBytes, val.Address)
	assert.EqualValues(pubKey, val.PubKey)
	assert.EqualValues(privKey, val.PrivKey)

	// export it and make sure it is the same
	out, err := json.Marshal(val)
	require.Nil(err, "%+v", err)
	assert.JSONEq(serialized, string(out))
}
