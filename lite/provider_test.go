// nolint: vetshadow
package lite_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/lite"
	liteErr "github.com/tendermint/tendermint/lite/errors"
)

// missingProvider doesn't store anything, always a miss
// Designed as a mock for testing
type missingProvider struct{}

// NewMissingProvider returns a provider which does not store anything and always misses.
func NewMissingProvider() lite.Provider {
	return missingProvider{}
}

func (missingProvider) StoreCommit(lite.FullCommit) error { return nil }
func (missingProvider) GetByHeight(int64) (lite.FullCommit, error) {
	return lite.FullCommit{}, liteErr.ErrCommitNotFound()
}
func (missingProvider) GetByHash([]byte) (lite.FullCommit, error) {
	return lite.FullCommit{}, liteErr.ErrCommitNotFound()
}
func (missingProvider) LatestCommit() (lite.FullCommit, error) {
	return lite.FullCommit{}, liteErr.ErrCommitNotFound()
}

func TestMemProvider(t *testing.T) {
	p := lite.NewMemStoreProvider()
	checkProvider(t, p, "test-mem", "empty")
}

func TestCacheProvider(t *testing.T) {
	p := lite.NewCacheProvider(
		NewMissingProvider(),
		lite.NewMemStoreProvider(),
		NewMissingProvider(),
	)
	checkProvider(t, p, "test-cache", "kjfhekfhkewhgit")
}

func checkProvider(t *testing.T, p lite.Provider, chainID, app string) {
	assert, require := assert.New(t), require.New(t)
	appHash := []byte(app)
	keys := lite.GenValKeys(5)
	count := 10

	// make a bunch of commits...
	commits := make([]lite.FullCommit, count)
	for i := 0; i < count; i++ {
		// two commits for each validator, to check how we handle dups
		// (10, 0), (10, 1), (10, 1), (10, 2), (10, 2), ...
		vals := keys.ToValidators(10, int64(count/2))
		h := int64(20 + 10*i)
		commits[i] = keys.GenFullCommit(chainID, h, nil, vals, appHash, []byte("params"), []byte("results"), 0, 5)
	}

	// check provider is empty
	fc, err := p.GetByHeight(20)
	require.NotNil(err)
	assert.True(liteErr.IsCommitNotFoundErr(err))

	fc, err = p.GetByHash(commits[3].ValidatorsHash())
	require.NotNil(err)
	assert.True(liteErr.IsCommitNotFoundErr(err))

	// now add them all to the provider
	for _, s := range commits {
		err = p.StoreCommit(s)
		require.Nil(err)
		// and make sure we can get it back
		s2, err := p.GetByHash(s.ValidatorsHash())
		assert.Nil(err)
		assert.Equal(s, s2)
		// by height as well
		s2, err = p.GetByHeight(s.Height())
		assert.Nil(err)
		assert.Equal(s, s2)
	}

	// make sure we get the last hash if we overstep
	fc, err = p.GetByHeight(5000)
	if assert.Nil(err) {
		assert.Equal(commits[count-1].Height(), fc.Height())
		assert.Equal(commits[count-1], fc)
	}

	// and middle ones as well
	fc, err = p.GetByHeight(47)
	if assert.Nil(err) {
		// we only step by 10, so 40 must be the one below this
		assert.EqualValues(40, fc.Height())
	}

}

type binarySearchHeightGetter interface {
	GetByHeightBinarySearch(h int64) (lite.FullCommit, error)
}

// this will make a get height, and if it is good, set the data as well
func checkGetHeight(t *testing.T, p lite.Provider, ask, expect int64) {
	// The goal here is to test checkGetHeight using both
	// provider.GetByHeight
	// *memStoreProvider.GetHeightBinarySearch
	fnMap := map[string]func(int64) (lite.FullCommit, error){
		"getByHeight": p.GetByHeight,
	}
	if bshg, ok := p.(binarySearchHeightGetter); ok {
		fnMap["getByHeightBinary"] = bshg.GetByHeightBinarySearch
	}

	for algo, fn := range fnMap {
		fc, err := fn(ask)
		// t.Logf("%s got=%v want=%d", algo, expect, fc.Height())
		require.Nil(t, err, "%s: %+v", algo, err)
		if assert.Equal(t, expect, fc.Height()) {
			err = p.StoreCommit(fc)
			require.Nil(t, err, "%s: %+v", algo, err)
		}
	}
}

