// Copyright 2017 Tendermint. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package trust

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tm-db"
)

const defaultStorePeriodicSaveInterval = 1 * time.Minute

var trustMetricKey = []byte("trustMetricStore")

// TrustMetricStore - Manages all trust metrics for peers
type TrustMetricStore struct {
	cmn.BaseService

	// Maps a Peer.Key to that peer's TrustMetric
	peerMetrics map[string]*TrustMetric

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
		peerMetrics: make(map[string]*TrustMetric),
		db:          db,
		config:      tmc,
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
	for _, tm := range tms.peerMetrics {
		tm.Stop()
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

// AddPeerTrustMetric takes an existing trust metric and associates it with a peer key.
// The caller is expected to call Start on the TrustMetric being added
func (tms *TrustMetricStore) AddPeerTrustMetric(key string, tm *TrustMetric) {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	if key == "" || tm == nil {
		return
	}
	tms.peerMetrics[key] = tm
}

// GetPeerTrustMetric returns a trust metric by peer key
func (tms *TrustMetricStore) GetPeerTrustMetric(key string) *TrustMetric {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	tm, ok := tms.peerMetrics[key]
	if !ok {
		// If the metric is not available, we will create it
		tm = NewMetricWithConfig(tms.config)
		tm.Start()
		// The metric needs to be in the map
		tms.peerMetrics[key] = tm
	}
	return tm
}

// PeerDisconnected pauses the trust metric associated with the peer identified by the key
func (tms *TrustMetricStore) PeerDisconnected(key string) {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	// If the Peer that disconnected has a metric, pause it
	if tm, ok := tms.peerMetrics[key]; ok {
		tm.Pause()
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

// size returns the number of entries in the store without acquiring the mutex
func (tms *TrustMetricStore) size() int {
	return len(tms.peerMetrics)
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

	peers := make(map[string]MetricHistoryJSON)
	err := json.Unmarshal(bytes, &peers)
	if err != nil {
		panic(fmt.Sprintf("Could not unmarshal Trust Metric Store DB data: %v", err))
	}

	// If history data exists in the file,
	// load it into trust metric
	for key, p := range peers {
		tm := NewMetricWithConfig(tms.config)

		tm.Start()
		tm.Init(p)
		// Load the peer trust metric into the store
		tms.peerMetrics[key] = tm
	}
	return true
}

// Saves the history data for all peers to the store DB
func (tms *TrustMetricStore) saveToDB() {
	tms.Logger.Debug("Saving TrustHistory to DB", "size", tms.size())

	peers := make(map[string]MetricHistoryJSON)

	for key, tm := range tms.peerMetrics {
		// Add an entry for the peer identified by key
		peers[key] = tm.HistoryJSON()
	}

	// Write all the data back to the DB
	bytes, err := json.Marshal(peers)
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
		case <-tms.Quit():
			break loop
		}
	}
}
