package light_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/light/provider"
	mockp "github.com/tendermint/tendermint/light/provider/mock"
	dbs "github.com/tendermint/tendermint/light/store/db"
	"github.com/tendermint/tendermint/types"
)

func TestClientReportsConflictingHeadersEvidence(t *testing.T) {
	// fullNode2 sends us different header
	altH2 := keys.GenSignedHeaderLastBlockID(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
		hash("app_hash2"), hash("cons_hash"), hash("results_hash"),
		0, len(keys), types.BlockID{Hash: h1.Hash()})
	fullNode2 := mockp.New(
		chainID,
		map[int64]*types.SignedHeader{
			1: h1,
			2: altH2,
		},
		map[int64]*types.ValidatorSet{
			1: vals,
			2: vals,
		},
	)

	c, err := light.NewClient(
		chainID,
		trustOptions,
		fullNode,
		[]provider.Provider{fullNode2},
		dbs.New(dbm.NewMemDB(), chainID),
		light.Logger(log.TestingLogger()),
		light.MaxRetryAttempts(1),
	)
	require.NoError(t, err)

	// Check verification returns an error.
	_, err = c.VerifyLightBlockAtHeight(2, bTime.Add(2*time.Hour))
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "does not match primary")
	}

	// Check evidence was sent to both full nodes.
	ev := &types.ConflictingHeadersEvidence{H1: h2, H2: altH2}
	assert.True(t, fullNode2.HasEvidence(ev))
	assert.True(t, fullNode.HasEvidence(ev))
}

func TestClientDivergentTraces(t *testing.T) {
	primary := mockp.New(GenMockNode(chainID, 10, 5, 2, bTime))
	firstBlock, _ := primary.LightBlock(1)
	witness := mockp.New(GenMockNode(chainID, 10, 5, 2, bTime))

	c, err := light.NewClient(
		chainID,
		light.TrustOptions{
			Height: 1,
			Hash:   firstBlock.Hash(),
			Period: 4 * time.Hour,
		},
		primary,
		[]provider.Provider{witness},
		dbs.New(dbm.NewMemDB(), chainID),
		light.Logger(log.TestingLogger()),
		light.MaxRetryAttempts(1),
	)
	require.NoError(t, err)

	// 1. Different nodes therefore a divergent header is produced but the
	// light client can't verify it because it has a different trusted header.
	_, err = c.VerifyLightBlockAtHeight(10, bTime.Add(1*time.Hour))
	assert.Error(t, err)
	assert.Equal(t, 0, len(c.Witnesses()))

	// 2. Two out of three nodes don't respond but the third has a header that matches
	// verification should be successful and all the witnesses should remain

	c, err = light.NewClient(
		chainID,
		light.TrustOptions{
			Height: 1,
			Hash:   firstBlock.Hash(),
			Period: 4 * time.Hour,
		},
		primary,
		[]provider.Provider{deadNode, deadNode, primary},
		dbs.New(dbm.NewMemDB(), chainID),
		light.Logger(log.TestingLogger()),
		light.MaxRetryAttempts(1),
	)
	require.NoError(t, err)
}
