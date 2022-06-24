package privval

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func TestGenLoadValidator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	privVal, tempKeyFileName, tempStateFileName := newTestFilePV(t)

	height := int64(100)
	privVal.LastSignState.Height = height
	require.NoError(t, privVal.Save())
	proTxHash, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)
	publicKey, err := privVal.GetFirstPubKey(ctx)
	require.NoError(t, err)
	privVal, err = LoadFilePV(tempKeyFileName, tempStateFileName)
	require.NoError(t, err)
	proTxHash2, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)
	publicKey2, err := privVal.GetFirstPubKey(ctx)
	require.NoError(t, err)
	require.Equal(t, proTxHash, proTxHash2, "expected privval proTxHashes to be the same")
	require.Equal(t, publicKey, publicKey2, "expected privval public keys to be the same")
	require.Equal(t, height, privVal.LastSignState.Height, "expected privval.LastHeight to have been saved")
}

func TestResetValidator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	privVal, _, tempStateFileName := newTestFilePV(t)
	emptyState := FilePVLastSignState{filePath: tempStateFileName}
	quorumHash, err := privVal.GetFirstQuorumHash(ctx)
	assert.NoError(t, err)

	// new priv val has empty state
	assert.Equal(t, privVal.LastSignState, emptyState)

	// test vote
	height, round := int64(10), int32(1)
	voteType := tmproto.PrevoteType
	randBytes := tmrand.Bytes(crypto.HashSize)
	blockID := types.BlockID{Hash: randBytes, PartSetHeader: types.PartSetHeader{}}

	stateID := types.RandStateID().WithHeight(height - 1)

	vote := newVote(privVal.Key.ProTxHash, 0, height, round, voteType, blockID, nil)
	err = privVal.SignVote(ctx, "mychainid", 0, quorumHash, vote.ToProto(), stateID, nil)
	assert.NoError(t, err, "expected no error signing vote")

	// priv val after signing is not same as empty
	assert.NotEqual(t, privVal.LastSignState, emptyState)

	// priv val after AcceptNewConnection is same as empty
	require.NoError(t, privVal.Reset())
	assert.Equal(t, privVal.LastSignState, emptyState)
}

func TestLoadOrGenValidator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempKeyFile, err := os.CreateTemp(t.TempDir(), "priv_validator_key_")
	require.NoError(t, err)
	tempStateFile, err := os.CreateTemp(t.TempDir(), "priv_validator_state_")
	require.NoError(t, err)

	tempKeyFilePath := tempKeyFile.Name()
	if err := os.Remove(tempKeyFilePath); err != nil {
		t.Error(err)
	}
	tempStateFilePath := tempStateFile.Name()
	if err := os.Remove(tempStateFilePath); err != nil {
		t.Error(err)
	}

	privVal, err := LoadOrGenFilePV(tempKeyFilePath, tempStateFilePath)
	require.NoError(t, err)
	proTxHash, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)
	publicKey, err := privVal.GetFirstPubKey(ctx)
	require.NoError(t, err)
	privVal, err = LoadOrGenFilePV(tempKeyFilePath, tempStateFilePath)
	require.NoError(t, err)
	proTxHash2, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)
	publicKey2, err := privVal.GetFirstPubKey(ctx)
	require.NoError(t, err)
	require.Equal(t, proTxHash, proTxHash2, "expected privval proTxHashes to be the same")
	require.Equal(t, publicKey, publicKey2, "expected privval public keys to be the same")
}

func TestUnmarshalValidatorState(t *testing.T) {
	// create some fixed values
	serialized := `{
		"height": "1",
		"round": 1,
		"step": 1
	}`

	val := FilePVLastSignState{}
	err := json.Unmarshal([]byte(serialized), &val)
	require.NoError(t, err)

	// make sure the values match
	assert.EqualValues(t, val.Height, 1)
	assert.EqualValues(t, val.Round, 1)
	assert.EqualValues(t, val.Step, 1)

	// export it and make sure it is the same
	out, err := json.Marshal(val)
	require.NoError(t, err)
	assert.JSONEq(t, serialized, string(out))
}

