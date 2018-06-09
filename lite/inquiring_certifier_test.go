package lite

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tmlibs/db"
)

func TestInquirerValidPath(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	trust := NewDBProvider(dbm.NewMemDB())
	source := NewDBProvider(db.NewMemDB())

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
		nvals := nkeys.ToValidators(vote, 0)
		// Extend the keys by 1 each time.
		keys = nkeys
		nkeys = nkeys.Extend(1)
		h := int64(20 + 10*i)
		appHash := []byte(fmt.Sprintf("h=%d", h))
		fcz[i] = keys.GenFullCommit(
			chainID, h, nil,
			vals, nvals,
			appHash, consHash, resHash, 0, len(keys))
	}

	// Initialize a certifier with the initial state.
	cert, err := NewInquiringCertifier(chainID, fcz[0], trust, source)
	require.Nil(err)

	// This should fail validation:
	commit := fcz[count-1].Commit
	err = cert.Certify(commit)
	require.NotNil(err)

	// Adding a few commits in the middle should be insufficient.
	for i := 10; i < 13; i++ {
		err := source.SaveFullCommit(fcz[i])
		require.Nil(err)
	}
	err = cert.Certify(commit)
	assert.NotNil(err)

	// With more info, we succeed.
	for i := 0; i < count; i++ {
		err := source.SaveFullCommit(fcz[i])
		require.Nil(err)
	}
	err = cert.Certify(commit)
	assert.Nil(err, "%+v", err)
}

func TestInquirerVerifyHistorical(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	trust := NewDBProvider(dbm.NewMemDB())
	source := NewDBProvider(dbm.NewMemDB())

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
		nvals := nkeys.ToValidators(vote, 0)
		// Extend the keys by 1 each time.
		keys = nkeys
		nkeys = nkeys.Extend(1)
		h := int64(20 + 10*i)
		appHash := []byte(fmt.Sprintf("h=%d", h))
		resHash := []byte(fmt.Sprintf("res=%d", h))
		fcz[i] = keys.GenFullCommit(
			chainID, h, nil,
			vals, nvals,
			appHash, consHash, resHash, 0, len(keys))
	}

	// Initialize a certifier with the initial state.
	cert, _ := NewInquiringCertifier(chainID, commits[0], trust, source)

	// Store a few commits as trust.
	for _, i := range []int{2, 5} {
		trust.SaveFullCommit(fcz[i])
	}

	// See if we can jump forward using trusted commits.
	err := source.SaveFullCommit(commits[7])
	require.Nil(err, "%+v", err)
	commit := fcz[7].Commit
	err = cert.Certify(commit)
	require.Nil(err, "%+v", err)
	assert.Equal(commit.Height(), cert.LastTrustedHeight())

	// Add access to all commits via untrusted source.
	for i := 0; i < count; i++ {
		err := source.SaveFullCommit(commits[i])
		require.Nil(err)
	}

	// Try to check an unknown seed in the past.
	commit = fcz[3].Commit
	err = cert.Certify(commit)
	require.Nil(err, "%+v", err)
	assert.Equal(commit.Height(), cert.LastTrustedHeight())

	// Jump all the way forward again.
	commit = fcz[count-1].Commit
	err = cert.Certify(commit)
	require.Nil(err, "%+v", err)
	assert.Equal(commit.Height(), cert.LastTrustedHeight())
}
