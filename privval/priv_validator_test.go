package privval

import (
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tendermint/libs/common"
)

func TestGenLoadValidator(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	privVal := GenFilePV(tempFilePath)

	height := int64(100)
	privVal.LastHeight = height
	privVal.Save()
	addr := privVal.GetAddress()

	privVal = LoadFilePV(tempFilePath)
	assert.Equal(addr, privVal.GetAddress(), "expected privval addr to be the same")
	assert.Equal(height, privVal.LastHeight, "expected privval.LastHeight to have been saved")
}

func TestLoadOrGenValidator(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	if err := os.Remove(tempFilePath); err != nil {
		t.Error(err)
	}
	privVal := LoadOrGenFilePV(tempFilePath)
	addr := privVal.GetAddress()
	privVal = LoadOrGenFilePV(tempFilePath)
	assert.Equal(addr, privVal.GetAddress(), "expected privval addr to be the same")
}

func TestUnmarshalValidator(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// create some fixed values
	privKey := crypto.GenPrivKeyEd25519()
	pubKey := privKey.PubKey()
	addr := pubKey.Address()
	pubArray := [32]byte(pubKey.(crypto.PubKeyEd25519))
	pubBytes := pubArray[:]
	privArray := [64]byte(privKey)
	privBytes := privArray[:]
	pubB64 := base64.StdEncoding.EncodeToString(pubBytes)
	privB64 := base64.StdEncoding.EncodeToString(privBytes)

	serialized := fmt.Sprintf(`{
  "address": "%s",
  "pub_key": {
    "type": "tendermint/PubKeyEd25519",
    "value": "%s"
  },
  "last_height": "0",
  "last_round": "0",
  "last_step": 0,
  "priv_key": {
    "type": "tendermint/PrivKeyEd25519",
    "value": "%s"
  }
}`, addr, pubB64, privB64)

	val := FilePV{}
	err := cdc.UnmarshalJSON([]byte(serialized), &val)
	require.Nil(err, "%+v", err)

	// make sure the values match
	assert.EqualValues(addr, val.GetAddress())
	assert.EqualValues(pubKey, val.GetPubKey())
	assert.EqualValues(privKey, val.PrivKey)

	// export it and make sure it is the same
	out, err := cdc.MarshalJSON(val)
	require.Nil(err, "%+v", err)
	assert.JSONEq(serialized, string(out))
}

func TestSignVote(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	privVal := GenFilePV(tempFilePath)

	block1 := types.BlockID{[]byte{1, 2, 3}, types.PartSetHeader{}}
	block2 := types.BlockID{[]byte{3, 2, 1}, types.PartSetHeader{}}
	height, round := int64(10), 1
	voteType := types.VoteTypePrevote

	// sign a vote for first time
	vote := newVote(privVal.Address, 0, height, round, voteType, block1)
	err := privVal.SignVote("mychainid", vote)
	assert.NoError(err, "expected no error signing vote")

	// try to sign the same vote again; should be fine
	err = privVal.SignVote("mychainid", vote)
	assert.NoError(err, "expected no error on signing same vote")

	// now try some bad votes
	cases := []*types.Vote{
		newVote(privVal.Address, 0, height, round-1, voteType, block1),   // round regression
		newVote(privVal.Address, 0, height-1, round, voteType, block1),   // height regression
		newVote(privVal.Address, 0, height-2, round+4, voteType, block1), // height regression and different round
		newVote(privVal.Address, 0, height, round, voteType, block2),     // different block
	}

	for _, c := range cases {
		err = privVal.SignVote("mychainid", c)
		assert.Error(err, "expected error on signing conflicting vote")
	}

	// try signing a vote with a different time stamp
	sig := vote.Signature
	vote.Timestamp = vote.Timestamp.Add(time.Duration(1000))
	err = privVal.SignVote("mychainid", vote)
	assert.NoError(err)
	assert.Equal(sig, vote.Signature)
}