func TestUnmarshalValidatorKey(t *testing.T) {
	// create some fixed values
	privKey := bls12381.GenPrivKey()
	quorumHash := crypto.RandQuorumHash()
	pubKey := privKey.PubKey()
	pubBytes := pubKey.Bytes()
	privBytes := privKey.Bytes()
	pubB64 := base64.StdEncoding.EncodeToString(pubBytes)
	privB64 := base64.StdEncoding.EncodeToString(privBytes)

	proTxHash := crypto.RandProTxHash()

	serialized := fmt.Sprintf(`{
  "private_keys" : {
    "%s" : {
	  "pub_key": {
	  	"type": "tendermint/PubKeyBLS12381",
	  	"value": "%s"
	  },
	  "priv_key": {
		"type": "tendermint/PrivKeyBLS12381",
		"value": "%s"
	  },
	  "threshold_public_key": {
	    "type": "tendermint/PubKeyBLS12381",
	    "value": "%s"
	  }
    }
  },
  "update_heights":{},
  "first_height_of_quorums":{},
  "pro_tx_hash": "%s"
}`, quorumHash, pubB64, privB64, pubB64, proTxHash)

	val := FilePVKey{}
	err := json.Unmarshal([]byte(serialized), &val)
	require.NoError(t, err)

	// make sure the values match
	assert.EqualValues(t, proTxHash, val.ProTxHash)
	assert.Len(t, val.PrivateKeys, 1)
	for quorumHashString, quorumKeys := range val.PrivateKeys {
		quorumHash2, err := hex.DecodeString(quorumHashString)
		require.NoError(t, err)
		require.EqualValues(t, quorumHash, quorumHash2)
		require.EqualValues(t, pubKey, quorumKeys.PubKey)
		require.EqualValues(t, privKey, quorumKeys.PrivKey)
		require.EqualValues(t, pubKey, quorumKeys.ThresholdPublicKey)
	}
	// export it and make sure it is the same
	out, err := json.Marshal(val)
	require.Nil(t, err, "%+v", err)
	assert.JSONEq(t, serialized, string(out))
}

func TestSignVote(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	privVal, _, _ := newTestFilePV(t)

	randbytes := tmrand.Bytes(crypto.HashSize)
	randbytes2 := tmrand.Bytes(crypto.HashSize)

	block1 := types.BlockID{Hash: randbytes,
		PartSetHeader: types.PartSetHeader{Total: 5, Hash: randbytes}}
	block2 := types.BlockID{Hash: randbytes2,
		PartSetHeader: types.PartSetHeader{Total: 10, Hash: randbytes2}}

	height, round := int64(10), int32(1)
	voteType := tmproto.PrevoteType

	stateID := types.RandStateID().WithHeight(height - 1)

	// sign a vote for first time
	vote := newVote(privVal.Key.ProTxHash, 0, height, round, voteType, block1, nil)
	v := vote.ToProto()

	quorumHash, err := privVal.GetFirstQuorumHash(ctx)
	assert.NoError(t, err)

	err = privVal.SignVote(ctx, "mychainid", 0, quorumHash, v, stateID, nil)
	assert.NoError(t, err, "expected no error signing vote")

	// try to sign the same vote again; should be fine
	err = privVal.SignVote(ctx, "mychainid", 0, quorumHash, v, stateID, nil)
	assert.NoError(t, err, "expected no error on signing same vote")

	// now try some bad votes
	cases := []*types.Vote{
		newVote(privVal.Key.ProTxHash, 0, height, round-1, voteType, block1, nil),   // round regression
		newVote(privVal.Key.ProTxHash, 0, height-1, round, voteType, block1, nil),   // height regression
		newVote(privVal.Key.ProTxHash, 0, height-2, round+4, voteType, block1, nil), // height regression and different round
		newVote(privVal.Key.ProTxHash, 0, height, round, voteType, block2, nil),     // different block
	}

	for _, c := range cases {
		assert.Error(t, privVal.SignVote(ctx, "mychainid", 0, crypto.QuorumHash{}, c.ToProto(), stateID, nil),
			"expected error on signing conflicting vote")
	}

	// try signing a vote with a different time stamp
	blockSignature := vote.BlockSignature
	stateSignature := vote.StateSignature

	err = privVal.SignVote(ctx, "mychainid", 0, quorumHash, v, stateID, nil)
	assert.NoError(t, err)
	assert.Equal(t, blockSignature, vote.BlockSignature)
	assert.Equal(t, stateSignature, vote.StateSignature)
}

