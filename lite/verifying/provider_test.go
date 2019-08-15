package verifying

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	log "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/lite"
	pks "github.com/tendermint/tendermint/lite/internal/privkeys"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

func TestProviderValidPath(t *testing.T) {
	require := require.New(t)
	trust := lite.NewDBProvider("trust", dbm.NewMemDB())
	source := lite.NewDBProvider("source", dbm.NewMemDB())

	// Set up the validators to generate test blocks.
	var vote int64 = 10
	keys := pks.GenPrivKeys(5)
	nkeys := keys.Extend(1)

	// Construct a bunch of commits, each with one more height than the last.
	chainID := "inquiry-test"
	consHash := []byte("params")
	resHash := []byte("results")
	count := 50
	fcz := make([]lite.FullCommit, count)
	for i := 0; i < count; i++ {
		vals := keys.ToValidators(vote, 0)
		nextVals := nkeys.ToValidators(vote, 0)
		h := int64(1 + i)
		appHash := []byte(fmt.Sprintf("h=%d", h))
		signedHeader := keys.GenSignedHeader(
			chainID, h, nil,
			vals, nextVals,
			appHash, consHash, resHash, 0, len(keys))
		fcz[i] = lite.NewFullCommit(signedHeader, vals, nextVals)
		// Extend the keys by 1 each time.
		keys = nkeys
		nkeys = nkeys.Extend(1)
	}

	// Initialize a Verifier with the initial state.
	err := trust.SaveFullCommit(fcz[0])
	require.NoError(err)
	vp, _ := NewProvider(chainID, trust, source)
	vp.SetLogger(log.TestingLogger())

	// The latest commit is the first one.
	fc, err := vp.LatestFullCommit(chainID, 0, fcz[count-1].SignedHeader.Height)
	require.NoError(err)
	require.NoError(fc.ValidateFull(chainID))
	require.Equal(fcz[0].SignedHeader, fc.SignedHeader)

	// Adding a few commits in the middle should be insufficient.
	// The latest commit is still the first one.
	for i := 10; i < 13; i++ {
		err := source.SaveFullCommit(fcz[i])
		require.NoError(err)
	}
	fc, err = vp.LatestFullCommit(chainID, 0, fcz[count-1].SignedHeader.Height)
	require.NoError(err)
	require.NoError(fc.ValidateFull(chainID))
	require.Equal(fcz[0].SignedHeader, fc.SignedHeader)

	// With more info, we succeed.
	for i := 0; i < count; i++ {
		err := source.SaveFullCommit(fcz[i])
		require.NoError(err)
	}
	fc, err = vp.LatestFullCommit(chainID, 0, fcz[count-1].SignedHeader.Height)
	require.NoError(err)
	require.NoError(fc.ValidateFull(chainID))
	require.Equal(fcz[count-1].SignedHeader, fc.SignedHeader)
}

func TestProviderDynamicVerification(t *testing.T) {
	trust := lite.NewDBProvider("trust", dbm.NewMemDB())
	source := lite.NewDBProvider("source", dbm.NewMemDB())

	// 10 commits with one valset, 1 to change,
	// 10 commits with the next one
	n1, n2 := 10, 10
	nCommits := n1 + n2 + 1
	maxHeight := int64(nCommits)
	fcz := make([]lite.FullCommit, nCommits)

	// gen the 2 val sets
	chainID := "dynamic-verifier"
	power := int64(10)
	keys1 := pks.GenPrivKeys(5)
	vals1 := keys1.ToValidators(power, 0)
	keys2 := pks.GenPrivKeys(5)
	vals2 := keys2.ToValidators(power, 0)

	// make some commits with the first
	for i := 0; i < n1; i++ {
		fcz[i] = makeFullCommit(int64(i), keys1, vals1, vals1, chainID)
	}

	// update the val set
	fcz[n1] = makeFullCommit(int64(n1), keys1, vals1, vals2, chainID)

	// make some commits with the new one
	for i := n1 + 1; i < nCommits; i++ {
		fcz[i] = makeFullCommit(int64(i), keys2, vals2, vals2, chainID)
	}

	// Save everything in the source
	for _, fc := range fcz {
		source.SaveFullCommit(fc)
	}

	// Initialize a Verifier with the initial state.
	err := trust.SaveFullCommit(fcz[0])
	require.NoError(t, err)
	vp, _ := NewProvider(chainID, trust, source)
	vp.SetLogger(log.TestingLogger())

	// fetch the latest from the source
	latestFC, err := source.LatestFullCommit(chainID, 1, maxHeight)
	require.NoError(t, err)
	require.NoError(latestFC.ValidateFull(chainID))
	require.Equal(fcz[nCommits-1].SignedHeader, latestFC.SignedHeader)
}

func makeFullCommit(height int64, keys lite.PrivKeys, vals, nextVals *types.ValidatorSet, chainID string) lite.FullCommit {
	height++
	consHash := []byte("special-params")
	appHash := []byte(fmt.Sprintf("h=%d", height))
	resHash := []byte(fmt.Sprintf("res=%d", height))
	signedHeader := keys.GenSignedHeader(
		chainID, height, nil,
		vals, nextVals,
		appHash, consHash, resHash, 0, len(keys))
	return lite.NewFullCommit(signedHeader, vals, nextVals)
}

