package privval

import (
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func TestGenLoadValidator(t *testing.T) {
	privVal, tempKeyFileName, tempStateFileName := newTestFilePV(t)

	height := int64(100)
	privVal.LastSignState.Height = height
	privVal.Save()
	addr := privVal.GetAddress()

	privVal = LoadFilePV(tempKeyFileName, tempStateFileName)
	assert.Equal(t, addr, privVal.GetAddress(), "expected privval addr to be the same")
	assert.Equal(t, height, privVal.LastSignState.Height, "expected privval.LastHeight to have been saved")
}

func TestResetValidator(t *testing.T) {
	privVal, _, tempStateFileName := newTestFilePV(t)
	emptyState := FilePVLastSignState{filePath: tempStateFileName}

	// new priv val has empty state
	assert.Equal(t, privVal.LastSignState, emptyState)

	// test vote
	height, round := int64(10), int32(1)
	voteType := tmproto.PrevoteType
	randBytes := tmrand.Bytes(tmhash.Size)
	blockID := types.BlockID{Hash: randBytes, PartSetHeader: types.PartSetHeader{}}
	vote := newVote(privVal.Key.Address, 0, height, round, voteType, blockID, nil)
	err := privVal.SignVote("mychainid", vote.ToProto())
	assert.NoError(t, err, "expected no error signing vote")

	// priv val after signing is not same as empty
	assert.NotEqual(t, privVal.LastSignState, emptyState)

	// priv val after AcceptNewConnection is same as empty
	privVal.Reset()
	assert.Equal(t, privVal.LastSignState, emptyState)
}

func TestLoadOrGenValidator(t *testing.T) {
	assert := assert.New(t)

	tempKeyFile, err := os.CreateTemp("", "priv_validator_key_")
	require.Nil(t, err)
	tempStateFile, err := os.CreateTemp("", "priv_validator_state_")
	require.Nil(t, err)

	tempKeyFilePath := tempKeyFile.Name()
	if err := os.Remove(tempKeyFilePath); err != nil {
		t.Error(err)
	}
	tempStateFilePath := tempStateFile.Name()
	if err := os.Remove(tempStateFilePath); err != nil {
		t.Error(err)
	}

	privVal := LoadOrGenFilePV(tempKeyFilePath, tempStateFilePath)
	addr := privVal.GetAddress()
	privVal = LoadOrGenFilePV(tempKeyFilePath, tempStateFilePath)
	assert.Equal(addr, privVal.GetAddress(), "expected privval addr to be the same")
}

func TestUnmarshalValidatorState(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// create some fixed values
	serialized := `{
		"height": "1",
		"round": 1,
		"step": 1
	}`

	val := FilePVLastSignState{}
	err := tmjson.Unmarshal([]byte(serialized), &val)
	require.Nil(err, "%+v", err)

	// make sure the values match
	assert.EqualValues(val.Height, 1)
	assert.EqualValues(val.Round, 1)
	assert.EqualValues(val.Step, 1)

	// export it and make sure it is the same
	out, err := tmjson.Marshal(val)
	require.Nil(err, "%+v", err)
	assert.JSONEq(serialized, string(out))
}

func TestUnmarshalValidatorKey(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// create some fixed values
	privKey := ed25519.GenPrivKey()
	pubKey := privKey.PubKey()
	addr := pubKey.Address()
	pubBytes := pubKey.Bytes()
	privBytes := privKey.Bytes()
	pubB64 := base64.StdEncoding.EncodeToString(pubBytes)
	privB64 := base64.StdEncoding.EncodeToString(privBytes)

	serialized := fmt.Sprintf(`{
  "address": "%s",
  "pub_key": {
    "type": "tendermint/PubKeyEd25519",
    "value": "%s"
  },
  "priv_key": {
    "type": "tendermint/PrivKeyEd25519",
    "value": "%s"
  }
}`, addr, pubB64, privB64)

	val := FilePVKey{}
	err := tmjson.Unmarshal([]byte(serialized), &val)
	require.Nil(err, "%+v", err)

	// make sure the values match
	assert.EqualValues(addr, val.Address)
	assert.EqualValues(pubKey, val.PubKey)
	assert.EqualValues(privKey, val.PrivKey)

	// export it and make sure it is the same
	out, err := tmjson.Marshal(val)
	require.Nil(err, "%+v", err)
	assert.JSONEq(serialized, string(out))
}

func TestSignVote(t *testing.T) {
	assert := assert.New(t)

	privVal, _, _ := newTestFilePV(t)

	randbytes := tmrand.Bytes(tmhash.Size)
	randbytes2 := tmrand.Bytes(tmhash.Size)

	block1 := types.BlockID{Hash: randbytes,
		PartSetHeader: types.PartSetHeader{Total: 5, Hash: randbytes}}
	block2 := types.BlockID{Hash: randbytes2,
		PartSetHeader: types.PartSetHeader{Total: 10, Hash: randbytes2}}

	height, round := int64(10), int32(1)
	voteType := tmproto.PrevoteType

	// sign a vote for first time
	vote := newVote(privVal.Key.Address, 0, height, round, voteType, block1, nil)
	v := vote.ToProto()
	err := privVal.SignVote("mychainid", v)
	assert.NoError(err, "expected no error signing vote")

	// try to sign the same vote again; should be fine
	err = privVal.SignVote("mychainid", v)
	assert.NoError(err, "expected no error on signing same vote")

	// now try some bad votes
	cases := []*types.Vote{
		newVote(privVal.Key.Address, 0, height, round-1, voteType, block1, nil),   // round regression
		newVote(privVal.Key.Address, 0, height-1, round, voteType, block1, nil),   // height regression
		newVote(privVal.Key.Address, 0, height-2, round+4, voteType, block1, nil), // height regression and different round
		newVote(privVal.Key.Address, 0, height, round, voteType, block2, nil),     // different block
	}

	for _, c := range cases {
		cpb := c.ToProto()
		err = privVal.SignVote("mychainid", cpb)
		assert.Error(err, "expected error on signing conflicting vote")
	}

	// try signing a vote with a different time stamp
	sig := vote.Signature
	vote.Timestamp = vote.Timestamp.Add(time.Duration(1000))
	err = privVal.SignVote("mychainid", v)
	assert.NoError(err)
	assert.Equal(sig, vote.Signature)
}

func TestSignProposal(t *testing.T) {
	assert := assert.New(t)

	privVal, _, _ := newTestFilePV(t)

	randbytes := tmrand.Bytes(tmhash.Size)
	randbytes2 := tmrand.Bytes(tmhash.Size)

	block1 := types.BlockID{Hash: randbytes,
		PartSetHeader: types.PartSetHeader{Total: 5, Hash: randbytes}}
	block2 := types.BlockID{Hash: randbytes2,
		PartSetHeader: types.PartSetHeader{Total: 10, Hash: randbytes2}}
	height, round := int64(10), int32(1)

	// sign a proposal for first time
	proposal := newProposal(height, round, block1)
	pbp := proposal.ToProto()
	err := privVal.SignProposal("mychainid", pbp)
	assert.NoError(err, "expected no error signing proposal")

	// try to sign the same proposal again; should be fine
	err = privVal.SignProposal("mychainid", pbp)
	assert.NoError(err, "expected no error on signing same proposal")

	// now try some bad Proposals
	cases := []*types.Proposal{
		newProposal(height, round-1, block1),   // round regression
		newProposal(height-1, round, block1),   // height regression
		newProposal(height-2, round+4, block1), // height regression and different round
		newProposal(height, round, block2),     // different block
	}

	for _, c := range cases {
		err = privVal.SignProposal("mychainid", c.ToProto())
		assert.Error(err, "expected error on signing conflicting proposal")
	}

	// try signing a proposal with a different time stamp
	sig := proposal.Signature
	proposal.Timestamp = proposal.Timestamp.Add(time.Duration(1000))
	err = privVal.SignProposal("mychainid", pbp)
	assert.NoError(err)
	assert.Equal(sig, proposal.Signature)
}

func TestDifferByTimestamp(t *testing.T) {
	tempKeyFile, err := os.CreateTemp("", "priv_validator_key_")
	require.Nil(t, err)
	tempStateFile, err := os.CreateTemp("", "priv_validator_state_")
	require.Nil(t, err)

	privVal := GenFilePV(tempKeyFile.Name(), tempStateFile.Name())
	randbytes := tmrand.Bytes(tmhash.Size)
	block1 := types.BlockID{Hash: randbytes, PartSetHeader: types.PartSetHeader{Total: 5, Hash: randbytes}}
	height, round := int64(10), int32(1)
	chainID := "mychainid"

	// test proposal
	{
		proposal := newProposal(height, round, block1)
		pb := proposal.ToProto()
		err := privVal.SignProposal(chainID, pb)
		assert.NoError(t, err, "expected no error signing proposal")
		signBytes := types.ProposalSignBytes(chainID, pb)

		sig := proposal.Signature
		timeStamp := proposal.Timestamp

		// manipulate the timestamp. should get changed back
		pb.Timestamp = pb.Timestamp.Add(time.Millisecond)
		var emptySig []byte
		proposal.Signature = emptySig
		err = privVal.SignProposal("mychainid", pb)
		assert.NoError(t, err, "expected no error on signing same proposal")

		assert.Equal(t, timeStamp, pb.Timestamp)
		assert.Equal(t, signBytes, types.ProposalSignBytes(chainID, pb))
		assert.Equal(t, sig, proposal.Signature)
	}

	// test vote
	{
		voteType := tmproto.PrevoteType
		blockID := types.BlockID{Hash: randbytes, PartSetHeader: types.PartSetHeader{}}
		vote := newVote(privVal.Key.Address, 0, height, round, voteType, blockID, nil)
		v := vote.ToProto()
		err := privVal.SignVote("mychainid", v)
		assert.NoError(t, err, "expected no error signing vote")

		signBytes := types.VoteSignBytes(chainID, v)
		sig := v.Signature
		extSig := v.ExtensionSignature
		timeStamp := vote.Timestamp

		// manipulate the timestamp. should get changed back
		v.Timestamp = v.Timestamp.Add(time.Millisecond)
		var emptySig []byte
		v.Signature = emptySig
		v.ExtensionSignature = emptySig
		err = privVal.SignVote("mychainid", v)
		assert.NoError(t, err, "expected no error on signing same vote")

		assert.Equal(t, timeStamp, v.Timestamp)
		assert.Equal(t, signBytes, types.VoteSignBytes(chainID, v))
		assert.Equal(t, sig, v.Signature)
		assert.Equal(t, extSig, v.ExtensionSignature)
	}
}

func TestVoteExtensionsAreAlwaysSigned(t *testing.T) {
	privVal, _, _ := newTestFilePV(t)
	pubKey, err := privVal.GetPubKey()
	assert.NoError(t, err)

	block := types.BlockID{
		Hash:          tmrand.Bytes(tmhash.Size),
		PartSetHeader: types.PartSetHeader{Total: 5, Hash: tmrand.Bytes(tmhash.Size)},
	}

	height, round := int64(10), int32(1)
	voteType := tmproto.PrecommitType

	// We initially sign this vote without an extension
	vote1 := newVote(privVal.Key.Address, 0, height, round, voteType, block, nil)
	vpb1 := vote1.ToProto()

	err = privVal.SignVote("mychainid", vpb1)
	assert.NoError(t, err, "expected no error signing vote")
	assert.NotNil(t, vpb1.ExtensionSignature)

	vesb1 := types.VoteExtensionSignBytes("mychainid", vpb1)
	assert.True(t, pubKey.VerifySignature(vesb1, vpb1.ExtensionSignature))

	// We duplicate this vote precisely, including its timestamp, but change
	// its extension
	vote2 := vote1.Copy()
	vote2.Extension = []byte("new extension")
	vpb2 := vote2.ToProto()

	err = privVal.SignVote("mychainid", vpb2)
	assert.NoError(t, err, "expected no error signing same vote with manipulated vote extension")

	// We need to ensure that a valid new extension signature has been created
	// that validates against the vote extension sign bytes with the new
	// extension, and does not validate against the vote extension sign bytes
	// with the old extension.
	vesb2 := types.VoteExtensionSignBytes("mychainid", vpb2)
	assert.True(t, pubKey.VerifySignature(vesb2, vpb2.ExtensionSignature))
	assert.False(t, pubKey.VerifySignature(vesb1, vpb2.ExtensionSignature))

	// We now manipulate the timestamp of the vote with the extension, as per
	// TestDifferByTimestamp
	expectedTimestamp := vpb2.Timestamp

	vpb2.Timestamp = vpb2.Timestamp.Add(time.Millisecond)
	vpb2.Signature = nil
	vpb2.ExtensionSignature = nil

	err = privVal.SignVote("mychainid", vpb2)
	assert.NoError(t, err, "expected no error signing same vote with manipulated timestamp and vote extension")
	assert.Equal(t, expectedTimestamp, vpb2.Timestamp)

	vesb3 := types.VoteExtensionSignBytes("mychainid", vpb2)
	assert.True(t, pubKey.VerifySignature(vesb3, vpb2.ExtensionSignature))
	assert.False(t, pubKey.VerifySignature(vesb1, vpb2.ExtensionSignature))
}

func newVote(addr types.Address, idx int32, height int64, round int32,
	typ tmproto.SignedMsgType, blockID types.BlockID, extension []byte) *types.Vote {
	return &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           height,
		Round:            round,
		Type:             typ,
		Timestamp:        tmtime.Now(),
		BlockID:          blockID,
		Extension:        extension,
	}
}

func newProposal(height int64, round int32, blockID types.BlockID) *types.Proposal {
	return &types.Proposal{
		Height:    height,
		Round:     round,
		BlockID:   blockID,
		Timestamp: tmtime.Now(),
	}
}

func newTestFilePV(t *testing.T) (*FilePV, string, string) {
	tempKeyFile, err := os.CreateTemp(t.TempDir(), "priv_validator_key_")
	require.NoError(t, err)
	tempStateFile, err := os.CreateTemp(t.TempDir(), "priv_validator_state_")
	require.NoError(t, err)

	privVal := GenFilePV(tempKeyFile.Name(), tempStateFile.Name())

	return privVal, tempKeyFile.Name(), tempStateFile.Name()
}
