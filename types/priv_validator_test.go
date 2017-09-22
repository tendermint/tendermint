package types

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
)

func TestGenLoadValidator(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	privVal := GenPrivValidatorFS(tempFilePath)

	height := 100
	privVal.LastHeight = height
	privVal.Save()
	addr := privVal.GetAddress()

	privVal = LoadPrivValidatorFS(tempFilePath)
	assert.Equal(addr, privVal.GetAddress(), "expected privval addr to be the same")
	assert.Equal(height, privVal.LastHeight, "expected privval.LastHeight to have been saved")
}

func TestLoadOrGenValidator(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	os.Remove(tempFilePath)
	privVal := LoadOrGenPrivValidatorFS(tempFilePath)
	addr := privVal.GetAddress()
	privVal = LoadOrGenPrivValidatorFS(tempFilePath)
	assert.Equal(addr, privVal.GetAddress(), "expected privval addr to be the same")
}

func TestUnmarshalValidator(t *testing.T) {
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
  "last_height": 0,
  "last_round": 0,
  "last_step": 0,
  "last_signature": null,
  "priv_key": {
    "type": "ed25519",
    "data": "%s"
  }
}`, addrStr, pubStr, privStr)

	val := PrivValidatorFS{}
	err = json.Unmarshal([]byte(serialized), &val)
	require.Nil(err, "%+v", err)

	// make sure the values match
	assert.EqualValues(addrBytes, val.GetAddress())
	assert.EqualValues(pubKey, val.GetPubKey())
	assert.EqualValues(privKey, val.PrivKey)

	// export it and make sure it is the same
	out, err := json.Marshal(val)
	require.Nil(err, "%+v", err)
	assert.JSONEq(serialized, string(out))
}

func TestSignVote(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	privVal := GenPrivValidatorFS(tempFilePath)

	block1 := BlockID{[]byte{1, 2, 3}, PartSetHeader{}}
	block2 := BlockID{[]byte{3, 2, 1}, PartSetHeader{}}
	height, round := 10, 1
	voteType := VoteTypePrevote

	// sign a vote for first time
	vote := newVote(privVal.Address, 0, height, round, voteType, block1)
	err := privVal.SignVote("mychainid", vote)
	assert.NoError(err, "expected no error signing vote")

	// try to sign the same vote again; should be fine
	err = privVal.SignVote("mychainid", vote)
	assert.NoError(err, "expected no error on signing same vote")

	// now try some bad votes
	cases := []*Vote{
		newVote(privVal.Address, 0, height, round-1, voteType, block1),   // round regression
		newVote(privVal.Address, 0, height-1, round, voteType, block1),   // height regression
		newVote(privVal.Address, 0, height-2, round+4, voteType, block1), // height regression and different round
		newVote(privVal.Address, 0, height, round, voteType, block2),     // different block
	}

	for _, c := range cases {
		err = privVal.SignVote("mychainid", c)
		assert.Error(err, "expected error on signing conflicting vote")
	}
}

func TestSignProposal(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	privVal := GenPrivValidatorFS(tempFilePath)

	block1 := PartSetHeader{5, []byte{1, 2, 3}}
	block2 := PartSetHeader{10, []byte{3, 2, 1}}
	height, round := 10, 1

	// sign a proposal for first time
	proposal := newProposal(height, round, block1)
	err := privVal.SignProposal("mychainid", proposal)
	assert.NoError(err, "expected no error signing proposal")

	// try to sign the same proposal again; should be fine
	err = privVal.SignProposal("mychainid", proposal)
	assert.NoError(err, "expected no error on signing same proposal")

	// now try some bad Proposals
	cases := []*Proposal{
		newProposal(height, round-1, block1),   // round regression
		newProposal(height-1, round, block1),   // height regression
		newProposal(height-2, round+4, block1), // height regression and different round
		newProposal(height, round, block2),     // different block
	}

	for _, c := range cases {
		err = privVal.SignProposal("mychainid", c)
		assert.Error(err, "expected error on signing conflicting proposal")
	}
}

func newVote(addr data.Bytes, idx, height, round int, typ byte, blockID BlockID) *Vote {
	return &Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           height,
		Round:            round,
		Type:             typ,
		BlockID:          blockID,
	}
}

func newProposal(height, round int, partsHeader PartSetHeader) *Proposal {
	return &Proposal{
		Height:           height,
		Round:            round,
		BlockPartsHeader: partsHeader,
	}
}
