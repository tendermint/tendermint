package client_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
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

func TestBroadcastEvidenceDuplicateVote(t *testing.T) {
	config := rpctest.GetConfig()
	chainID := config.ChainID()
	pvKeyFile := config.PrivValidatorKeyFile()
	pvKeyStateFile := config.PrivValidatorStateFile()
	pv := privval.LoadOrGenFilePV(pvKeyFile, pvKeyStateFile)

	correct, fakes := makeEvidences(t, pv, chainID)
	t.Logf("evidence %v", correct)

	for i, c := range GetClients() {
		t.Logf("client %d", i)

		result, err := c.BroadcastEvidence(correct)
		require.Nil(t, err)
		require.Equal(t, correct.Hash(), result.Hash, "Invalid response, result %+v", result)

		status, err := c.Status()
		require.NoError(t, err)
		client.WaitForHeight(c, status.SyncInfo.LatestBlockHeight+2, nil)

		ed25519pub := correct.PubKey.(ed25519.PubKeyEd25519)
		rawpub := ed25519pub[:]
		result2, err := c.ABCIQuery("/val", rawpub)
		require.Nil(t, err, "Error querying evidence, err %v", err)
		qres := result2.Response
		require.True(t, qres.IsOK(), "Response not OK")

		var v abci.ValidatorUpdate
		err = abci.ReadMessage(bytes.NewReader(qres.Value), &v)
		require.NoError(t, err, "Error reading query result, value %v", qres.Value)

		require.EqualValues(t, rawpub, v.PubKey.Data, "Stored PubKey not equal with expected, value %v", string(qres.Value))
		require.Equal(t, int64(9), v.Power, "Stored Power not equal with expected, value %v", string(qres.Value))

		for _, fake := range fakes {
			_, err := c.BroadcastEvidence(fake)
			require.Error(t, err, "Broadcasting fake evidence succeed: %s", fake.String())
		}
	}
}
