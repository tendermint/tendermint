package privval

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/encode"
	privvalproto "github.com/tendermint/tendermint/proto/privval"
	tmproto "github.com/tendermint/tendermint/proto/types"
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
	blockID := types.BlockID{Hash: []byte{1, 2, 3}, PartsHeader: types.PartSetHeader{}}
	vote := newVote(privVal.Key.Address, 0, height, round, voteType, blockID)
	err = privVal.SignVote("mychainid", vote)
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

	val := privvalproto.FilePVLastSignState{}
	err := jsonpb.Unmarshal(strings.NewReader(serialized), &val)
	require.NoError(err, "%+v", err)

	// make sure the values match
	assert.EqualValues(val.Height, 1)
	assert.EqualValues(val.Round, 1)
	assert.EqualValues(val.Step, 1)

	// export it and make sure it is the same

	out, err := encode.MarshalJSON(&val)
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
	addrB64 := base64.StdEncoding.EncodeToString(addr.Bytes())
	privB64 := base64.StdEncoding.EncodeToString(privBytes)

	serialized := fmt.Sprintf(`{
	  "address": "%s",
	  "pubKey": {"ed25519":"%s"},
	  "privKey": {"ed25519":"%s"}
	}`, addrB64, pubB64, privB64)

	val := privvalproto.FilePVKey{}
	err := jsonpb.Unmarshal(strings.NewReader(string(serialized)), &val)
	require.NoError(err, "%+v", err)
	fpvk, err := FilePVKeyFromProto(&val)
	require.NoError(err, "%+v", err)

	// make sure the values match
	assert.EqualValues(addr, fpvk.Address)
	assert.EqualValues(pubKey, fpvk.PubKey)
	assert.EqualValues(privKey, fpvk.PrivKey)

	pb, err := fpvk.ToProto()
	require.NoError(err, "%+v", err)

	// export it and make sure it is the same
	out, err := encode.MarshalJSON(pb)
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

	block1 := types.BlockID{Hash: []byte{1, 2, 3}, PartsHeader: types.PartSetHeader{}}
	block2 := types.BlockID{Hash: []byte{3, 2, 1}, PartsHeader: types.PartSetHeader{}}

	height, round := int64(10), int32(1)
	voteType := tmproto.PrevoteType

	// sign a vote for first time
	vote := newVote(privVal.Key.Address, 0, height, round, voteType, block1)
	err = privVal.SignVote("mychainid", vote)
	assert.NoError(err, "expected no error signing vote")

	// try to sign the same vote again; should be fine
	err = privVal.SignVote("mychainid", vote)
	assert.NoError(err, "expected no error on signing same vote")

	// now try some bad votes
	cases := []*types.Vote{
		newVote(privVal.Key.Address, 0, height, round-1, voteType, block1),   // round regression
		newVote(privVal.Key.Address, 0, height-1, round, voteType, block1),   // height regression
		newVote(privVal.Key.Address, 0, height-2, round+4, voteType, block1), // height regression and different round
		newVote(privVal.Key.Address, 0, height, round, voteType, block2),     // different block
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

	tempKeyFile, err := ioutil.TempFile("", "priv_validator_key_")
	require.Nil(t, err)
	tempStateFile, err := ioutil.TempFile("", "priv_validator_state_")
	require.Nil(t, err)

	privVal := GenFilePV(tempKeyFile.Name(), tempStateFile.Name())

	block1 := types.BlockID{Hash: []byte{1, 2, 3}, PartsHeader: types.PartSetHeader{Total: 5, Hash: []byte{1, 2, 3}}}
	block2 := types.BlockID{Hash: []byte{3, 2, 1}, PartsHeader: types.PartSetHeader{Total: 10, Hash: []byte{3, 2, 1}}}
	height, round := int64(10), int32(1)

	// sign a proposal for first time
	proposal := newProposal(height, round, block1)
	err = privVal.SignProposal("mychainid", proposal)
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
	tempKeyFile, err := ioutil.TempFile("", "priv_validator_key_")
	require.Nil(t, err)
	tempStateFile, err := ioutil.TempFile("", "priv_validator_state_")
	require.Nil(t, err)

	privVal := GenFilePV(tempKeyFile.Name(), tempStateFile.Name())

	block1 := types.BlockID{Hash: []byte{1, 2, 3}, PartsHeader: types.PartSetHeader{Total: 5, Hash: []byte{1, 2, 3}}}
	height, round := int64(10), int32(1)
	chainID := "mychainid"

	// test proposal
	{
		proposal := newProposal(height, round, block1)
		err := privVal.SignProposal(chainID, proposal)
		assert.NoError(t, err, "expected no error signing proposal")
		signBytes := proposal.SignBytes(chainID)
		sig := proposal.Signature
		timeStamp := proposal.Timestamp

		// manipulate the timestamp. should get changed back
		proposal.Timestamp = proposal.Timestamp.Add(time.Millisecond)
		var emptySig []byte
		proposal.Signature = emptySig
		err = privVal.SignProposal("mychainid", proposal)
		assert.NoError(t, err, "expected no error on signing same proposal")

		assert.Equal(t, timeStamp, proposal.Timestamp)
		assert.Equal(t, signBytes, proposal.SignBytes(chainID))
		assert.Equal(t, sig, proposal.Signature)
	}

	// test vote
	{
		voteType := tmproto.PrevoteType
		blockID := types.BlockID{Hash: []byte{1, 2, 3}, PartsHeader: types.PartSetHeader{}}
		vote := newVote(privVal.Key.Address, 0, height, round, voteType, blockID)
		err := privVal.SignVote("mychainid", vote)
		assert.NoError(t, err, "expected no error signing vote")

		signBytes := vote.SignBytes(chainID)
		sig := vote.Signature
		timeStamp := vote.Timestamp

		// manipulate the timestamp. should get changed back
		vote.Timestamp = vote.Timestamp.Add(time.Millisecond)
		var emptySig []byte
		vote.Signature = emptySig
		err = privVal.SignVote("mychainid", vote)
		assert.NoError(t, err, "expected no error on signing same vote")

		assert.Equal(t, timeStamp, vote.Timestamp)
		assert.Equal(t, signBytes, vote.SignBytes(chainID))
		assert.Equal(t, sig, vote.Signature)
	}
}

