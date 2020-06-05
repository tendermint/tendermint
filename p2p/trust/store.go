// Copyright 2017 Tendermint. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package trust

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/service"
)

const defaultStorePeriodicSaveInterval = 1 * time.Minute

var trustMetricKey = []byte("trustMetricStore")

// MetricStore - Manages all trust metrics for peers
type MetricStore struct {
	service.BaseService

	// Maps a Peer.Key to that peer's TrustMetric
	peerMetrics map[string]*Metric

	// Mutex that protects the map and history data file
	mtx sync.Mutex

	// The db where peer trust metric history data will be stored
	db dbm.DB

	// This configuration will be used when creating new TrustMetrics
	config MetricConfig
}

// NewTrustMetricStore returns a store that saves data to the DB
// and uses the config when creating new trust metrics.
// Use Start to to initialize the trust metric store
func NewTrustMetricStore(db dbm.DB, tmc MetricConfig) *MetricStore {
	tms := &MetricStore{
		peerMetrics: make(map[string]*Metric),
		db:          db,
		config:      tmc,
	}

	tms.BaseService = *service.NewBaseService(nil, "MetricStore", tms)
	return tms
}

// OnStart implements Service
func (tms *MetricStore) OnStart() error {
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
func (tms *MetricStore) OnStop() {
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
func (tms *MetricStore) Size() int {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	return tms.size()
}

// AddPeerTrustMetric takes an existing trust metric and associates it with a peer key.
// The caller is expected to call Start on the TrustMetric being added
func (tms *MetricStore) AddPeerTrustMetric(key string, tm *Metric) {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	if key == "" || tm == nil {
		return
	}
	tms.peerMetrics[key] = tm
}

// GetPeerTrustMetric returns a trust metric by peer key
func (tms *MetricStore) GetPeerTrustMetric(key string) *Metric {
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
func (tms *MetricStore) PeerDisconnected(key string) {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	// If the Peer that disconnected has a metric, pause it
	if tm, ok := tms.peerMetrics[key]; ok {
		tm.Pause()
	}
}

// Saves the history data for all peers to the store DB.
// This public method acquires the trust metric store lock
func (tms *MetricStore) SaveToDB() {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	tms.saveToDB()
}

/* Private methods */

// size returns the number of entries in the store without acquiring the mutex
func (tms *MetricStore) size() int {
	return len(tms.peerMetrics)
}

/* Loading & Saving */
/* Both loadFromDB and savetoDB assume the mutex has been acquired */

// Loads the history data for all peers from the store DB
// cmn.Panics if file is corrupt
func (tms *MetricStore) loadFromDB() bool {
	// Obtain the history data we have so far
	bytes, err := tms.db.Get(trustMetricKey)
	if err != nil {
		panic(err)
	}
	if bytes == nil {
		return false
	}

	peers := make(map[string]MetricHistoryJSON)
	err = json.Unmarshal(bytes, &peers)
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
func (tms *MetricStore) saveToDB() {
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
func (tms *MetricStore) saveRoutine() {
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