func TestCacheGetsBestHeight(t *testing.T) {
	// assert, require := assert.New(t), require.New(t)
	require := require.New(t)

	// we will write data to the second level of the cache (p2),
	// and see what gets cached, stored in
	p := lite.NewMemStoreProvider()
	p2 := lite.NewMemStoreProvider()
	cp := lite.NewCacheProvider(p, p2)

	chainID := "cache-best-height"
	appHash := []byte("01234567")
	keys := lite.GenValKeys(5)
	count := 10

	// set a bunch of commits
	for i := 0; i < count; i++ {
		vals := keys.ToValidators(10, int64(count/2))
		h := int64(10 * (i + 1))
		fc := keys.GenFullCommit(chainID, h, nil, vals, appHash, []byte("params"), []byte("results"), 0, 5)
		err := p2.StoreCommit(fc)
		require.NoError(err)
	}

	// let's get a few heights from the cache and set them proper
	checkGetHeight(t, cp, 57, 50)
	checkGetHeight(t, cp, 33, 30)

	// make sure they are set in p as well (but nothing else)
	checkGetHeight(t, p, 44, 30)
	checkGetHeight(t, p, 50, 50)
	checkGetHeight(t, p, 99, 50)

	// now, query the cache for a higher value
	checkGetHeight(t, p2, 99, 90)
	checkGetHeight(t, cp, 99, 90)
}

var blankFullCommit lite.FullCommit

func ensureNonExistentCommitsAtHeight(t *testing.T, prefix string, fn func(int64) (lite.FullCommit, error), data []int64) {
	for i, qh := range data {
		fc, err := fn(qh)
		assert.NotNil(t, err, "#%d: %s: height=%d should return non-nil error", i, prefix, qh)
		assert.Equal(t, fc, blankFullCommit, "#%d: %s: height=%d\ngot =%+v\nwant=%+v", i, prefix, qh, fc, blankFullCommit)
	}
}

func TestMemStoreProviderGetByHeightBinaryAndLinearSameResult(t *testing.T) {
	p := lite.NewMemStoreProvider()

	// Store a bunch of commits at specific heights
	// and then ensure that:
	//  * GetByHeight
	//  * GetByHeightBinarySearch
	// both return the exact same result

	// 1. Non-existent height commits
	nonExistent := []int64{-1000, -1, 0, 1, 10, 11, 17, 31, 67, 1000, 1e9}
	ensureNonExistentCommitsAtHeight(t, "GetByHeight", p.GetByHeight, nonExistent)
	ensureNonExistentCommitsAtHeight(t, "GetByHeightBinarySearch", p.(binarySearchHeightGetter).GetByHeightBinarySearch, nonExistent)

	// 2. Save some known height commits
	knownHeights := []int64{0, 1, 7, 9, 12, 13, 18, 44, 23, 16, 1024, 100, 199, 1e9}
	createAndStoreCommits(t, p, knownHeights)

	// 3. Now check if those heights are retrieved
	ensureExistentCommitsAtHeight(t, "GetByHeight", p.GetByHeight, knownHeights)
	ensureExistentCommitsAtHeight(t, "GetByHeightBinarySearch", p.(binarySearchHeightGetter).GetByHeightBinarySearch, knownHeights)

	// 4. And now for the height probing to ensure that any height
	// requested returns a fullCommit of height <= requestedHeight.
	checkGetHeight(t, p, 0, 0)
	checkGetHeight(t, p, 1, 1)
	checkGetHeight(t, p, 2, 1)
	checkGetHeight(t, p, 5, 1)
	checkGetHeight(t, p, 7, 7)
	checkGetHeight(t, p, 10, 9)
	checkGetHeight(t, p, 12, 12)
	checkGetHeight(t, p, 14, 13)
	checkGetHeight(t, p, 19, 18)
	checkGetHeight(t, p, 43, 23)
	checkGetHeight(t, p, 45, 44)
	checkGetHeight(t, p, 1025, 1024)
	checkGetHeight(t, p, 101, 100)
	checkGetHeight(t, p, 1e3, 199)
	checkGetHeight(t, p, 1e4, 1024)
	checkGetHeight(t, p, 1e9, 1e9)
	checkGetHeight(t, p, 1e9+1, 1e9)
}

func ensureExistentCommitsAtHeight(t *testing.T, prefix string, fn func(int64) (lite.FullCommit, error), data []int64) {
	for i, qh := range data {
		fc, err := fn(qh)
		assert.Nil(t, err, "#%d: %s: height=%d should not return an error: %v", i, prefix, qh, err)
		assert.NotEqual(t, fc, blankFullCommit, "#%d: %s: height=%d got a blankCommit", i, prefix, qh)
	}
}

func createAndStoreCommits(t *testing.T, p lite.Provider, heights []int64) {
	chainID := "cache-best-height-binary-and-linear"
	appHash := []byte("0xdeadbeef")
	keys := lite.GenValKeys(len(heights) / 2)

	for _, h := range heights {
		vals := keys.ToValidators(10, int64(len(heights)/2))
		fc := keys.GenFullCommit(chainID, h, nil, vals, appHash, []byte("params"), []byte("results"), 0, 5)
		err := p.StoreCommit(fc)
		require.NoError(t, err, "StoreCommit height=%d", h)
	}
}
