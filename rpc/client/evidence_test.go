package client_test

import (
	"bytes"
	"context"
	"github.com/dashevo/dashd-go/btcjson"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto"

	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/rpc/client"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	"github.com/tendermint/tendermint/types"
)

// For some reason the empty node used in tests has a time of
// 2018-10-10 08:20:13.695936996 +0000 UTC
// this is because the test genesis time is set here
// so in order to validate evidence we need evidence to be the same time
var defaultTestTime = time.Date(2018, 10, 10, 8, 20, 13, 695936996, time.UTC)

func newEvidence(t *testing.T, val *privval.FilePV,
	vote *types.Vote, vote2 *types.Vote,
	chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash) *types.DuplicateVoteEvidence {

	var err error

	v := vote.ToProto()
	v2 := vote2.ToProto()

	vote.BlockSignature, err = val.Key.PrivKey.SignDigest(types.VoteBlockSignId(chainID, v, quorumType, quorumHash))
	require.NoError(t, err)

	vote2.BlockSignature, err = val.Key.PrivKey.SignDigest(types.VoteBlockSignId(chainID, v2, quorumType, quorumHash))
	require.NoError(t, err)

	vote.StateSignature, err = val.Key.PrivKey.SignDigest(types.VoteStateSignId(chainID, v, quorumType, quorumHash))
	require.NoError(t, err)

	vote2.StateSignature, err = val.Key.PrivKey.SignDigest(types.VoteStateSignId(chainID, v2, quorumType, quorumHash))
	require.NoError(t, err)

	validator := types.NewValidator(val.Key.PubKey, 100, val.Key.ProTxHash)
	valSet := types.NewValidatorSet([]*types.Validator{validator}, validator.PubKey, quorumType, quorumHash, true)

	return types.NewDuplicateVoteEvidence(vote, vote2, defaultTestTime, valSet)
}

func makeEvidences(
	t *testing.T,
	val *privval.FilePV,
	chainID string,
    quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
) (correct *types.DuplicateVoteEvidence, fakes []*types.DuplicateVoteEvidence) {
	vote := types.Vote{
		ValidatorProTxHash: val.Key.ProTxHash,
		ValidatorIndex:     0,
		Height:             1,
		Round:              0,
		Type:               tmproto.PrevoteType,
		BlockID: types.BlockID{
			Hash: tmhash.Sum(tmrand.Bytes(tmhash.Size)),
			PartSetHeader: types.PartSetHeader{
				Total: 1000,
				Hash:  tmhash.Sum([]byte("partset")),
			},
		},
		StateID: types.StateID{
			LastAppHash: tmhash.Sum(tmrand.Bytes(tmhash.Size)),
		},
	}

	vote2 := vote
	vote2.BlockID.Hash = tmhash.Sum([]byte("blockhash2"))
	correct = newEvidence(t, val, &vote, &vote2, chainID, quorumType, quorumHash)

	fakes = make([]*types.DuplicateVoteEvidence, 0)

	// different address
	{
		v := vote2
		v.ValidatorProTxHash = []byte("some_pro_tx_hash")
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID, quorumType, quorumHash))
	}

	// different height
	{
		v := vote2
		v.Height = vote.Height + 1
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID, quorumType, quorumHash))
	}

	// different round
	{
		v := vote2
		v.Round = vote.Round + 1
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID, quorumType, quorumHash))
	}

	// different type
	{
		v := vote2
		v.Type = tmproto.PrecommitType
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID, quorumType, quorumHash))
	}

	// exactly same vote
	{
		v := vote
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID, quorumType, quorumHash))
	}

	return correct, fakes
}

func TestBroadcastEvidence_DuplicateVoteEvidence(t *testing.T) {
	var (
		config  = rpctest.GetConfig()
		chainID = config.ChainID()
		pv      = privval.LoadOrGenFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	)

	for i, c := range GetClients() {
		h := int64(1)
		vals, err := c.Validators(context.Background(), &h, nil, nil, nil)
		correct, fakes := makeEvidences(t, pv, chainID, vals.QuorumType, *vals.QuorumHash)
		t.Logf("client %d", i)

		result, err := c.BroadcastEvidence(context.Background(), correct)
		require.NoError(t, err, "BroadcastEvidence(%s) failed", correct)
		assert.Equal(t, correct.Hash(), result.Hash, "expected result hash to match evidence hash")

		status, err := c.Status(context.Background())
		require.NoError(t, err)
		err = client.WaitForHeight(c, status.SyncInfo.LatestBlockHeight+2, nil)
		require.NoError(t, err)

		proTxHash := pv.Key.ProTxHash
		bls12381pub := pv.Key.PubKey.(bls12381.PubKey)
		rawpub := bls12381pub.Bytes()
		result2, err := c.ABCIQuery(context.Background(), "/val", proTxHash.Bytes())
		require.NoError(t, err)
		qres := result2.Response
		require.True(t, qres.IsOK())

		var v abci.ValidatorUpdate
		err = abci.ReadMessage(bytes.NewReader(qres.Value), &v)
		require.NoError(t, err, "Error reading query result, value %v", qres.Value)

		pk, err := cryptoenc.PubKeyFromProto(v.PubKey)
		require.NoError(t, err)

		require.EqualValues(t, rawpub, pk, "Stored PubKey not equal with expected, value %v", string(qres.Value))
		require.Equal(t, types.DefaultDashVotingPower, v.Power,
			"Stored Power not equal with expected, value %v", string(qres.Value))

		for _, fake := range fakes {
			_, err := c.BroadcastEvidence(context.Background(), fake)
			require.Error(t, err, "BroadcastEvidence(%s) succeeded, but the evidence was fake", fake)
		}
	}
}

func TestBroadcastEmptyEvidence(t *testing.T) {
	for _, c := range GetClients() {
		_, err := c.BroadcastEvidence(context.Background(), nil)
		assert.Error(t, err)
	}
}
