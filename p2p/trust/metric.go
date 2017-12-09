// Copyright 2017 Tendermint. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package trust

import (
	"math"
	"sync"
	"time"
)

const defaultStorePeriodicSaveInterval = 1 * time.Minute

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
// and uses the config when creating new trust metrics
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

// GetPeerTrustMetric returns a trust metric by peer key
func (tms *TrustMetricStore) GetPeerTrustMetric(key string) *TrustMetric {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	tm, ok := tms.peerMetrics[key]
	if !ok {
		// If the metric is not available, we will create it
		tm = NewMetricWithConfig(tms.config)
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

var trustMetricKey = []byte("trustMetricStore")

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
		cmn.PanicCrisis(cmn.Fmt("Could not unmarshal Trust Metric Store DB data: %v", err))
	}

	// If history data exists in the file,
	// load it into trust metric
	for key, p := range peers {
		tm := NewMetricWithConfig(tms.config)

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
		case <-tms.Quit:
			break loop
		}
	}
}

//---------------------------------------------------------------------------------------

const (
	// The weight applied to the derivative when current behavior is >= previous behavior
	defaultDerivativeGamma1 = 0

	// The weight applied to the derivative when current behavior is less than previous behavior
	defaultDerivativeGamma2 = 1.0

	// The weight applied to history data values when calculating the history value
	defaultHistoryDataWeight = 0.8
)

// MetricHistoryJSON - history data necessary to save the trust metric
type MetricHistoryJSON struct {
	NumIntervals int       `json:"intervals"`
	History      []float64 `json:"history"`
}

// TrustMetric - keeps track of peer reliability
// See tendermint/docs/architecture/adr-006-trust-metric.md for details
type TrustMetric struct {
	// Mutex that protects the metric from concurrent access
	mtx sync.Mutex

	// Determines the percentage given to current behavior
	proportionalWeight float64

	// Determines the percentage given to prior behavior
	integralWeight float64

	// Count of how many time intervals this metric has been tracking
	numIntervals int

	// Size of the time interval window for this trust metric
	maxIntervals int

	// The time duration for a single time interval
	intervalLen time.Duration

	// Stores the trust history data for this metric
	history []float64

	// Weights applied to the history data when calculating the history value
	historyWeights []float64

	// The sum of the history weights used when calculating the history value
	historyWeightSum float64

	// The current number of history data elements
	historySize int

	// The maximum number of history data elements
	historyMaxSize int

	// The calculated history value for the current time interval
	historyValue float64

	// The number of recorded good and bad events for the current time interval
	bad, good float64

	// While true, history data is not modified
	paused bool

	// Signal channel for stopping the trust metric go-routine
	stop chan struct{}
}

// NewMetric returns a trust metric with the default configuration
func NewMetric() *TrustMetric {
	return NewMetricWithConfig(DefaultConfig())
}

// NewMetricWithConfig returns a trust metric with a custom configuration
func NewMetricWithConfig(tmc TrustMetricConfig) *TrustMetric {
	tm := new(TrustMetric)
	config := customConfig(tmc)

	// Setup using the configuration values
	tm.proportionalWeight = config.ProportionalWeight
	tm.integralWeight = config.IntegralWeight
	tm.intervalLen = config.IntervalLength
	// The maximum number of time intervals is the tracking window / interval length
	tm.maxIntervals = int(config.TrackingWindow / tm.intervalLen)
	// The history size will be determined by the maximum number of time intervals
	tm.historyMaxSize = intervalToHistoryOffset(tm.maxIntervals) + 1
	// This metric has a perfect history so far
	tm.historyValue = 1.0
	// Setup the stop channel
	tm.stop = make(chan struct{})

	go tm.processRequests()
	return tm
}

// Returns a snapshot of the trust metric history data
func (tm *TrustMetric) HistoryJSON() MetricHistoryJSON {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()

	return MetricHistoryJSON{
		NumIntervals: tm.numIntervals,
		History:      tm.history,
	}
}

// Instantiates a trust metric by loading the history data for a single peer.
// This is called only once and only right after creation, which is why the
// lock is not held while accessing the trust metric struct members
func (tm *TrustMetric) Init(hist MetricHistoryJSON) {
	// Restore the number of time intervals we have previously tracked
	if hist.NumIntervals > tm.maxIntervals {
		hist.NumIntervals = tm.maxIntervals
	}
	tm.numIntervals = hist.NumIntervals
	// Restore the history and its current size
	if len(hist.History) > tm.historyMaxSize {
		// Keep the history no larger than historyMaxSize
		last := len(hist.History) - tm.historyMaxSize
		hist.History = hist.History[last:]
	}
	tm.history = hist.History
	tm.historySize = len(tm.history)
	// Create the history weight values and weight sum
	for i := 1; i <= tm.numIntervals; i++ {
		x := math.Pow(defaultHistoryDataWeight, float64(i)) // Optimistic weight
		tm.historyWeights = append(tm.historyWeights, x)
	}

	for _, v := range tm.historyWeights {
		tm.historyWeightSum += v
	}
	// Calculate the history value based on the loaded history data
	tm.historyValue = tm.calcHistoryValue()
}

// Pause tells the metric to pause recording data over time intervals.
// All method calls that indicate events will unpause the metric
func (tm *TrustMetric) Pause() {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()

	// Pause the metric for now
	tm.paused = true
}

// Stop tells the metric to stop recording data over time intervals
func (tm *TrustMetric) Stop() {
	tm.stop <- struct{}{}
}

// BadEvents indicates that an undesirable event(s) took place
func (tm *TrustMetric) BadEvents(num int) {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()

	tm.unpause()
	tm.bad += float64(num)
}

// GoodEvents indicates that a desirable event(s) took place
func (tm *TrustMetric) GoodEvents(num int) {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()

	tm.unpause()
	tm.good += float64(num)
}

