// Copyright 2017 Tendermint. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package trust

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"
)

func getTempDir(prefix string) string {
	dir, err := ioutil.TempDir("", prefix)
	if err != nil {
		panic(err)
	}
	return dir
}

func TestTrustMetricStoreSaveLoad(t *testing.T) {
	dir := getTempDir("trustMetricStoreTest")
	defer os.Remove(dir)

	historyDB := dbm.NewDB("trusthistory", "goleveldb", dir)

	config := TrustMetricConfig{
		TrackingWindow: 5 * time.Minute,
		IntervalLength: 50 * time.Millisecond,
	}

	// 0 peers saved
	store := NewTrustMetricStore(historyDB, config)
	store.SetLogger(log.TestingLogger())
	store.saveToDB()
	// Load the data from the file
	store = NewTrustMetricStore(historyDB, config)
	store.SetLogger(log.TestingLogger())
	store.loadFromDB()
	// Make sure we still have 0 entries
	assert.Zero(t, store.Size())

	// 100 peers
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("peer_%d", i)
		tm := store.GetPeerTrustMetric(key)

		tm.AddBadEvents(10)
		tm.GoodEvent()
	}

	// Check that we have 100 entries and save
	assert.Equal(t, 100, store.Size())
	// Give the metrics time to process the history data
	time.Sleep(1 * time.Second)

	// Stop all the trust metrics and save
	for _, tm := range store.peerMetrics {
		tm.Stop()
	}
	store.saveToDB()

	// Load the data from the DB
	store = NewTrustMetricStore(historyDB, config)
	store.SetLogger(log.TestingLogger())
	store.loadFromDB()

	// Check that we still have 100 peers with imperfect trust values
	assert.Equal(t, 100, store.Size())
	for _, tm := range store.peerMetrics {
		assert.NotEqual(t, 1.0, tm.TrustValue())
	}

	// Stop all the trust metrics
	for _, tm := range store.peerMetrics {
		tm.Stop()
	}
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

	// Have the store make us a metric with the config
	tm := store.GetPeerTrustMetric("TestKey")

	// Check that the options made it to the metric
	assert.Equal(t, 0.5, tm.proportionalWeight)
	assert.Equal(t, 0.5, tm.integralWeight)
	tm.Stop()
}

func TestTrustMetricStoreLookup(t *testing.T) {
	historyDB := dbm.NewDB("", "memdb", "")

	store := NewTrustMetricStore(historyDB, DefaultConfig())
	store.SetLogger(log.TestingLogger())

	// Create 100 peers in the trust metric store
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("peer_%d", i)
		store.GetPeerTrustMetric(key)

		// Check that the trust metric was successfully entered
		ktm := store.peerMetrics[key]
		assert.NotNil(t, ktm, "Expected to find TrustMetric %s but wasn't there.", key)
	}

	// Stop all the trust metrics
	for _, tm := range store.peerMetrics {
		tm.Stop()
	}
}

func TestTrustMetricStorePeerScore(t *testing.T) {
	historyDB := dbm.NewDB("", "memdb", "")

	store := NewTrustMetricStore(historyDB, DefaultConfig())
	store.SetLogger(log.TestingLogger())

	key := "TestKey"
	tm := store.GetPeerTrustMetric(key)

	// This peer is innocent so far
	first := tm.TrustScore()
	assert.Equal(t, 100, first)

	// Add some undesirable events and disconnect
	tm.BadEvent()
	first = tm.TrustScore()
	assert.NotEqual(t, 100, first)
	tm.AddBadEvents(10)
	second := tm.TrustScore()

	if second > first {
		t.Errorf("A greater number of bad events should lower the trust score")
	}
	store.PeerDisconnected(key)

	// We will remember our experiences with this peer
	tm = store.GetPeerTrustMetric(key)
	assert.NotEqual(t, 100, tm.TrustScore())
	tm.Stop()
}

func TestTrustMetricScores(t *testing.T) {
	tm := NewMetric()

	// Perfect score
	tm.GoodEvent()
	score := tm.TrustScore()
	assert.Equal(t, 100, score)

	// Less than perfect score
	tm.AddBadEvents(10)
	score = tm.TrustScore()
	assert.NotEqual(t, 100, score)
	tm.Stop()
}

func TestTrustMetricConfig(t *testing.T) {
	// 7 days
	window := time.Minute * 60 * 24 * 7
	config := TrustMetricConfig{
		TrackingWindow: window,
		IntervalLength: 2 * time.Minute,
	}

	tm := NewMetricWithConfig(config)

	// The max time intervals should be the TrackingWindow / IntervalLen
	assert.Equal(t, int(config.TrackingWindow/config.IntervalLength), tm.maxIntervals)

	dc := DefaultConfig()
	// These weights should still be the default values
	assert.Equal(t, dc.ProportionalWeight, tm.proportionalWeight)
	assert.Equal(t, dc.IntegralWeight, tm.integralWeight)
	tm.Stop()

	config.ProportionalWeight = 0.3
	config.IntegralWeight = 0.7
	tm = NewMetricWithConfig(config)

	// These weights should be equal to our custom values
	assert.Equal(t, config.ProportionalWeight, tm.proportionalWeight)
	assert.Equal(t, config.IntegralWeight, tm.integralWeight)
	tm.Stop()
}

func TestTrustMetricStopPause(t *testing.T) {
	// Cause time intervals to pass quickly
	config := TrustMetricConfig{
		TrackingWindow: 5 * time.Minute,
		IntervalLength: 10 * time.Millisecond,
	}

	tm := NewMetricWithConfig(config)

	// Allow some time intervals to pass and pause
	time.Sleep(50 * time.Millisecond)
	tm.Pause()
	// Give the pause some time to take place
	time.Sleep(10 * time.Millisecond)

	first := tm.numIntervals
	// Allow more time to pass and check the intervals are unchanged
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, first, tm.numIntervals)

	// Get the trust metric activated again
	tm.AddGoodEvents(5)
	// Allow some time intervals to pass and stop
	time.Sleep(50 * time.Millisecond)
	tm.Stop()
	// Give the stop some time to take place
	time.Sleep(10 * time.Millisecond)

	second := tm.numIntervals
	// Allow more time to pass and check the intervals are unchanged
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, second, tm.numIntervals)

	if first >= second {
		t.Fatalf("numIntervals should always increase or stay the same over time")
	}
}
