// Copyright 2017 Tendermint. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package trust

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"
)

func TestTrustMetricStoreSaveLoad(t *testing.T) {
	dir, err := ioutil.TempDir("", "trust_test")
	if err != nil {
		panic(err)
	}
	defer os.Remove(dir)

	historyDB := dbm.NewDB("trusthistory", "goleveldb", dir)

	// 0 peers saved
	store := NewTrustMetricStore(historyDB, DefaultConfig())
	store.SetLogger(log.TestingLogger())
	store.saveToDB()
	// Load the data from the file
	store = NewTrustMetricStore(historyDB, DefaultConfig())
	store.SetLogger(log.TestingLogger())
	store.Start()
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
		tm.Start()
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
	store.Stop()

	// Load the data from the DB
	store = NewTrustMetricStore(historyDB, DefaultConfig())
	store.SetLogger(log.TestingLogger())
	store.Start()

	// Check that we still have 100 peers with imperfect trust values
	assert.Equal(t, 100, store.Size())
	for _, tm := range store.peerMetrics {
		assert.NotEqual(t, 1.0, tm.TrustValue())
	}

	store.Stop()
}

func TestTrustMetricStoreConfig(t *testing.T) {
	historyDB := dbm.NewDB("", "memdb", "")

	config := TrustMetricConfig{
		ProportionalWeight: 0.5,
		IntegralWeight:     0.5,
	}

	// Create a store with custom config
	store := NewTrustMetricStore(historyDB, config)
	store.SetLogger(log.TestingLogger())
	store.Start()

	// Have the store make us a metric with the config
	tm := store.GetPeerTrustMetric("TestKey")

	// Check that the options made it to the metric
	assert.Equal(t, 0.5, tm.proportionalWeight)
	assert.Equal(t, 0.5, tm.integralWeight)
	store.Stop()
}

func TestTrustMetricStoreLookup(t *testing.T) {
	historyDB := dbm.NewDB("", "memdb", "")

	store := NewTrustMetricStore(historyDB, DefaultConfig())
	store.SetLogger(log.TestingLogger())
	store.Start()

	// Create 100 peers in the trust metric store
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("peer_%d", i)
		store.GetPeerTrustMetric(key)

		// Check that the trust metric was successfully entered
		ktm := store.peerMetrics[key]
		assert.NotNil(t, ktm, "Expected to find TrustMetric %s but wasn't there.", key)
	}

	store.Stop()
}

func TestTrustMetricStorePeerScore(t *testing.T) {
	historyDB := dbm.NewDB("", "memdb", "")

	store := NewTrustMetricStore(historyDB, DefaultConfig())
	store.SetLogger(log.TestingLogger())
	store.Start()

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
		t.Errorf("A greater number of bad events should lower the trust score")
	}
	store.PeerDisconnected(key)

	// We will remember our experiences with this peer
	tm = store.GetPeerTrustMetric(key)
	assert.NotEqual(t, 100, tm.TrustScore())
	store.Stop()
}