func newVote(addr types.Address, idx uint32, height int64,
	round int32, typ tmproto.SignedMsgType, blockID types.BlockID) *types.Vote {
	return &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           height,
		Round:            round,
		Type:             typ,
		Timestamp:        tmtime.Now(),
		BlockID:          blockID,
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

func TestFilePVKeyProtobuf(t *testing.T) {
	privKey := ed25519.GenPrivKey()

	f1 := FilePVKey{
		Address:  privKey.PubKey().Address(),
		PubKey:   privKey.PubKey(),
		PrivKey:  privKey,
		filePath: "keyFilePath",
	}
	f2 := FilePVKey{
		Address:  privKey.PubKey().Address(),
		PubKey:   nil,
		PrivKey:  privKey,
		filePath: "keyFilePath",
	}
	f3 := FilePVKey{
		Address:  privKey.PubKey().Address(),
		PubKey:   privKey.PubKey(),
		PrivKey:  nil,
		filePath: "keyFilePath",
	}

	type cases struct {
		testname string
		fpk      *FilePVKey
		expPass  bool
		expPass2 bool
	}

	testCases := []cases{
		{"success valid FilePVKey", &f1, true, true},
		{"fail pubkey empty", &f2, false, false},
		{"fail priv empty", &f3, false, false},
		{"fail nil FilePVKey", nil, false, false},
	}

	for _, tc := range testCases {
		tc := tc

		pb, err := tc.fpk.ToProto()
		if !tc.expPass {
			require.Error(t, err)
		}

		fpvk, err := FilePVKeyFromProto(pb)
		if tc.expPass2 {
			require.Equal(t, tc.fpk, &fpvk, tc.testname)
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func TestFilePVLastSignStateProtobuf(t *testing.T) {
	path := "stateFilePath "

	f1 := FilePVLastSignState{
		Step:     stepNone,
		filePath: path,
	}

	f2 := FilePVLastSignState{
		Height:    1,
		Round:     2,
		Step:      stepPropose,
		Signature: []byte("sigs"),
		SignBytes: []byte("signBytes"),

		filePath: path,
	}

	type cases struct {
		testname string
		fpk      *FilePVLastSignState
		expPass  bool
		expPass2 bool
	}

	testCases := []cases{
		{"success valid FilePVKey", &f1, true, true},
		{"success valid FilePVKey", &f2, true, true},
		{"fail nil FilePVKey", nil, false, false},
	}

	for _, tc := range testCases {
		tc := tc

		pb, err := tc.fpk.ToProto()
		if !tc.expPass {
			require.Error(t, err)
		}

		fpvss, err := FilePVLastSignStateFromProto(pb)
		if tc.expPass2 {
			require.Equal(t, tc.fpk, fpvss, tc.testname)
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}
