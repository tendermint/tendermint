// Copyright 2017 Tendermint. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package trust

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
)

func TestTrustMetricStoreSaveLoad(t *testing.T) {
	dir, err := os.MkdirTemp("", "trust_test")
	require.NoError(t, err)
	defer os.Remove(dir)

	historyDB, err := dbm.NewDB("trusthistory", "goleveldb", dir)
	require.NoError(t, err)

	// 0 peers saved
	store := NewTrustMetricStore(historyDB, DefaultConfig())
	store.SetLogger(log.TestingLogger())
	store.saveToDB()
	// Load the data from the file
	store = NewTrustMetricStore(historyDB, DefaultConfig())
	store.SetLogger(log.TestingLogger())
	err = store.Start()
	require.NoError(t, err)
	// Make sure we still have 0 entries
	assert.Zero(t, store.Size())

	// 100 TestTickers
	var tt []*TestTicker
	for i := 0; i < 100; i++ {
		// The TestTicker will provide manual control over
		// the passing of time within the metric
		tt = append(tt, NewTestTicker())
	}
	// 100 peers
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("peer_%d", i)
		tm := NewMetric()

		tm.SetTicker(tt[i])
		err = tm.Start()
		require.NoError(t, err)
		store.AddPeerTrustMetric(key, tm)

		tm.BadEvents(10)
		tm.GoodEvents(1)
	}
	// Check that we have 100 entries and save
	assert.Equal(t, 100, store.Size())
	// Give the 100 metrics time to process the history data
	for i := 0; i < 100; i++ {
		tt[i].NextTick()
		tt[i].NextTick()
	}
	// Stop all the trust metrics and save
	err = store.Stop()
	require.NoError(t, err)

	// Load the data from the DB
	store = NewTrustMetricStore(historyDB, DefaultConfig())
	store.SetLogger(log.TestingLogger())
	err = store.Start()
	require.NoError(t, err)

	// Check that we still have 100 peers with imperfect trust values
	assert.Equal(t, 100, store.Size())
	for _, tm := range store.peerMetrics {
		assert.NotEqual(t, 1.0, tm.TrustValue())
	}

	err = store.Stop()
	require.NoError(t, err)
}

func TestTrustMetricStoreConfig(t *testing.T) {
	historyDB, err := dbm.NewDB("", "memdb", "")
	require.NoError(t, err)

	config := MetricConfig{
		ProportionalWeight: 0.5,
		IntegralWeight:     0.5,
	}

	// Create a store with custom config
	store := NewTrustMetricStore(historyDB, config)
	store.SetLogger(log.TestingLogger())
	err = store.Start()
	require.NoError(t, err)

	// Have the store make us a metric with the config
	tm := store.GetPeerTrustMetric("TestKey")

	// Check that the options made it to the metric
	assert.Equal(t, 0.5, tm.proportionalWeight)
	assert.Equal(t, 0.5, tm.integralWeight)
	err = store.Stop()
	require.NoError(t, err)
}

func TestTrustMetricStoreLookup(t *testing.T) {
	historyDB, err := dbm.NewDB("", "memdb", "")
	require.NoError(t, err)

	store := NewTrustMetricStore(historyDB, DefaultConfig())
	store.SetLogger(log.TestingLogger())
	err = store.Start()
	require.NoError(t, err)

	// Create 100 peers in the trust metric store
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("peer_%d", i)
		store.GetPeerTrustMetric(key)

		// Check that the trust metric was successfully entered
		ktm := store.peerMetrics[key]
		assert.NotNil(t, ktm, "Expected to find TrustMetric %s but wasn't there.", key)
	}

	err = store.Stop()
	require.NoError(t, err)
}

func TestTrustMetricStorePeerScore(t *testing.T) {
	historyDB, err := dbm.NewDB("", "memdb", "")
	require.NoError(t, err)

	store := NewTrustMetricStore(historyDB, DefaultConfig())
	store.SetLogger(log.TestingLogger())
	err = store.Start()
	require.NoError(t, err)

	key := "TestKey"
	tm := store.GetPeerTrustMetric(key)

	// This peer is innocent so far
	first := tm.TrustScore()
	assert.Equal(t, 100, first)

	// Add some undesirable events and disconnect
	tm.BadEvents(1)
	first = tm.TrustScore()
	assert.NotEqual(t, 100, first)
	tm.BadEvents(10)
	second := tm.TrustScore()

	if second > first {
		t.Errorf("a greater number of bad events should lower the trust score")
	}
	store.PeerDisconnected(key)

	// We will remember our experiences with this peer
	tm = store.GetPeerTrustMetric(key)
	assert.NotEqual(t, 100, tm.TrustScore())
	err = store.Stop()
	require.NoError(t, err)
}
