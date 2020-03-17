package client_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/rpc/client"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	"github.com/tendermint/tendermint/types"
)

func newEvidence(t *testing.T, val *privval.FilePV,
	vote *types.Vote, vote2 *types.Vote,
	chainID string) *types.DuplicateVoteEvidence {

	var err error
	vote.Signature, err = val.Key.PrivKey.Sign(vote.SignBytes(chainID))
	require.NoError(t, err)

	vote2.Signature, err = val.Key.PrivKey.Sign(vote2.SignBytes(chainID))
	require.NoError(t, err)

	return types.NewDuplicateVoteEvidence(val.Key.PubKey, vote, vote2)
}

func makeEvidences(
	t *testing.T,
	val *privval.FilePV,
	chainID string,
) (correct *types.DuplicateVoteEvidence, fakes []*types.DuplicateVoteEvidence) {
	vote := types.Vote{
		ValidatorAddress: val.Key.Address,
		ValidatorIndex:   0,
		Height:           1,
		Round:            0,
		Type:             types.PrevoteType,
		Timestamp:        time.Now().UTC(),
		BlockID: types.BlockID{
			Hash: tmhash.Sum([]byte("blockhash")),
			PartsHeader: types.PartSetHeader{
				Total: 1000,
				Hash:  tmhash.Sum([]byte("partset")),
			},
		},
	}

	vote2 := vote
	vote2.BlockID.Hash = tmhash.Sum([]byte("blockhash2"))
	correct = newEvidence(t, val, &vote, &vote2, chainID)

	fakes = make([]*types.DuplicateVoteEvidence, 0)

	// different address
	{
		v := vote2
		v.ValidatorAddress = []byte("some_address")
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID))
	}

	// different index
	{
		v := vote2
		v.ValidatorIndex = vote.ValidatorIndex + 1
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID))
	}

	// different height
	{
		v := vote2
		v.Height = vote.Height + 1
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID))
	}

	// different round
	{
		v := vote2
		v.Round = vote.Round + 1
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID))
	}

	// different type
	{
		v := vote2
		v.Type = types.PrecommitType
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID))
	}

	// exactly same vote
	{
		v := vote
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID))
	}

	return correct, fakes
}

func TestBroadcastEvidence_DuplicateVoteEvidence(t *testing.T) {
	var (
		config  = rpctest.GetConfig()
		chainID = config.ChainID()
		pv      = privval.LoadOrGenFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	)

	correct, fakes := makeEvidences(t, pv, chainID)

	for i, c := range GetClients() {
		t.Logf("client %d", i)

		result, err := c.BroadcastEvidence(correct)
		require.NoError(t, err, "BroadcastEvidence(%s) failed", correct)
		assert.Equal(t, correct.Hash(), result.Hash, "expected result hash to match evidence hash")

		status, err := c.Status()
		require.NoError(t, err)
		client.WaitForHeight(c, status.SyncInfo.LatestBlockHeight+2, nil)

		ed25519pub := correct.PubKey.(ed25519.PubKeyEd25519)
		rawpub := ed25519pub[:]
		result2, err := c.ABCIQuery("/val", rawpub)
		require.NoError(t, err)
		qres := result2.Response
		require.True(t, qres.IsOK())

		var v abci.ValidatorUpdate
		err = abci.ReadMessage(bytes.NewReader(qres.Value), &v)
		require.NoError(t, err, "Error reading query result, value %v", qres.Value)

		require.EqualValues(t, rawpub, v.PubKey.Data, "Stored PubKey not equal with expected, value %v", string(qres.Value))
		require.Equal(t, int64(9), v.Power, "Stored Power not equal with expected, value %v", string(qres.Value))

		for _, fake := range fakes {
			_, err := c.BroadcastEvidence(fake)
			require.Error(t, err, "BroadcastEvidence(%s) succeeded, but the evidence was fake", fake)
		}
	}
}

func TestBroadcastEvidence_ConflictingHeadersEvidence(t *testing.T) {
	var (
		config  = rpctest.GetConfig()
		chainID = config.ChainID()
		pv      = privval.LoadOrGenFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	)

	for i, c := range GetClients() {
		t.Logf("client %d", i)

		h1, err := c.Commit(nil)
		require.NoError(t, err)
		require.NotNil(t, h1.SignedHeader.Header)

		// Create an alternative header with a different AppHash.
		h2 := h1.SignedHeader
		h2.AppHash = []byte("app_hash2")
		blockID := types.BlockID{
			Hash:        h2.Hash(),
			PartsHeader: types.PartSetHeader{Total: 1, Hash: crypto.CRandBytes(32)},
		}
		h2.Commit.BlockID = blockID
		vote := &types.Vote{
			ValidatorAddress: pv.Key.Address,
			ValidatorIndex:   0,
			Height:           h1.Height,
			Round:            h1.Commit.Round,
			Timestamp:        h1.Time,
			Type:             types.PrecommitType,
			BlockID:          blockID,
		}
		signBytes, err := pv.Key.PrivKey.Sign(vote.SignBytes(chainID))
		require.NoError(t, err)
		h2.Commit.Signatures[0] = types.NewCommitSigForBlock(signBytes, pv.Key.Address, h1.Time)

		t.Logf("h1 AppHash: %X", h1.AppHash)
		t.Logf("h2 AppHash: %X", h2.AppHash)

		ev := types.ConflictingHeadersEvidence{
			H1: h1.SignedHeader,
			H2: h2,
		}

		result, err := c.BroadcastEvidence(ev)
		require.NoError(t, err, "BroadcastEvidence(%s) failed", ev)
		assert.Equal(t, ev.Hash(), result.Hash, "expected result hash to match evidence hash")
	}
}
