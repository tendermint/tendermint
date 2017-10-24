package certifiers_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/certifiers"
)

func TestInquirerValidPath(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	trust := certifiers.NewMemStoreProvider()
	source := certifiers.NewMemStoreProvider()

	// set up the validators to generate test blocks
	var vote int64 = 10
	keys := certifiers.GenValKeys(5)
	vals := keys.ToValidators(vote, 0)

	// construct a bunch of commits, each with one more height than the last
	chainID := "inquiry-test"
	count := 50
	commits := make([]certifiers.FullCommit, count)
	for i := 0; i < count; i++ {
		// extend the keys by 1 each time
		keys = keys.Extend(1)
		vals = keys.ToValidators(vote, 0)
		h := 20 + 10*i
		appHash := []byte(fmt.Sprintf("h=%d", h))
		commits[i] = keys.GenFullCommit(chainID, h, nil, vals, appHash, 0, len(keys))
	}

	// initialize a certifier with the initial state
	cert := certifiers.NewInquiring(chainID, commits[0], trust, source)

	// this should fail validation....
	commit := commits[count-1].Commit
	err := cert.Certify(commit)
	require.NotNil(err)

	// add a few seed in the middle should be insufficient
	for i := 10; i < 13; i++ {
		err := source.StoreCommit(commits[i])
		require.Nil(err)
	}
	err = cert.Certify(commit)
	assert.NotNil(err)

	// with more info, we succeed
	for i := 0; i < count; i++ {
		err := source.StoreCommit(commits[i])
		require.Nil(err)
	}
	err = cert.Certify(commit)
	assert.Nil(err, "%+v", err)
}

func TestInquirerMinimalPath(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	trust := certifiers.NewMemStoreProvider()
	source := certifiers.NewMemStoreProvider()

	// set up the validators to generate test blocks
	var vote int64 = 10
	keys := certifiers.GenValKeys(5)
	vals := keys.ToValidators(vote, 0)

	// construct a bunch of commits, each with one more height than the last
	chainID := "minimal-path"
	count := 12
	commits := make([]certifiers.FullCommit, count)
	for i := 0; i < count; i++ {
		// extend the validators, so we are just below 2/3
		keys = keys.Extend(len(keys)/2 - 1)
		vals = keys.ToValidators(vote, 0)
		h := 5 + 10*i
		appHash := []byte(fmt.Sprintf("h=%d", h))
		commits[i] = keys.GenFullCommit(chainID, h, nil, vals, appHash, 0, len(keys))
	}

	// initialize a certifier with the initial state
	cert := certifiers.NewInquiring(chainID, commits[0], trust, source)

	// this should fail validation....
	commit := commits[count-1].Commit
	err := cert.Certify(commit)
	require.NotNil(err)

	// add a few seed in the middle should be insufficient
	for i := 5; i < 8; i++ {
		err := source.StoreCommit(commits[i])
		require.Nil(err)
	}
	err = cert.Certify(commit)
	assert.NotNil(err)

	// with more info, we succeed
	for i := 0; i < count; i++ {
		err := source.StoreCommit(commits[i])
		require.Nil(err)
	}
	err = cert.Certify(commit)
	assert.Nil(err, "%+v", err)
}

func TestInquirerVerifyHistorical(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	trust := certifiers.NewMemStoreProvider()
	source := certifiers.NewMemStoreProvider()

	// set up the validators to generate test blocks
	var vote int64 = 10
	keys := certifiers.GenValKeys(5)
	vals := keys.ToValidators(vote, 0)

	// construct a bunch of commits, each with one more height than the last
	chainID := "inquiry-test"
	count := 10
	commits := make([]certifiers.FullCommit, count)
	for i := 0; i < count; i++ {
		// extend the keys by 1 each time
		keys = keys.Extend(1)
		vals = keys.ToValidators(vote, 0)
		h := 20 + 10*i
		appHash := []byte(fmt.Sprintf("h=%d", h))
		commits[i] = keys.GenFullCommit(chainID, h, nil, vals, appHash, 0, len(keys))
	}

	// initialize a certifier with the initial state
	cert := certifiers.NewInquiring(chainID, commits[0], trust, source)

	// store a few commits as trust
	for _, i := range []int{2, 5} {
		trust.StoreCommit(commits[i])
	}

	// let's see if we can jump forward using trusted commits
	err := source.StoreCommit(commits[7])
	require.Nil(err, "%+v", err)
	check := commits[7].Commit
	err = cert.Certify(check)
	require.Nil(err, "%+v", err)
	assert.Equal(check.Height(), cert.LastHeight())

	// add access to all commits via untrusted source
	for i := 0; i < count; i++ {
		err := source.StoreCommit(commits[i])
		require.Nil(err)
	}

	// try to check an unknown seed in the past
	mid := commits[3].Commit
	err = cert.Certify(mid)
	require.Nil(err, "%+v", err)
	assert.Equal(mid.Height(), cert.LastHeight())

	// and jump all the way forward again
	end := commits[count-1].Commit
	err = cert.Certify(end)
	require.Nil(err, "%+v", err)
	assert.Equal(end.Height(), cert.LastHeight())
}
