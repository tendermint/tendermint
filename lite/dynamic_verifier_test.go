package lite

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tendermint/libs/db"
	log "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

func TestInquirerValidPath(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	trust := NewDBProvider("trust", dbm.NewMemDB())
	source := NewDBProvider("source", dbm.NewMemDB())

	// Set up the validators to generate test blocks.
	var vote int64 = 10
	keys := genPrivKeys(5)
	nkeys := keys.Extend(1)

	// Construct a bunch of commits, each with one more height than the last.
	chainID := "inquiry-test"
	consHash := []byte("params")
	resHash := []byte("results")
	count := 50
	fcz := make([]FullCommit, count)
	for i := 0; i < count; i++ {
		vals := keys.ToValidators(vote, 0)
		nextVals := nkeys.ToValidators(vote, 0)
		h := int64(1 + i)
		appHash := []byte(fmt.Sprintf("h=%d", h))
		fcz[i] = keys.GenFullCommit(
			chainID, h, nil,
			vals, nextVals,
			appHash, consHash, resHash, 0, len(keys))
		// Extend the keys by 1 each time.
		keys = nkeys
		nkeys = nkeys.Extend(1)
	}

	// Initialize a Verifier with the initial state.
	err := trust.SaveFullCommit(fcz[0])
	require.Nil(err)
	cert := NewDynamicVerifier(chainID, trust, source)
	cert.SetLogger(log.TestingLogger())

	// This should fail validation:
	sh := fcz[count-1].SignedHeader
	err = cert.Verify(sh)
	require.NotNil(err)

	// Adding a few commits in the middle should be insufficient.
	for i := 10; i < 13; i++ {
		err := source.SaveFullCommit(fcz[i])
		require.Nil(err)
	}
	err = cert.Verify(sh)
	assert.NotNil(err)

	// With more info, we succeed.
	for i := 0; i < count; i++ {
		err := source.SaveFullCommit(fcz[i])
		require.Nil(err)
	}
	err = cert.Verify(sh)
	assert.Nil(err, "%+v", err)
}

func TestDynamicVerify(t *testing.T) {
	trust := NewDBProvider("trust", dbm.NewMemDB())
	source := NewDBProvider("source", dbm.NewMemDB())

	// 10 commits with one valset, 1 to change,
	// 10 commits with the next one
	n1, n2 := 10, 10
	nCommits := n1 + n2 + 1
	maxHeight := int64(nCommits)
	fcz := make([]FullCommit, nCommits)

	// gen the 2 val sets
	chainID := "dynamic-verifier"
	power := int64(10)
	keys1 := genPrivKeys(5)
	vals1 := keys1.ToValidators(power, 0)
	keys2 := genPrivKeys(5)
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
	require.Nil(t, err)
	ver := NewDynamicVerifier(chainID, trust, source)
	ver.SetLogger(log.TestingLogger())

	// fetch the latest from the source
	latestFC, err := source.LatestFullCommit(chainID, 1, maxHeight)
	require.NoError(t, err)

	// try to update to the latest
	err = ver.Verify(latestFC.SignedHeader)
	require.NoError(t, err)

}

func makeFullCommit(height int64, keys privKeys, vals, nextVals *types.ValidatorSet, chainID string) FullCommit {
	height += 1
	consHash := []byte("special-params")
	appHash := []byte(fmt.Sprintf("h=%d", height))
	resHash := []byte(fmt.Sprintf("res=%d", height))
	return keys.GenFullCommit(
		chainID, height, nil,
		vals, nextVals,
		appHash, consHash, resHash, 0, len(keys))
}

