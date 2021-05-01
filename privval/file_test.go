package privval

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/tmhash"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func TestGenLoadValidator(t *testing.T) {
	assert := assert.New(t)

	tempKeyFile, err := ioutil.TempFile("", "priv_validator_key_")
	require.Nil(t, err)
	tempStateFile, err := ioutil.TempFile("", "priv_validator_state_")
	require.Nil(t, err)

	privVal := GenFilePV(tempKeyFile.Name(), tempStateFile.Name())

	height := int64(100)
	privVal.LastSignState.Height = height
	privVal.Save()
	addr := privVal.GetAddress()

	privVal = LoadFilePV(tempKeyFile.Name(), tempStateFile.Name())
	assert.Equal(addr, privVal.GetAddress(), "expected privval addr to be the same")
	assert.Equal(height, privVal.LastSignState.Height, "expected privval.LastHeight to have been saved")
}

func TestResetValidator(t *testing.T) {
	tempKeyFile, err := ioutil.TempFile("", "priv_validator_key_")
	require.Nil(t, err)
	tempStateFile, err := ioutil.TempFile("", "priv_validator_state_")
	require.Nil(t, err)

	privVal := GenFilePV(tempKeyFile.Name(), tempStateFile.Name())
	emptyState := FilePVLastSignState{filePath: tempStateFile.Name()}

	// new priv val has empty state
	assert.Equal(t, privVal.LastSignState, emptyState)

	// test vote
	height, round := int64(10), int32(1)
	voteType := tmproto.PrevoteType
	randBytes := tmrand.Bytes(tmhash.Size)
	randStateBytes := tmrand.Bytes(tmhash.Size)
	blockID := types.BlockID{Hash: randBytes, PartSetHeader: types.PartSetHeader{}}
	stateID := types.StateID{LastAppHash: randStateBytes}
	vote := newVote(privVal.Key.ProTxHash, 0, height, round, voteType, blockID, stateID)
	err = privVal.SignVote("mychainid", 0, crypto.QuorumHash{}, vote.ToProto())
	assert.NoError(t, err, "expected no error signing vote")

	// priv val after signing is not same as empty
	assert.NotEqual(t, privVal.LastSignState, emptyState)

	// priv val after AcceptNewConnection is same as empty
	privVal.Reset()
	assert.Equal(t, privVal.LastSignState, emptyState)
}

func TestLoadOrGenValidator(t *testing.T) {
	assert := assert.New(t)

	tempKeyFile, err := ioutil.TempFile("", "priv_validator_key_")
	require.Nil(t, err)
	tempStateFile, err := ioutil.TempFile("", "priv_validator_state_")
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
	privKey := bls12381.GenPrivKey()
	pubKey := privKey.PubKey()
	addr := pubKey.Address()
	pubBytes := pubKey.Bytes()
	privBytes := privKey.Bytes()
	pubB64 := base64.StdEncoding.EncodeToString(pubBytes)
	privB64 := base64.StdEncoding.EncodeToString(privBytes)

	proTxHash := crypto.RandProTxHash()

	serialized := fmt.Sprintf(`{
  "address": "%s",
  "pub_key": {
    "type": "tendermint/PubKeyBLS12381",
    "value": "%s"
  },
  "priv_key": {
    "type": "tendermint/PrivKeyBLS12381",
    "value": "%s"
  },
  "pro_tx_hash": "%s"
}`, addr, pubB64, privB64, proTxHash)

	val := FilePVKey{}
	err := tmjson.Unmarshal([]byte(serialized), &val)
	require.Nil(err, "%+v", err)

	// make sure the values match
	assert.EqualValues(addr, val.Address)
	assert.EqualValues(pubKey, val.PubKey)
	assert.EqualValues(privKey, val.PrivKey)
	assert.EqualValues(proTxHash, val.ProTxHash)

	// export it and make sure it is the same
	out, err := tmjson.Marshal(val)
	require.Nil(err, "%+v", err)
	assert.JSONEq(serialized, string(out))
}

func TestSignVote(t *testing.T) {
	assert := assert.New(t)

	tempKeyFile, err := ioutil.TempFile("", "priv_validator_key_")
	require.Nil(t, err)
	tempStateFile, err := ioutil.TempFile("", "priv_validator_state_")
	require.Nil(t, err)

	privVal := GenFilePV(tempKeyFile.Name(), tempStateFile.Name())

	randbytes := tmrand.Bytes(tmhash.Size)
	randbytes2 := tmrand.Bytes(tmhash.Size)
	randLastAppHash := tmrand.Bytes(tmhash.Size)

	block1 := types.BlockID{Hash: randbytes,
		PartSetHeader: types.PartSetHeader{Total: 5, Hash: randbytes}}
	block2 := types.BlockID{Hash: randbytes2,
		PartSetHeader: types.PartSetHeader{Total: 10, Hash: randbytes2}}
	state := types.StateID{LastAppHash: randLastAppHash}

	height, round := int64(10), int32(1)
	voteType := tmproto.PrevoteType

	// sign a vote for first time
	vote := newVote(privVal.Key.ProTxHash, 0, height, round, voteType, block1, state)
	v := vote.ToProto()
	err = privVal.SignVote("mychainid", 0, crypto.QuorumHash{}, v)
	assert.NoError(err, "expected no error signing vote")

	// try to sign the same vote again; should be fine
	err = privVal.SignVote("mychainid", 0,  crypto.QuorumHash{}, v)
	assert.NoError(err, "expected no error on signing same vote")

	// now try some bad votes
	cases := []*types.Vote{
		newVote(privVal.Key.ProTxHash, 0, height, round-1, voteType, block1, state),   // round regression
		newVote(privVal.Key.ProTxHash, 0, height-1, round, voteType, block1, state),   // height regression
		newVote(privVal.Key.ProTxHash, 0, height-2, round+4, voteType, block1, state), // height reg and diff round
		newVote(privVal.Key.ProTxHash, 0, height, round, voteType, block2, state),     // different block
	}

	for _, c := range cases {
		cpb := c.ToProto()
		err = privVal.SignVote("mychainid", 0, crypto.QuorumHash{}, cpb)
		assert.Error(err, "expected error on signing conflicting vote")
	}

	// try signing a vote with a different time stamp
	blockSignature := vote.BlockSignature
	stateSignature := vote.StateSignature

	err = privVal.SignVote("mychainid", 0, crypto.QuorumHash{}, v)
	assert.NoError(err)
	assert.Equal(blockSignature, vote.BlockSignature)
	assert.Equal(stateSignature, vote.StateSignature)
}

