// Copyright 2017 Tendermint. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package trust

import (
	"encoding/json"
	"sync"
	"time"

	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
)

const defaultStorePeriodicSaveInterval = 1 * time.Minute

var trustMetricKey = []byte("trustMetricStore")

// TrustMetricStore - Manages all trust metrics for peers
type TrustMetricStore struct {
	cmn.BaseService

	// Maps a Peer.Key to that peer's TrustMetric
	reactorPeerMetrics map[string]map[string]*TrustMetric

	// Mutex that protects the map and history data file
	mtx sync.Mutex

	// The db where peer trust metric history data will be stored
	db dbm.DB

	// This configuration will be used when creating new TrustMetrics
	config TrustMetricConfig
}

// NewTrustMetricStore returns a store that saves data to the DB
// and uses the config when creating new trust metrics.
// Use Start to to initialize the trust metric store
func NewTrustMetricStore(db dbm.DB, tmc TrustMetricConfig) *TrustMetricStore {
	tms := &TrustMetricStore{
		reactorPeerMetrics: make(map[string]map[string]*TrustMetric),
		db:                 db,
		config:             tmc,
	}

	tms.BaseService = *cmn.NewBaseService(nil, "TrustMetricStore", tms)
	return tms
}

// OnStart implements Service
func (tms *TrustMetricStore) OnStart() error {
	if err := tms.BaseService.OnStart(); err != nil {
		return err
	}

	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	tms.loadFromDB()
	go tms.saveRoutine()
	return nil
}

// OnStop implements Service
func (tms *TrustMetricStore) OnStop() {
	tms.BaseService.OnStop()

	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	// Stop all trust metric go-routines
	for _, peers := range tms.reactorPeerMetrics {
		for _, tm := range peers {
			tm.Stop()
		}
	}

	// Make the final trust history data save
	tms.saveToDB()
}

// Size returns the number of entries in the trust metric store
func (tms *TrustMetricStore) Size() int {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	return tms.size()
}

// AddPeerTrustMetric takes an existing trust metric and associates it with a peer and reactor ID.
// The caller is expected to call Start on the TrustMetric being added
func (tms *TrustMetricStore) AddPeerTrustMetric(peer, reactor string, tm *TrustMetric) {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	tms.addPeerTrustMetric(peer, reactor, tm)
}

// GetPeerTrustMetric returns a trust metric by peer key
func (tms *TrustMetricStore) GetPeerTrustMetric(peer, reactor string) *TrustMetric {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	// If the metric is in the store, return it to the caller
	if peers, ok := tms.reactorPeerMetrics[reactor]; ok {
		if tm, found := peers[peer]; found {
			return tm
		}
	}
	// Otherwise, initialize a new metric for the caller to use
	tm := NewMetricWithConfig(tms.config)
	tms.addPeerTrustMetric(peer, reactor, tm)
	tm.Start()
	return tm
}

// PeerDisconnected pauses the trust metric associated with the peer identified by the key
func (tms *TrustMetricStore) PeerDisconnected(peer, reactor string) {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	// If the peer that disconnected had metrics, pause them
	if peers, ok := tms.reactorPeerMetrics[reactor]; ok {
		if tm, found := peers[peer]; found {
			tm.Pause()
		}
	}
}

// Saves the history data for all peers to the store DB.
// This public method acquires the trust metric store lock
func (tms *TrustMetricStore) SaveToDB() {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	tms.saveToDB()
}

/* Private methods */

// addPeerTrustMetric takes an existing trust metric and associates it with a peer and reactor ID.
// This version of the method assumes the mutex has already been acquired
func (tms *TrustMetricStore) addPeerTrustMetric(peer, reactor string, tm *TrustMetric) {
	if peer == "" || reactor == "" || tm == nil {
		return
	}
	// Check if we have a peer/metric map for this reactor yet
	if _, ok := tms.reactorPeerMetrics[reactor]; !ok {
		tms.reactorPeerMetrics[reactor] = make(map[string]*TrustMetric)
	}
	tms.reactorPeerMetrics[reactor][peer] = tm
}

// size returns the number of entries in the store without acquiring the mutex
func (tms *TrustMetricStore) size() int {
	var size int

	for _, peers := range tms.reactorPeerMetrics {
		size += len(peers)
	}
	return size
}

/* Loading & Saving */
/* Both loadFromDB and savetoDB assume the mutex has been acquired */

// Loads the history data for all peers from the store DB
// cmn.Panics if file is corrupt
func (tms *TrustMetricStore) loadFromDB() bool {
	// Obtain the history data we have so far
	bytes := tms.db.Get(trustMetricKey)
	if bytes == nil {
		return false
	}

	history := make(map[string]map[string]MetricHistoryJSON)
	err := json.Unmarshal(bytes, &history)
	if err != nil {
		cmn.PanicCrisis(cmn.Fmt("Could not unmarshal Trust Metric Store DB data: %v", err))
	}

	// If history data exists in the file,
	// load it into trust metric
	for reactor, peers := range history {
		for peer, data := range peers {
			tm := NewMetricWithConfig(tms.config)

			tm.Start()
			tm.Init(data)
			// Load the peer trust metric into the store
			tms.addPeerTrustMetric(peer, reactor, tm)

		}
	}
	return true
}

// Saves the history data for all peers to the store DB
func (tms *TrustMetricStore) saveToDB() {
	tms.Logger.Debug("Saving TrustHistory to DB", "size", tms.size())

	history := make(map[string]map[string]MetricHistoryJSON)

	for reactor, peers := range tms.reactorPeerMetrics {
		history[reactor] = make(map[string]MetricHistoryJSON)

		for peer, tm := range peers {
			// Add an entry for the metric
			history[reactor][peer] = tm.HistoryJSON()
		}
	}

	// Write all the data back to the DB
	bytes, err := json.Marshal(history)
	if err != nil {
		tms.Logger.Error("Failed to encode the TrustHistory", "err", err)
		return
	}
	tms.db.SetSync(trustMetricKey, bytes)
}

// Periodically saves the trust history data to the DB
func (tms *TrustMetricStore) saveRoutine() {
	t := time.NewTicker(defaultStorePeriodicSaveInterval)
	defer t.Stop()
loop:
	for {
		select {
		case <-t.C:
			tms.SaveToDB()
		case <-tms.Quit:
			break loop
		}
	}
}