func TestInquirerVerifyHistorical(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	trust := NewDBProvider("trust", dbm.NewMemDB())
	source := NewDBProvider("source", dbm.NewMemDB())

	// Set up the validators to generate test blocks.
	var vote int64 = 10
	keys := genPrivKeys(5)
	nkeys := keys.Extend(1)

	// Construct a bunch of commits, each with one more height than the last.
	chainID := "inquiry-test"
	count := 10
	consHash := []byte("special-params")
	fcz := make([]FullCommit, count)
	for i := 0; i < count; i++ {
		vals := keys.ToValidators(vote, 0)
		nextVals := nkeys.ToValidators(vote, 0)
		h := int64(1 + i)
		appHash := []byte(fmt.Sprintf("h=%d", h))
		resHash := []byte(fmt.Sprintf("res=%d", h))
		fcz[i] = keys.GenFullCommit(
			chainID, h, nil,
			vals, nextVals,
			appHash, consHash, resHash, 0, len(keys))
		// Extend the keys by 1 each time.
		keys = nkeys
		nkeys = nkeys.Extend(1)
	}

	// Initialize a Verifier with the initial state.
	err := trust.SaveFullCommit(fcz[0])
	require.Nil(err)
	cert := NewDynamicVerifier(chainID, trust, source)
	cert.SetLogger(log.TestingLogger())

	// Store a few full commits as trust.
	for _, i := range []int{2, 5} {
		trust.SaveFullCommit(fcz[i])
	}

	// See if we can jump forward using trusted full commits.
	// Souce doesn't have fcz[9] so cert.LastTrustedHeight wont' change.
	err = source.SaveFullCommit(fcz[7])
	require.Nil(err, "%+v", err)
	sh := fcz[8].SignedHeader
	err = cert.Verify(sh)
	require.Nil(err, "%+v", err)
	assert.Equal(fcz[7].Height(), cert.LastTrustedHeight())
	fc_, err := trust.LatestFullCommit(chainID, fcz[8].Height(), fcz[8].Height())
	require.NotNil(err, "%+v", err)
	assert.Equal(fc_, (FullCommit{}))

	// With fcz[9] Verify will update last trusted height.
	err = source.SaveFullCommit(fcz[9])
	require.Nil(err, "%+v", err)
	sh = fcz[8].SignedHeader
	err = cert.Verify(sh)
	require.Nil(err, "%+v", err)
	assert.Equal(fcz[8].Height(), cert.LastTrustedHeight())
	fc_, err = trust.LatestFullCommit(chainID, fcz[8].Height(), fcz[8].Height())
	require.Nil(err, "%+v", err)
	assert.Equal(fc_.Height(), fcz[8].Height())

	// Add access to all full commits via untrusted source.
	for i := 0; i < count; i++ {
		err := source.SaveFullCommit(fcz[i])
		require.Nil(err)
	}

	// Try to check an unknown seed in the past.
	sh = fcz[3].SignedHeader
	err = cert.Verify(sh)
	require.Nil(err, "%+v", err)
	assert.Equal(fcz[8].Height(), cert.LastTrustedHeight())

	// Jump all the way forward again.
	sh = fcz[count-1].SignedHeader
	err = cert.Verify(sh)
	require.Nil(err, "%+v", err)
	assert.Equal(fcz[9].Height(), cert.LastTrustedHeight())
}

func TestConcurrencyInquirerVerify(t *testing.T) {
	_, require := assert.New(t), require.New(t)
	trust := NewDBProvider("trust", dbm.NewMemDB()).SetLimit(10)
	source := NewDBProvider("source", dbm.NewMemDB())

	// Set up the validators to generate test blocks.
	var vote int64 = 10
	keys := genPrivKeys(5)
	nkeys := keys.Extend(1)

	// Construct a bunch of commits, each with one more height than the last.
	chainID := "inquiry-test"
	count := 10
	consHash := []byte("special-params")
	fcz := make([]FullCommit, count)
	for i := 0; i < count; i++ {
		vals := keys.ToValidators(vote, 0)
		nextVals := nkeys.ToValidators(vote, 0)
		h := int64(1 + i)
		appHash := []byte(fmt.Sprintf("h=%d", h))
		resHash := []byte(fmt.Sprintf("res=%d", h))
		fcz[i] = keys.GenFullCommit(
			chainID, h, nil,
			vals, nextVals,
			appHash, consHash, resHash, 0, len(keys))
		// Extend the keys by 1 each time.
		keys = nkeys
		nkeys = nkeys.Extend(1)
	}

	// Initialize a Verifier with the initial state.
	err := trust.SaveFullCommit(fcz[0])
	require.Nil(err)
	cert := NewDynamicVerifier(chainID, trust, source)
	cert.SetLogger(log.TestingLogger())

	err = source.SaveFullCommit(fcz[7])
	err = source.SaveFullCommit(fcz[8])
	require.Nil(err, "%+v", err)
	sh := fcz[8].SignedHeader

	var wg sync.WaitGroup
	count = 100
	errList := make([]error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			errList[index] = cert.Verify(sh)
			defer wg.Done()
		}(i)
	}
	wg.Wait()
	for _, err := range errList {
		require.Nil(err)
	}
}