func TestVerifingProviderHistorical(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	trust := lite.NewDBProvider("trust", dbm.NewMemDB())
	source := lite.NewDBProvider("source", dbm.NewMemDB())

	// Set up the validators to generate test blocks.
	var vote int64 = 10
	keys := pks.GenPrivKeys(5)
	nkeys := keys.Extend(1)

	// Construct a bunch of commits, each with one more height than the last.
	chainID := "inquiry-test"
	count := 10
	consHash := []byte("special-params")
	fcz := make([]lite.FullCommit, count)
	for i := 0; i < count; i++ {
		vals := keys.ToValidators(vote, 0)
		nextVals := nkeys.ToValidators(vote, 0)
		h := int64(1 + i)
		appHash := []byte(fmt.Sprintf("h=%d", h))
		resHash := []byte(fmt.Sprintf("res=%d", h))
		signedHeader := keys.GenSignedHeader(
			chainID, h, nil,
			vals, nextVals,
			appHash, consHash, resHash, 0, len(keys))
		fcz[i] = lite.NewFullCommit(signedHeader, vals, nextVals)
		// Extend the keys by 1 each time.
		keys = nkeys
		nkeys = nkeys.Extend(1)
	}

	// Initialize a Verifier with the initial state.
	err := trust.SaveFullCommit(fcz[0])
	require.NoError(err)
	vp, _ := NewProvider(chainID, trust, source)
	vp.SetLogger(log.TestingLogger())

	// Store a few full commits as trust.
	for _, i := range []int{2, 5} {
		trust.SaveFullCommit(fcz[i])
	}

	// See if we can jump forward using trusted full commits.
	// Souce doesn't have fcz[9] so vp.LastTrustedHeight wont' change.
	err = source.SaveFullCommit(fcz[7])
	require.NoError(err, "%+v", err)
	assert.Equal(fcz[7].Height(), vp.LastTrustedHeight())
	fc_, err := trust.LatestFullCommit(chainID, fcz[8].Height(), fcz[8].Height())
	require.Error(err, "%+v", err)
	assert.Equal((lite.FullCommit{}), fc_)

	// With fcz[9] Verify will update last trusted height.
	err = source.SaveFullCommit(fcz[9])
	require.NoError(err, "%+v", err)
	assert.Equal(fcz[8].Height(), vp.LastTrustedHeight())
	fc_, err = trust.LatestFullCommit(chainID, fcz[8].Height(), fcz[8].Height())
	require.NoError(err, "%+v", err)
	assert.Equal(fcz[8].Height(), fc_.Height())

	// Add access to all full commits via untrusted source.
	for i := 0; i < count; i++ {
		err := source.SaveFullCommit(fcz[i])
		require.NoError(err)
	}

	// Try to fetch an unknown commit from the past.
	fc_, err = trust.LatestFullCommit(chainID, fcz[2].Height(), fcz[3].Height())
	require.NoError(err, "%+v", err)
	assert.Equal(fcz[2].Height(), fc_.Height())
	assert.Equal(fcz[8].Height(), vp.LastTrustedHeight())
	// TODO This should work for as long as the trust period hasn't passed for
	// fcz[2].  Write a test that tries to retroactively fetchees fcz[3] from
	// source.  Initially it should fail since source doesn't have it, but it
	// should succeed once source is provided it.

	// Try to fetch the latest known commit.
	fc_, err = trust.LatestFullCommit(chainID, 0, fcz[9].Height())
	require.NoError(err, "%+v", err)
	assert.Equal(fcz[9].Height(), fc_.Height())
	assert.Equal(fcz[9].Height(), vp.LastTrustedHeight())
}

func TestConcurrentProvider(t *testing.T) {
	_, require := assert.New(t), require.New(t)
	trust := lite.NewDBProvider("trust", dbm.NewMemDB()).SetLimit(10)
	source := lite.NewDBProvider("source", dbm.NewMemDB())

	// Set up the validators to generate test blocks.
	var vote int64 = 10
	keys := pks.GenPrivKeys(5)
	nkeys := keys.Extend(1)

	// Construct a bunch of commits, each with one more height than the last.
	chainID := "inquiry-test"
	count := 10
	consHash := []byte("special-params")
	fcz := make([]lite.FullCommit, count)
	for i := 0; i < count; i++ {
		vals := keys.ToValidators(vote, 0)
		nextVals := nkeys.ToValidators(vote, 0)
		h := int64(1 + i)
		appHash := []byte(fmt.Sprintf("h=%d", h))
		resHash := []byte(fmt.Sprintf("res=%d", h))
		signedHeader := keys.GenSignedHeader(
			chainID, h, nil,
			vals, nextVals,
			appHash, consHash, resHash, 0, len(keys))
		fcz[i] = lite.NewFullCommit(signedHeader, vals, nextVals)
		// Extend the keys by 1 each time.
		keys = nkeys
		nkeys = nkeys.Extend(1)
	}

	// Initialize a Verifier with the initial state.
	err := trust.SaveFullCommit(fcz[0])
	require.NoError(err)
	vp, _ := NewProvider(chainID, trust, source)
	vp.SetLogger(log.TestingLogger())
	cp := lite.NewConcurrentProvider(vp)

	err = source.SaveFullCommit(fcz[7])
	require.Nil(err, "%+v", err)
	err = source.SaveFullCommit(fcz[8])
	require.NoError(err, "%+v", err)
	// sh := fcz[8].SignedHeader unused

	var wg sync.WaitGroup
	count = 100
	errList := make([]error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			errList[index] = cp.UpdateToHeight(chainID, fcz[8].SignedHeader.Height)
			defer wg.Done()
		}(i)
	}
	wg.Wait()
	for _, err := range errList {
		require.NoError(err)
	}
}
