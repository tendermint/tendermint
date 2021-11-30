// Copyright 2017 Tendermint. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package trust

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	dbm "github.com/tendermint/tm-db"

	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/libs/log"
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
	mtx tmsync.Mutex

	// The db where peer trust metric history data will be stored
	db dbm.DB

	// This configuration will be used when creating new TrustMetrics
	config MetricConfig
}

// NewTrustMetricStore returns a store that saves data to the DB
// and uses the config when creating new trust metrics.
// Use Start to to initialize the trust metric store
func NewTrustMetricStore(db dbm.DB, tmc MetricConfig, logger log.Logger) *MetricStore {
	tms := &MetricStore{
		peerMetrics: make(map[string]*Metric),
		db:          db,
		config:      tmc,
	}

	tms.BaseService = *service.NewBaseService(logger, "MetricStore", tms)
	return tms
}

// OnStart implements Service
func (tms *MetricStore) OnStart(ctx context.Context) error {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	tms.loadFromDB(ctx)
	go tms.saveRoutine(ctx)
	return nil
}

// OnStop implements Service
func (tms *MetricStore) OnStop() {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	// Stop all trust metric go-routines
	wg := &sync.WaitGroup{}

	for _, tm := range tms.peerMetrics {
		wg.Add(1)
		go func(m *Metric) {
			defer wg.Done()
			if err := m.Stop(); err != nil {
				if !errors.Is(err, service.ErrAlreadyStopped) {
					tms.Logger.Info("problem stopping peer metrics",
						"peer", m.String(),
						"error", err.Error(),
					)
				}
			}
		}(tm)
	}
	wg.Wait()

	// Make the final trust history data save
	tms.saveToDB()
}

func (tms *MetricStore) Wait() {
	wg := &sync.WaitGroup{}

	for _, tm := range tms.peerMetrics {
		wg.Add(1)
		go func(m *Metric) {
			defer wg.Done()
			m.Wait()
		}(tm)
	}
	wg.Wait()
	tms.BaseService.Wait()
}

// Size returns the number of entries in the trust metric store
func (tms *MetricStore) Size() int {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	return tms.size()
}

// GetPeerTrustMetric returns a trust metric by peer key
func (tms *MetricStore) GetPeerTrustMetric(ctx context.Context, key string) *Metric {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	tm, ok := tms.peerMetrics[key]
	if !ok {
		// If the metric is not available, we will create it
		tm = NewMetricWithConfig(tms.config)
		if err := tm.Start(ctx); err != nil {
			tms.Logger.Error("unable to start metric store", "error", err)
		}
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
func (tms *MetricStore) loadFromDB(ctx context.Context) bool {
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

		if err := tm.Start(ctx); err != nil {
			tms.Logger.Error("unable to start metric", "error", err)
		}
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
	if err := tms.db.SetSync(trustMetricKey, bytes); err != nil {
		tms.Logger.Error("failed to flush data to disk", "error", err)
	}
}

// Periodically saves the trust history data to the DB
func (tms *MetricStore) saveRoutine(ctx context.Context) {
	t := time.NewTicker(defaultStorePeriodicSaveInterval)
	defer t.Stop()
loop:
	for {
		select {
		case <-t.C:
			tms.SaveToDB()
		case <-ctx.Done():
			break loop
		}
	}
}
