package lite

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	log "github.com/tendermint/tendermint/libs/log"
	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

// missingProvider doesn't store anything, always a miss.
// Designed as a mock for testing.
type missingProvider struct{}

// NewMissingProvider returns a provider which does not store anything and always misses.
func NewMissingProvider() PersistentProvider {
	return missingProvider{}
}

func (missingProvider) SaveFullCommit(FullCommit) error { return nil }
func (missingProvider) LatestFullCommit(chainID string, minHeight, maxHeight int64) (FullCommit, error) {
	return FullCommit{}, lerr.ErrCommitNotFound()
}
func (missingProvider) ValidatorSet(chainID string, height int64) (*types.ValidatorSet, error) {
	return nil, errors.New("missing validator set")
}
func (missingProvider) SetLogger(_ log.Logger) {}

func TestMemProvider(t *testing.T) {
	p := NewDBProvider("mem", dbm.NewMemDB())
	checkProvider(t, p, "test-mem", "empty")
}

func TestMultiProvider(t *testing.T) {
	p := NewMultiProvider(
		NewMissingProvider(),
		NewDBProvider("mem", dbm.NewMemDB()),
		NewMissingProvider(),
	)
	checkProvider(t, p, "test-cache", "kjfhekfhkewhgit")
}

func checkProvider(t *testing.T, p PersistentProvider, chainID, app string) {
	assert, require := assert.New(t), require.New(t)
	appHash := []byte(app)
	keys := genPrivKeys(5)
	count := 10

	// Make a bunch of full commits.
	fcz := make([]FullCommit, count)
	for i := 0; i < count; i++ {
		vals := keys.ToValidators(10, int64(count/2))
		h := int64(20 + 10*i)
		fcz[i] = keys.GenFullCommit(chainID, h, nil, vals, vals, appHash, []byte("params"), []byte("results"), 0, 5)
	}

	// Check that provider is initially empty.
	fc, err := p.LatestFullCommit(chainID, 1, 1<<63-1)
	require.NotNil(err)
	assert.True(lerr.IsErrCommitNotFound(err))

	// Save all full commits to the provider.
	for _, fc := range fcz {
		err = p.SaveFullCommit(fc)
		require.Nil(err)
		// Make sure we can get it back.
		fc2, err := p.LatestFullCommit(chainID, fc.Height(), fc.Height())
		assert.Nil(err)
		assert.Equal(fc.SignedHeader, fc2.SignedHeader)
		assert.Equal(fc.Validators, fc2.Validators)
		assert.Equal(fc.NextValidators, fc2.NextValidators)
	}

	// Make sure we get the last hash if we overstep.
	fc, err = p.LatestFullCommit(chainID, 1, 5000)
	if assert.Nil(err) {
		assert.Equal(fcz[count-1].Height(), fc.Height())
		assert.Equal(fcz[count-1], fc)
	}

	// ... and middle ones as well.
	fc, err = p.LatestFullCommit(chainID, 1, 47)
	if assert.Nil(err) {
		// we only step by 10, so 40 must be the one below this
		assert.EqualValues(40, fc.Height())
	}

}

// This will make a get height, and if it is good, set the data as well.
func checkLatestFullCommit(t *testing.T, p PersistentProvider, chainID string, ask, expect int64) {
	fc, err := p.LatestFullCommit(chainID, 1, ask)
	require.Nil(t, err)
	if assert.Equal(t, expect, fc.Height()) {
		err = p.SaveFullCommit(fc)
		require.Nil(t, err)
	}
}

func TestMultiLatestFullCommit(t *testing.T) {
	require := require.New(t)

	// We will write data to the second level of the cache (p2), and see what
	// gets cached/stored in.
	p := NewDBProvider("mem1", dbm.NewMemDB())
	p2 := NewDBProvider("mem2", dbm.NewMemDB())
	cp := NewMultiProvider(p, p2)

	chainID := "cache-best-height"
	appHash := []byte("01234567")
	keys := genPrivKeys(5)
	count := 10

	// Set a bunch of full commits.
	for i := 0; i < count; i++ {
		vals := keys.ToValidators(10, int64(count/2))
		h := int64(10 * (i + 1))
		fc := keys.GenFullCommit(chainID, h, nil, vals, vals, appHash, []byte("params"), []byte("results"), 0, 5)
		err := p2.SaveFullCommit(fc)
		require.NoError(err)
	}

	// Get a few heights from the cache and set them proper.
	checkLatestFullCommit(t, cp, chainID, 57, 50)
	checkLatestFullCommit(t, cp, chainID, 33, 30)

	// make sure they are set in p as well (but nothing else)
	checkLatestFullCommit(t, p, chainID, 44, 30)
	checkLatestFullCommit(t, p, chainID, 50, 50)
	checkLatestFullCommit(t, p, chainID, 99, 50)

	// now, query the cache for a higher value
	checkLatestFullCommit(t, p2, chainID, 99, 90)
	checkLatestFullCommit(t, cp, chainID, 99, 90)
}