// TrustValue gets the dependable trust value; always between 0 and 1
func (tm *TrustMetric) TrustValue() float64 {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()

	return tm.calcTrustValue()
}

// TrustScore gets a score based on the trust value always between 0 and 100
func (tm *TrustMetric) TrustScore() int {
	score := tm.TrustValue() * 100

	return int(math.Floor(score))
}

// NextTimeInterval saves current time interval data and prepares for the following interval
func (tm *TrustMetric) NextTimeInterval() {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()

	if tm.paused {
		// Do not prepare for the next time interval while paused
		return
	}

	// Add the current trust value to the history data
	newHist := tm.calcTrustValue()
	tm.history = append(tm.history, newHist)

	// Update history and interval counters
	if tm.historySize < tm.historyMaxSize {
		tm.historySize++
	} else {
		// Keep the history no larger than historyMaxSize
		last := len(tm.history) - tm.historyMaxSize
		tm.history = tm.history[last:]
	}

	if tm.numIntervals < tm.maxIntervals {
		tm.numIntervals++
		// Add the optimistic weight for the new time interval
		wk := math.Pow(defaultHistoryDataWeight, float64(tm.numIntervals))
		tm.historyWeights = append(tm.historyWeights, wk)
		tm.historyWeightSum += wk
	}

	// Update the history data using Faded Memories
	tm.updateFadedMemory()
	// Calculate the history value for the upcoming time interval
	tm.historyValue = tm.calcHistoryValue()
	tm.good = 0
	tm.bad = 0
}

// Copy returns a new trust metric with members containing the same values
func (tm *TrustMetric) Copy() *TrustMetric {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()
	if tm == nil {
		return nil
	}

	return &TrustMetric{
		proportionalWeight: tm.proportionalWeight,
		integralWeight:     tm.integralWeight,
		numIntervals:       tm.numIntervals,
		maxIntervals:       tm.maxIntervals,
		intervalLen:        tm.intervalLen,
		history:            tm.history,
		historyWeights:     tm.historyWeights,
		historyWeightSum:   tm.historyWeightSum,
		historySize:        tm.historySize,
		historyMaxSize:     tm.historyMaxSize,
		historyValue:       tm.historyValue,
		good:               tm.good,
		bad:                tm.bad,
		paused:             tm.paused,
		stop:               make(chan struct{}),
	}
}

/* Private methods */

// This method is for a goroutine that handles all requests on the metric
func (tm *TrustMetric) processRequests() {
	t := time.NewTicker(tm.intervalLen)
	defer t.Stop()
loop:
	for {
		select {
		case <-t.C:
			tm.NextTimeInterval()
		case <-tm.stop:
			// Stop all further tracking for this metric
			break loop
		}
	}
}

// Wakes the trust metric up if it is currently paused
// This method needs to be called with the mutex locked
func (tm *TrustMetric) unpause() {
	// Check if this is the first experience with
	// what we are tracking since being paused
	if tm.paused {
		tm.good = 0
		tm.bad = 0
		// New events cause us to unpause the metric
		tm.paused = false
	}
}

// Calculates the trust value for the request processing
func (tm *TrustMetric) calcTrustValue() float64 {
	weightedP := tm.proportionalWeight * tm.proportionalValue()
	weightedI := tm.integralWeight * tm.historyValue
	weightedD := tm.weightedDerivative()

	tv := weightedP + weightedI + weightedD
	// Do not return a negative value.
	if tv < 0 {
		tv = 0
	}
	return tv
}

// Calculates the current score for good/bad experiences
func (tm *TrustMetric) proportionalValue() float64 {
	value := 1.0

	total := tm.good + tm.bad
	if total > 0 {
		value = tm.good / total
	}
	return value
}

// Strengthens the derivative component when the change is negative
func (tm *TrustMetric) weightedDerivative() float64 {
	var weight float64 = defaultDerivativeGamma1

	d := tm.derivativeValue()
	if d < 0 {
		weight = defaultDerivativeGamma2
	}
	return weight * d
}

// Calculates the derivative component
func (tm *TrustMetric) derivativeValue() float64 {
	return tm.proportionalValue() - tm.historyValue
}

// Calculates the integral (history) component of the trust value
func (tm *TrustMetric) calcHistoryValue() float64 {
	var hv float64

	for i := 0; i < tm.numIntervals; i++ {
		hv += tm.fadedMemoryValue(i) * tm.historyWeights[i]
	}

	return hv / tm.historyWeightSum
}

// Retrieves the actual history data value that represents the requested time interval
func (tm *TrustMetric) fadedMemoryValue(interval int) float64 {
	first := tm.historySize - 1

	if interval == 0 {
		// Base case
		return tm.history[first]
	}

	offset := intervalToHistoryOffset(interval)
	return tm.history[first-offset]
}

// Performs the update for our Faded Memories process, which allows the
// trust metric tracking window to be large while maintaining a small
// number of history data values
func (tm *TrustMetric) updateFadedMemory() {
	if tm.historySize < 2 {
		return
	}

	end := tm.historySize - 1
	// Keep the most recent history element
	for count := 1; count < tm.historySize; count++ {
		i := end - count
		// The older the data is, the more we spread it out
		x := math.Pow(2, float64(count))
		// Two history data values are merged into a single value
		tm.history[i] = ((tm.history[i] * (x - 1)) + tm.history[i+1]) / x
	}
}

// Map the interval value down to an offset from the beginning of history
func intervalToHistoryOffset(interval int) int {
	// The system maintains 2^m interval values in the form of m history
	// data values. Therefore, we access the ith interval by obtaining
	// the history data index = the floor of log2(i)
	return int(math.Floor(math.Log2(float64(interval))))
}