func TestSignProposal(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	privVal := GenFilePV(tempFilePath)

	block1 := types.PartSetHeader{5, []byte{1, 2, 3}}
	block2 := types.PartSetHeader{10, []byte{3, 2, 1}}
	height, round := int64(10), 1

	// sign a proposal for first time
	proposal := newProposal(height, round, block1)
	err := privVal.SignProposal("mychainid", proposal)
	assert.NoError(err, "expected no error signing proposal")

	// try to sign the same proposal again; should be fine
	err = privVal.SignProposal("mychainid", proposal)
	assert.NoError(err, "expected no error on signing same proposal")

	// now try some bad Proposals
	cases := []*types.Proposal{
		newProposal(height, round-1, block1),   // round regression
		newProposal(height-1, round, block1),   // height regression
		newProposal(height-2, round+4, block1), // height regression and different round
		newProposal(height, round, block2),     // different block
	}

	for _, c := range cases {
		err = privVal.SignProposal("mychainid", c)
		assert.Error(err, "expected error on signing conflicting proposal")
	}

	// try signing a proposal with a different time stamp
	sig := proposal.Signature
	proposal.Timestamp = proposal.Timestamp.Add(time.Duration(1000))
	err = privVal.SignProposal("mychainid", proposal)
	assert.NoError(err)
	assert.Equal(sig, proposal.Signature)
}

func TestDifferByTimestamp(t *testing.T) {
	_, tempFilePath := cmn.Tempfile("priv_validator_")
	privVal := GenFilePV(tempFilePath)

	block1 := types.PartSetHeader{5, []byte{1, 2, 3}}
	height, round := int64(10), 1
	chainID := "mychainid"

	// test proposal
	{
		proposal := newProposal(height, round, block1)
		err := privVal.SignProposal(chainID, proposal)
		assert.NoError(t, err, "expected no error signing proposal")
		signBytes := proposal.SignBytes(chainID)
		sig := proposal.Signature
		timeStamp := clipToMS(proposal.Timestamp)

		// manipulate the timestamp. should get changed back
		proposal.Timestamp = proposal.Timestamp.Add(time.Millisecond)
		var emptySig crypto.Signature
		proposal.Signature = emptySig
		err = privVal.SignProposal("mychainid", proposal)
		assert.NoError(t, err, "expected no error on signing same proposal")

		assert.Equal(t, timeStamp, proposal.Timestamp)
		assert.Equal(t, signBytes, proposal.SignBytes(chainID))
		assert.Equal(t, sig, proposal.Signature)
	}

	// test vote
	{
		voteType := types.VoteTypePrevote
		blockID := types.BlockID{[]byte{1, 2, 3}, types.PartSetHeader{}}
		vote := newVote(privVal.Address, 0, height, round, voteType, blockID)
		err := privVal.SignVote("mychainid", vote)
		assert.NoError(t, err, "expected no error signing vote")

		signBytes := vote.SignBytes(chainID)
		sig := vote.Signature
		timeStamp := clipToMS(vote.Timestamp)

		// manipulate the timestamp. should get changed back
		vote.Timestamp = vote.Timestamp.Add(time.Millisecond)
		var emptySig crypto.Signature
		vote.Signature = emptySig
		err = privVal.SignVote("mychainid", vote)
		assert.NoError(t, err, "expected no error on signing same vote")

		assert.Equal(t, timeStamp, vote.Timestamp)
		assert.Equal(t, signBytes, vote.SignBytes(chainID))
		assert.Equal(t, sig, vote.Signature)
	}
}

func newVote(addr types.Address, idx int, height int64, round int, typ byte, blockID types.BlockID) *types.Vote {
	return &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           height,
		Round:            round,
		Type:             typ,
		Timestamp:        time.Now().UTC(),
		BlockID:          blockID,
	}
}

func newProposal(height int64, round int, partsHeader types.PartSetHeader) *types.Proposal {
	return &types.Proposal{
		Height:           height,
		Round:            round,
		BlockPartsHeader: partsHeader,
		Timestamp:        time.Now().UTC(),
	}
}

func clipToMS(t time.Time) time.Time {
	nano := t.UnixNano()
	million := int64(1000000)
	nano = (nano / million) * million
	return time.Unix(0, nano).UTC()
}