func TestSignProposal(t *testing.T) {
	assert := assert.New(t)

	tempKeyFile, err := ioutil.TempFile("", "priv_validator_key_")
	require.Nil(t, err)
	tempStateFile, err := ioutil.TempFile("", "priv_validator_state_")
	require.Nil(t, err)

	privVal := GenFilePV(tempKeyFile.Name(), tempStateFile.Name())

	randbytes := tmrand.Bytes(tmhash.Size)
	randbytes2 := tmrand.Bytes(tmhash.Size)

	block1 := types.BlockID{Hash: randbytes,
		PartSetHeader: types.PartSetHeader{Total: 5, Hash: randbytes}}
	block2 := types.BlockID{Hash: randbytes2,
		PartSetHeader: types.PartSetHeader{Total: 10, Hash: randbytes2}}
	height, round := int64(10), int32(1)

	// sign a proposal for first time
	proposal := newProposal(height, 1, round, block1)
	pbp := proposal.ToProto()
	err = privVal.SignProposal("mychainid", 0, crypto.QuorumHash{}, pbp)
	assert.NoError(err, "expected no error signing proposal")

	// try to sign the same proposal again; should be fine
	err = privVal.SignProposal("mychainid", 0, crypto.QuorumHash{}, pbp)
	assert.NoError(err, "expected no error on signing same proposal")

	// now try some bad Proposals
	cases := []*types.Proposal{
		newProposal(height, 1, round-1, block1),   // round regression
		newProposal(height-1, 1, round, block1),   // height regression
		newProposal(height-2, 1, round+4, block1), // height regression and different round
		newProposal(height, 1, round, block2),     // different block
	}

	for _, c := range cases {
		err = privVal.SignProposal("mychainid", 0, crypto.QuorumHash{}, c.ToProto())
		assert.Error(err, "expected error on signing conflicting proposal")
	}

	// try signing a proposal with a different time stamp
	sig := proposal.Signature
	proposal.Timestamp = proposal.Timestamp.Add(time.Duration(1000))
	err = privVal.SignProposal("mychainid", 0, crypto.QuorumHash{}, pbp)
	assert.NoError(err)
	assert.Equal(sig, proposal.Signature)
}

func TestDifferByTimestamp(t *testing.T) {
	tempKeyFile, err := ioutil.TempFile("", "priv_validator_key_")
	require.Nil(t, err)
	tempStateFile, err := ioutil.TempFile("", "priv_validator_state_")
	require.Nil(t, err)

	privVal := GenFilePV(tempKeyFile.Name(), tempStateFile.Name())
	randbytes := tmrand.Bytes(tmhash.Size)
	block1 := types.BlockID{Hash: randbytes, PartSetHeader: types.PartSetHeader{Total: 5, Hash: randbytes}}
	height, round := int64(10), int32(1)
	chainID := "mychainid"

	// test proposal
	{
		proposal := newProposal(height, 1, round, block1)
		pb := proposal.ToProto()
		err := privVal.SignProposal(chainID, 0, crypto.QuorumHash{}, pb)
		assert.NoError(t, err, "expected no error signing proposal")
		signBytes := types.ProposalBlockSignBytes(chainID, pb)

		sig := proposal.Signature
		timeStamp := proposal.Timestamp

		// manipulate the timestamp. should get changed back
		pb.Timestamp = pb.Timestamp.Add(time.Millisecond)
		var emptySig []byte
		proposal.Signature = emptySig
		err = privVal.SignProposal("mychainid", 0, crypto.QuorumHash{}, pb)
		assert.NoError(t, err, "expected no error on signing same proposal")

		assert.Equal(t, timeStamp, pb.Timestamp)
		assert.Equal(t, signBytes, types.ProposalBlockSignBytes(chainID, pb))
		assert.Equal(t, sig, proposal.Signature)
	}
}

func newVote(proTxHash types.ProTxHash, idx int32, height int64, round int32,
	typ tmproto.SignedMsgType, blockID types.BlockID, stateID types.StateID) *types.Vote {
	return &types.Vote{
		ValidatorProTxHash: proTxHash,
		ValidatorIndex:     idx,
		Height:             height,
		Round:              round,
		Type:               typ,
		BlockID:            blockID,
		StateID:            stateID,
	}
}

func newProposal(height int64, coreChainLockedHeight uint32, round int32, blockID types.BlockID) *types.Proposal {
	return &types.Proposal{
		Height:                height,
		CoreChainLockedHeight: coreChainLockedHeight,
		Round:                 round,
		BlockID:               blockID,
		Timestamp:             tmtime.Now(),
	}
}
