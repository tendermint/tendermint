// Copyright 2017 Tendermint. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package trust

import (
	"encoding/json"
	"math"
	"sync"
	"time"

	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
)

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
	tms.BaseService.OnStart()

	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	tms.loadFromDB()
	return nil
}

// OnStop implements Service
func (tms *TrustMetricStore) OnStop() {
	tms.mtx.Lock()
	defer tms.mtx.Unlock()

	// Stop all trust metric goroutines
	for _, tm := range tms.peerMetrics {
		tm.Stop()
	}

	tms.saveToDB()
	tms.BaseService.OnStop()
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
		if tm != nil {
			// The metric needs to be in the map
			tms.peerMetrics[key] = tm
		}
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

/* Private methods */

// size returns the number of entries in the store without acquiring the mutex
func (tms *TrustMetricStore) size() int {
	return len(tms.peerMetrics)
}

/* Loading & Saving */
/* Both of these methods assume the mutex has been acquired, since they write to the map */

var trustMetricKey = []byte("trustMetricStore")

type peerHistoryJSON struct {
	NumIntervals int       `json:"intervals"`
	History      []float64 `json:"history"`
}

// Loads the history data for the Peer identified by key from the store DB.
// cmn.Panics if file is corrupt
func (tms *TrustMetricStore) loadFromDB() bool {
	// Obtain the history data we have so far
	bytes := tms.db.Get(trustMetricKey)
	if bytes == nil {
		return false
	}

	peers := make(map[string]peerHistoryJSON, 0)
	err := json.Unmarshal(bytes, &peers)
	if err != nil {
		cmn.PanicCrisis(cmn.Fmt("Could not unmarchal Trust Metric Store DB data: %v", err))
	}

	// If history data exists in the file,
	// load it into trust metrics and recalc
	for key, p := range peers {
		tm := NewMetricWithConfig(tms.config)

		// Restore the number of time intervals we have previously tracked
		if p.NumIntervals > tm.maxIntervals {
			p.NumIntervals = tm.maxIntervals
		}
		tm.numIntervals = p.NumIntervals
		// Restore the history and its current size
		if len(p.History) > tm.historyMaxSize {
			p.History = p.History[:tm.historyMaxSize]
		}
		tm.history = p.History
		tm.historySize = len(tm.history)
		// Calculate the history value based on the loaded history data
		tm.historyValue = tm.calcHistoryValue()
		// Load the peer trust metric into the store
		tms.peerMetrics[key] = tm
	}
	return true
}

// Saves the history data for all peers to the store DB
func (tms *TrustMetricStore) saveToDB() {
	tms.Logger.Info("Saving TrustHistory to DB", "size", tms.size())

	peers := make(map[string]peerHistoryJSON, 0)

	for key, tm := range tms.peerMetrics {
		// Add an entry for the peer identified by key
		peers[key] = peerHistoryJSON{
			NumIntervals: tm.numIntervals,
			History:      tm.history,
		}
	}

	// Write all the data back to the DB
	bytes, err := json.Marshal(peers)
	if err != nil {
		tms.Logger.Error("Failed to encode the TrustHistory", "err", err)
		return
	}
	tms.db.SetSync(trustMetricKey, bytes)
}

//---------------------------------------------------------------------------------------

// The number of event updates that can be sent on a single metric before blocking
const defaultUpdateChanCapacity = 10

// The number of trust value requests that can be made simultaneously before blocking
const defaultRequestChanCapacity = 10

// TrustMetric - keeps track of peer reliability
// See tendermint/docs/architecture/adr-006-trust-metric.md for details
type TrustMetric struct {
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

	// Sending true on this channel stops tracking, while false pauses tracking
	stop chan bool

	// For sending information about new good/bad events to be recorded
	update chan *updateBadGood

	// The channel to request a newly calculated trust value
	trustValue chan *reqTrustValue
}

// For the TrustMetric update channel
type updateBadGood struct {
	IsBad bool
	Add   int
}

// For the TrustMetric trustValue channel
type reqTrustValue struct {
	// The requested trust value is sent back on this channel
	Resp chan float64
}

// Pause tells the metric to pause recording data over time intervals.
// All method calls that indicate events will unpause the metric
func (tm *TrustMetric) Pause() {
	tm.stop <- false
}

// Stop tells the metric to stop recording data over time intervals
func (tm *TrustMetric) Stop() {
	tm.stop <- true
}

// BadEvent indicates that an undesirable event took place
func (tm *TrustMetric) BadEvent() {
	tm.update <- &updateBadGood{IsBad: true, Add: 1}
}

// AddBadEvents acknowledges multiple undesirable events
func (tm *TrustMetric) AddBadEvents(num int) {
	tm.update <- &updateBadGood{IsBad: true, Add: num}
}

// GoodEvent indicates that a desirable event took place
func (tm *TrustMetric) GoodEvent() {
	tm.update <- &updateBadGood{IsBad: false, Add: 1}
}

// AddGoodEvents acknowledges multiple desirable events
func (tm *TrustMetric) AddGoodEvents(num int) {
	tm.update <- &updateBadGood{IsBad: false, Add: num}
}

// TrustValue gets the dependable trust value; always between 0 and 1
func (tm *TrustMetric) TrustValue() float64 {
	resp := make(chan float64, 1)

	tm.trustValue <- &reqTrustValue{Resp: resp}
	return <-resp
}

// TrustScore gets a score based on the trust value always between 0 and 100
func (tm *TrustMetric) TrustScore() int {
	resp := make(chan float64, 1)

	tm.trustValue <- &reqTrustValue{Resp: resp}
	return int(math.Floor(<-resp * 100))
}

// TrustMetricConfig - Configures the weight functions and time intervals for the metric
type TrustMetricConfig struct {
	// Determines the percentage given to current behavior
	ProportionalWeight float64

	// Determines the percentage given to prior behavior
	IntegralWeight float64

	// The window of time that the trust metric will track events across.
	// This can be set to cover many days without issue
	TrackingWindow time.Duration

	// Each interval should be short for adapability.
	// Less than 30 seconds is too sensitive,
	// and greater than 5 minutes will make the metric numb
	IntervalLength time.Duration
}

// DefaultConfig returns a config with values that have been tested and produce desirable results
func DefaultConfig() TrustMetricConfig {
	return TrustMetricConfig{
		ProportionalWeight: 0.4,
		IntegralWeight:     0.6,
		TrackingWindow:     (time.Minute * 60 * 24) * 14, // 14 days.
		IntervalLength:     1 * time.Minute,
	}
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
	tm.historyMaxSize = intervalToHistoryIndex(tm.maxIntervals) + 1
	// This metric has a perfect history so far
	tm.historyValue = 1.0
	// Setup the channels
	tm.update = make(chan *updateBadGood, defaultUpdateChanCapacity)
	tm.trustValue = make(chan *reqTrustValue, defaultRequestChanCapacity)
	tm.stop = make(chan bool, 2)

	go tm.processRequests()
	return tm
}

/* Private methods */

// Ensures that all configuration elements have valid values
func customConfig(tmc TrustMetricConfig) TrustMetricConfig {
	config := DefaultConfig()

	// Check the config for set values, and setup appropriately
	if tmc.ProportionalWeight > 0 {
		config.ProportionalWeight = tmc.ProportionalWeight
	}

	if tmc.IntegralWeight > 0 {
		config.IntegralWeight = tmc.IntegralWeight
	}

	if tmc.IntervalLength > time.Duration(0) {
		config.IntervalLength = tmc.IntervalLength
	}

	if tmc.TrackingWindow > time.Duration(0) &&
		tmc.TrackingWindow >= config.IntervalLength {
		config.TrackingWindow = tmc.TrackingWindow
	}

	return config
}

// Calculates the derivative component
func (tm *TrustMetric) derivativeValue() float64 {
	return tm.proportionalValue() - tm.historyValue
}

// Strengthens the derivative component when the change is negative
func (tm *TrustMetric) weightedDerivative() float64 {
	var weight float64

	d := tm.derivativeValue()

	if d < 0 {
		weight = 1.0
	}
	return weight * d
}

// Map the interval value down to an actual history index
func intervalToHistoryIndex(interval int) int {
	return int(math.Floor(math.Log(float64(interval)) / math.Log(2)))
}

// Retrieves the actual history data value that represents the requested time interval
func (tm *TrustMetric) fadedMemoryValue(interval int) float64 {
	if interval == 0 {
		// Base case
		return tm.history[0]
	}
	return tm.history[intervalToHistoryIndex(interval)]
}

// Performs the update for our Faded Memories process, which allows the
// trust metric tracking window to be large while maintaining a small
// number of history data values
func (tm *TrustMetric) updateFadedMemory() {
	if tm.historySize < 2 {
		return
	}

	// Keep the most recent history element
	faded := tm.history[:1]

	for i := 1; i < tm.historySize; i++ {
		// The older the data is, the more we spread it out
		x := math.Pow(2, float64(i))
		// Two history data values are merged into a single value
		ftv := ((tm.history[i] * (x - 1)) + tm.history[i-1]) / x
		faded = append(faded, ftv)
	}

	tm.history = faded
}

// Calculates the integral (history) component of the trust value
func (tm *TrustMetric) calcHistoryValue() float64 {
	var wk []float64

	// Create the weights.
	hlen := tm.numIntervals
	for i := 0; i < hlen; i++ {
		x := math.Pow(.8, float64(i+1)) // Optimistic weight
		wk = append(wk, x)
	}

	var wsum float64
	// Calculate the sum of the weights
	for _, v := range wk {
		wsum += v
	}

	var hv float64
	// Calculate the history value
	for i := 0; i < hlen; i++ {
		weight := wk[i] / wsum
		hv += tm.fadedMemoryValue(i) * weight
	}
	return hv
}

// Calculates the current score for good/bad experiences
func (tm *TrustMetric) proportionalValue() float64 {
	value := 1.0
	// Bad events are worth more in the calculation of our score
	total := tm.good + math.Pow(tm.bad, 2)

	if tm.bad > 0 || tm.good > 0 {
		value = tm.good / total
	}
	return value
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

// This method is for a goroutine that handles all requests on the metric
func (tm *TrustMetric) processRequests() {
	t := time.NewTicker(tm.intervalLen)
	defer t.Stop()
loop:
	for {
		select {
		case bg := <-tm.update:
			// Check if this is the first experience with
			// what we are tracking since being paused
			if tm.paused {
				tm.good = 0
				tm.bad = 0
				// New events cause us to unpause the metric
				tm.paused = false
			}

			if bg.IsBad {
				tm.bad += float64(bg.Add)
			} else {
				tm.good += float64(bg.Add)
			}
		case rtv := <-tm.trustValue:
			rtv.Resp <- tm.calcTrustValue()
		case <-t.C:
			if !tm.paused {
				// Add the current trust value to the history data
				newHist := tm.calcTrustValue()
				tm.history = append([]float64{newHist}, tm.history...)

				// Update history and interval counters
				if tm.historySize < tm.historyMaxSize {
					tm.historySize++
				} else {
					tm.history = tm.history[:tm.historyMaxSize]
				}

				if tm.numIntervals < tm.maxIntervals {
					tm.numIntervals++
				}

				// Update the history data using Faded Memories
				tm.updateFadedMemory()
				// Calculate the history value for the upcoming time interval
				tm.historyValue = tm.calcHistoryValue()
				tm.good = 0
				tm.bad = 0
			}
		case stop := <-tm.stop:
			if stop {
				// Stop all further tracking for this metric
				break loop
			}
			// Pause the metric for now
			tm.paused = true
		}
	}
}
