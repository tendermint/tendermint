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
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/tmhash"
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
	chainID string) *types.DuplicateVoteEvidence {

	var err error

	v := vote.ToProto()
	v2 := vote2.ToProto()

	vote.Signature, err = val.Key.PrivKey.Sign(types.VoteSignBytes(chainID, v))
	require.NoError(t, err)

	vote2.Signature, err = val.Key.PrivKey.Sign(types.VoteSignBytes(chainID, v2))
	require.NoError(t, err)

	return types.NewDuplicateVoteEvidence(vote, vote2, defaultTestTime)
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
		Type:             tmproto.PrevoteType,
		Timestamp:        defaultTestTime,
		BlockID: types.BlockID{
			Hash: tmhash.Sum([]byte("blockhash")),
			PartSetHeader: types.PartSetHeader{
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
		v.Type = tmproto.PrecommitType
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

		t.Log(correct.Time())

		result, err := c.BroadcastEvidence(correct)
		require.NoError(t, err, "BroadcastEvidence(%s) failed", correct)
		assert.Equal(t, correct.Hash(), result.Hash, "expected result hash to match evidence hash")

		status, err := c.Status()
		require.NoError(t, err)
		err = client.WaitForHeight(c, status.SyncInfo.LatestBlockHeight+2, nil)
		require.NoError(t, err)

		ed25519pub := pv.Key.PubKey.(ed25519.PubKey)
		rawpub := ed25519pub.Bytes()
		result2, err := c.ABCIQuery("/val", rawpub)
		require.NoError(t, err)
		qres := result2.Response
		require.True(t, qres.IsOK())

		var v abci.ValidatorUpdate
		err = abci.ReadMessage(bytes.NewReader(qres.Value), &v)
		require.NoError(t, err, "Error reading query result, value %v", qres.Value)

		pk, err := cryptoenc.PubKeyFromProto(v.PubKey)
		require.NoError(t, err)

		require.EqualValues(t, rawpub, pk, "Stored PubKey not equal with expected, value %v", string(qres.Value))
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
		h2 := &types.SignedHeader{
			Header: &types.Header{
				Version:            h1.Version,
				ChainID:            h1.ChainID,
				Height:             h1.Height,
				Time:               h1.Time,
				LastBlockID:        h1.LastBlockID,
				LastCommitHash:     h1.LastCommitHash,
				DataHash:           h1.DataHash,
				ValidatorsHash:     h1.ValidatorsHash,
				NextValidatorsHash: h1.NextValidatorsHash,
				ConsensusHash:      h1.ConsensusHash,
				AppHash:            crypto.CRandBytes(32),
				LastResultsHash:    h1.LastResultsHash,
				EvidenceHash:       h1.EvidenceHash,
				ProposerAddress:    h1.ProposerAddress,
			},
			Commit: types.NewCommit(h1.Height, 1, h1.Commit.BlockID, h1.Commit.Signatures),
		}
		h2.Commit.BlockID = types.BlockID{
			Hash:          h2.Hash(),
			PartSetHeader: types.PartSetHeader{Total: 1, Hash: crypto.CRandBytes(32)},
		}
		vote := &types.Vote{
			ValidatorAddress: pv.Key.Address,
			ValidatorIndex:   0,
			Height:           h2.Height,
			Round:            h2.Commit.Round,
			Timestamp:        h2.Time,
			Type:             tmproto.PrecommitType,
			BlockID:          h2.Commit.BlockID,
		}

		v := vote.ToProto()
		signBytes, err := pv.Key.PrivKey.Sign(types.VoteSignBytes(chainID, v))
		require.NoError(t, err)
		vote.Signature = v.Signature

		h2.Commit.Signatures[0] = types.NewCommitSigForBlock(signBytes, pv.Key.Address, h2.Time)

		t.Logf("h1 AppHash: %X", h1.AppHash)
		t.Logf("h2 AppHash: %X", h2.AppHash)

		ev := &types.ConflictingHeadersEvidence{
			H1: &h1.SignedHeader,
			H2: h2,
		}

		result, err := c.BroadcastEvidence(ev)
		require.NoError(t, err, "BroadcastEvidence(%s) failed", ev)
		assert.Equal(t, ev.Hash(), result.Hash, "expected result hash to match evidence hash")
	}
}

func TestBroadcastEmptyEvidence(t *testing.T) {
	for _, c := range GetClients() {
		_, err := c.BroadcastEvidence(nil)
		assert.Error(t, err)
	}
}