func TestSignProposal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	privVal, _, _ := newTestFilePV(t)

	randbytes := tmrand.Bytes(crypto.HashSize)
	randbytes2 := tmrand.Bytes(crypto.HashSize)

	block1 := types.BlockID{Hash: randbytes,
		PartSetHeader: types.PartSetHeader{Total: 5, Hash: randbytes}}
	block2 := types.BlockID{Hash: randbytes2,
		PartSetHeader: types.PartSetHeader{Total: 10, Hash: randbytes2}}
	height, round := int64(10), int32(1)

	quorumHash, err := privVal.GetFirstQuorumHash(ctx)
	assert.NoError(t, err)

	// sign a proposal for first time
	proposal := newProposal(height, 1, round, block1)
	pbp := proposal.ToProto()

	_, err = privVal.SignProposal(ctx, "mychainid", 0, quorumHash, pbp)
	assert.NoError(t, err, "expected no error signing proposal")

	// try to sign the same proposal again; should be fine
	_, err = privVal.SignProposal(ctx, "mychainid", 0, quorumHash, pbp)
	assert.NoError(t, err, "expected no error on signing same proposal")

	// now try some bad Proposals
	cases := []*types.Proposal{
		newProposal(height, 1, round-1, block1),   // round regression
		newProposal(height-1, 1, round, block1),   // height regression
		newProposal(height-2, 1, round+4, block1), // height regression and different round
		newProposal(height, 1, round, block2),     // different block
		newProposal(height, 1, round, block1),     // different timestamp
	}

	for _, c := range cases {
		_, err = privVal.SignProposal(ctx, "mychainid", 0, crypto.QuorumHash{}, c.ToProto())
		assert.Error(t, err, "expected error on signing conflicting proposal")
	}
}

func TestDifferByTimestamp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempKeyFile, err := os.CreateTemp(t.TempDir(), "priv_validator_key_")
	require.NoError(t, err)
	tempStateFile, err := os.CreateTemp(t.TempDir(), "priv_validator_state_")
	require.NoError(t, err)

	privVal := GenFilePV(tempKeyFile.Name(), tempStateFile.Name())
	require.NoError(t, err)
	randbytes := tmrand.Bytes(crypto.HashSize)
	block1 := types.BlockID{Hash: randbytes, PartSetHeader: types.PartSetHeader{Total: 5, Hash: randbytes}}
	height, round := int64(10), int32(1)
	chainID := "mychainid"

	quorumHash, err := privVal.GetFirstQuorumHash(ctx)
	assert.NoError(t, err)

	// test proposal
	{
		proposal := newProposal(height, 1, round, block1)
		pb := proposal.ToProto()
		_, err := privVal.SignProposal(ctx, chainID, 0, quorumHash, pb)
		assert.NoError(t, err, "expected no error signing proposal")

		// manipulate the timestamp
		pb.Timestamp = pb.Timestamp.Add(time.Millisecond)
		var emptySig []byte
		proposal.Signature = emptySig
		_, err = privVal.SignProposal(ctx, "mychainid", 0, quorumHash, pb)
		require.Error(t, err)
	}
}

func TestVoteExtensionsAreAlwaysSigned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const (
		chainID    = "mychainid"
		quorumType = btcjson.LLMQType_5_60
	)

	logger := log.NewTestingLogger(t)

	privVal, _, _ := newTestFilePV(t)
	proTxHash, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)
	quorumHash, err := privVal.GetFirstQuorumHash(ctx)
	require.NoError(t, err)
	pubKey, err := privVal.GetPubKey(ctx, quorumHash)
	require.NoError(t, err)

	blockID := types.BlockID{
		Hash:          tmrand.Bytes(crypto.HashSize),
		PartSetHeader: types.PartSetHeader{Total: 5, Hash: tmrand.Bytes(crypto.HashSize)},
	}

	height, round := int64(10), int32(1)
	voteType := tmproto.PrecommitType
	stateID := types.RandStateID().WithHeight(height - 1)
	exts := types.VoteExtensions{
		tmproto.VoteExtensionType_DEFAULT: []types.VoteExtension{{Extension: []byte("extension")}},
	}
	// We initially sign this vote without an extension
	vote1 := newVote(proTxHash, 0, height, round, voteType, blockID, exts)
	vpb1 := vote1.ToProto()

	err = privVal.SignVote(ctx, chainID, quorumType, quorumHash, vpb1, stateID, logger)
	assert.NoError(t, err, "expected no error signing vote")
	assert.NotNil(t, vpb1.VoteExtensions[0].Signature)

	extSignItem1, err := types.MakeVoteExtensionSignItems(chainID, vpb1, quorumType, quorumHash)
	require.NoError(t, err)
	assert.True(t, pubKey.VerifySignatureDigest(extSignItem1[tmproto.VoteExtensionType_DEFAULT][0].ID, vpb1.VoteExtensions[0].Signature))

	// We duplicate this vote precisely, including its timestamp, but change
	// its extension
	vote2 := vote1.Copy()
	vote2.VoteExtensions = types.VoteExtensions{
		tmproto.VoteExtensionType_DEFAULT: []types.VoteExtension{{Extension: []byte("new extension")}},
	}
	vpb2 := vote2.ToProto()

	err = privVal.SignVote(ctx, chainID, quorumType, quorumHash, vpb2, stateID, logger)
	assert.NoError(t, err, "expected no error signing same vote with manipulated vote extension")

	// We need to ensure that a valid new extension signature has been created
	// that validates against the vote extension sign bytes with the new
	// extension, and does not validate against the vote extension sign bytes
	// with the old extension.
	extSignItem2, err := types.MakeVoteExtensionSignItems(chainID, vpb2, quorumType, quorumHash)
	require.NoError(t, err)
	assert.True(t, pubKey.VerifySignatureDigest(extSignItem2[tmproto.VoteExtensionType_DEFAULT][0].ID, vpb2.VoteExtensions[0].Signature))
	assert.False(t, pubKey.VerifySignatureDigest(extSignItem1[tmproto.VoteExtensionType_DEFAULT][0].ID, vpb2.VoteExtensions[0].Signature))

	vpb2.BlockSignature = nil
	vpb2.StateSignature = nil
	vpb2.VoteExtensions[0].Signature = nil

	err = privVal.SignVote(ctx, chainID, quorumType, quorumHash, vpb2, stateID, logger)
	assert.NoError(t, err, "expected no error signing same vote with manipulated timestamp and vote extension")

	extSignItem3, err := types.MakeVoteExtensionSignItems(chainID, vpb2, quorumType, quorumHash)
	require.NoError(t, err)
	assert.True(t, pubKey.VerifySignatureDigest(extSignItem3[tmproto.VoteExtensionType_DEFAULT][0].ID, vpb2.VoteExtensions[0].Signature))
	assert.False(t, pubKey.VerifySignatureDigest(extSignItem1[tmproto.VoteExtensionType_DEFAULT][0].ID, vpb2.VoteExtensions[0].Signature))
}

func newVote(proTxHash types.ProTxHash, idx int32, height int64, round int32,
	typ tmproto.SignedMsgType, blockID types.BlockID, exts types.VoteExtensions) *types.Vote {
	return &types.Vote{
		ValidatorProTxHash: proTxHash,
		ValidatorIndex:     idx,
		Height:             height,
		Round:              round,
		Type:               typ,
		BlockID:            blockID,
		VoteExtensions:     exts,
	}
}

func newProposal(height int64, coreChainLockedHeight uint32, round int32, blockID types.BlockID) *types.Proposal {
	return &types.Proposal{
		Height:                height,
		CoreChainLockedHeight: coreChainLockedHeight,
		Round:                 round,
		BlockID:               blockID,
		Timestamp:             time.Now(),
	}
}

func newTestFilePV(t *testing.T) (*FilePV, string, string) {
	tempKeyFile, err := os.CreateTemp(t.TempDir(), "priv_validator_key_")
	require.NoError(t, err)
	tempStateFile, err := os.CreateTemp(t.TempDir(), "priv_validator_state_")
	require.NoError(t, err)

	privVal := GenFilePV(tempKeyFile.Name(), tempStateFile.Name())
	require.NoError(t, err)

	return privVal, tempKeyFile.Name(), tempStateFile.Name()
}
