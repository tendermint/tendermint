package trust

import (
	"encoding/json"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"time"
)

var (
	store *trustMetricStore
)

type peerMetricRequest struct {
	Key  string
	Resp chan *TrustMetric
}

type trustMetricStore struct {
	PeerMetrics map[string]*TrustMetric
	Requests    chan *peerMetricRequest
	Disconn     chan string
}

func init() {
	store = &trustMetricStore{
		PeerMetrics: make(map[string]*TrustMetric),
		Requests:    make(chan *peerMetricRequest, 10),
		Disconn:     make(chan string, 10),
	}

	go store.processRequests()
}

type peerHistory struct {
	NumIntervals int       `json:"intervals"`
	History      []float64 `json:"history"`
}

func loadSaveFromFile(key string, isLoad bool, data *peerHistory) *peerHistory {
	tmhome, ok := os.LookupEnv("TMHOME")
	if !ok {
		return nil
	}

	filename := filepath.Join(tmhome, "trust_history.json")

	peers := make(map[string]peerHistory, 0)
	// read in previously written history data
	content, err := ioutil.ReadFile(filename)
	if err == nil {
		err = json.Unmarshal(content, &peers)
	}

	var result *peerHistory

	if isLoad {
		if p, ok := peers[key]; ok {
			result = &p
		}
	} else {
		peers[key] = *data

		b, err := json.Marshal(peers)
		if err == nil {
			err = ioutil.WriteFile(filename, b, 0644)
		}
	}
	return result
}

func createLoadPeerMetric(key string) *TrustMetric {
	tm := NewMetric()

	if tm == nil {
		return tm
	}

	// attempt to load the peer's trust history data
	if ph := loadSaveFromFile(key, true, nil); ph != nil {
		tm.historySize = len(ph.History)

		if tm.historySize > 0 {
			tm.numIntervals = ph.NumIntervals
			tm.history = ph.History

			tm.historyValue = tm.calcHistoryValue()
		}
	}
	return tm
}

func (tms *trustMetricStore) processRequests() {
	for {
		select {
		case req := <-tms.Requests:
			tm, ok := tms.PeerMetrics[req.Key]

			if !ok {
				tm = createLoadPeerMetric(req.Key)

				if tm != nil {
					tms.PeerMetrics[req.Key] = tm
				}
			}

			req.Resp <- tm
		case key := <-tms.Disconn:
			if tm, ok := tms.PeerMetrics[key]; ok {
				ph := peerHistory{
					NumIntervals: tm.numIntervals,
					History:      tm.history,
				}

				tm.Stop()
				delete(tms.PeerMetrics, key)
				loadSaveFromFile(key, false, &ph)
			}
		}
	}
}

// request a TrustMetric by Peer Key
func GetPeerTrustMetric(key string) *TrustMetric {
	resp := make(chan *TrustMetric, 1)

	store.Requests <- &peerMetricRequest{Key: key, Resp: resp}
	return <-resp
}

// the trust metric store should know when a Peer disconnects
func PeerDisconnected(key string) {
	store.Disconn <- key
}

// keep track of Peer reliability
type TrustMetric struct {
	proportionalWeight float64
	integralWeight     float64
	numIntervals       int
	maxIntervals       int
	intervalLen        time.Duration
	history            []float64
	historySize        int
	historyMaxSize     int
	historyValue       float64
	bad, good          float64
	stop               chan int
	update             chan *updateBadGood
	trustValue         chan *reqTrustValue
}

type TrustMetricConfig struct {
	// be careful changing these weights
	ProportionalWeight float64
	IntegralWeight     float64
	// don't allow 2^HistoryMaxSize to be greater than int max value
	HistoryMaxSize int
	// each interval should be short for adapability
	// less than 30 seconds is too sensitive,
	// and greater than 5 minutes will make the metric numb
	IntervalLen time.Duration
}

func defaultConfig() *TrustMetricConfig {
	return &TrustMetricConfig{
		ProportionalWeight: 0.4,
		IntegralWeight:     0.6,
		HistoryMaxSize:     16,
		IntervalLen:        1 * time.Minute,
	}
}

type updateBadGood struct {
	IsBad bool
	Add   int
}

type reqTrustValue struct {
	Resp chan float64
}

// calculates the derivative component
func (tm *TrustMetric) derivativeValue() float64 {
	return tm.proportionalValue() - tm.historyValue
}

// strengthens the derivative component
func (tm *TrustMetric) weightedDerivative() float64 {
	var weight float64

	d := tm.derivativeValue()
	if d < 0 {
		weight = 1.0
	}

	return weight * d
}

func (tm *TrustMetric) fadedMemoryValue(interval int) float64 {
	if interval == 0 {
		// base case
		return tm.history[0]
	}

	index := int(math.Floor(math.Log(float64(interval)) / math.Log(2)))
	// map the interval value down to an actual history index
	return tm.history[index]
}

func (tm *TrustMetric) updateFadedMemory() {
	if tm.historySize < 2 {
		return
	}

	// keep the last history element
	faded := tm.history[:1]

	for i := 1; i < tm.historySize; i++ {
		x := math.Pow(2, float64(i))

		ftv := ((tm.history[i] * (x - 1)) + tm.history[i-1]) / x

		faded = append(faded, ftv)
	}

	tm.history = faded
}

// calculates the integral (history) component of the trust value
func (tm *TrustMetric) calcHistoryValue() float64 {
	var wk []float64

	// create the weights
	hlen := tm.numIntervals
	for i := 0; i < hlen; i++ {
		x := math.Pow(.8, float64(i+1)) // optimistic wk
		wk = append(wk, x)
	}

	var wsum float64
	// calculate the sum of the weights
	for _, v := range wk {
		wsum += v
	}

	var hv float64
	// calculate the history value
	for i := 0; i < hlen; i++ {
		weight := wk[i] / wsum
		hv += tm.fadedMemoryValue(i) * weight
	}
	return hv
}

// calculates the current score for good experiences
func (tm *TrustMetric) proportionalValue() float64 {
	value := 1.0
	// bad events are worth more
	total := tm.good + math.Pow(tm.bad, 2)

	if tm.bad > 0 || tm.good > 0 {
		value = tm.good / total
	}
	return value
}

func (tm *TrustMetric) calcTrustValue() float64 {
	weightedP := tm.proportionalWeight * tm.proportionalValue()
	weightedI := tm.integralWeight * tm.historyValue
	weightedD := tm.weightedDerivative()

	tv := weightedP + weightedI + weightedD
	if tv < 0 {
		tv = 0
	}
	return tv
}

func (tm *TrustMetric) processRequests() {
	t := time.NewTicker(tm.intervalLen)
	defer t.Stop()
loop:
	for {
		select {
		case bg := <-tm.update:
			if bg.IsBad {
				tm.bad += float64(bg.Add)
			} else {
				tm.good += float64(bg.Add)
			}
		case rtv := <-tm.trustValue:
			// send the calculated trust value back
			rtv.Resp <- tm.calcTrustValue()
		case <-t.C:
			newHist := tm.calcTrustValue()
			tm.history = append([]float64{newHist}, tm.history...)

			if tm.historySize < tm.historyMaxSize {
				tm.historySize++
			} else {
				tm.history = tm.history[:tm.historyMaxSize]
			}

			if tm.numIntervals < tm.maxIntervals {
				tm.numIntervals++
			}

			tm.updateFadedMemory()
			tm.historyValue = tm.calcHistoryValue()
			tm.good = 0
			tm.bad = 0
		case <-tm.stop:
			break loop
		}
	}
}

func (tm *TrustMetric) Stop() {
	tm.stop <- 1
}

// indicate that an undesirable event took place
func (tm *TrustMetric) IncBad() {
	tm.update <- &updateBadGood{IsBad: true, Add: 1}
}

// multiple undesirable events need to be acknowledged
func (tm *TrustMetric) AddBad(num int) {
	tm.update <- &updateBadGood{IsBad: true, Add: num}
}

// positive events need to be recorded as well
func (tm *TrustMetric) IncGood() {
	tm.update <- &updateBadGood{IsBad: false, Add: 1}
}

// multiple positive can be indicated in a single call
func (tm *TrustMetric) AddGood(num int) {
	tm.update <- &updateBadGood{IsBad: false, Add: num}
}

// get the dependable trust value; a score that takes a long history into account
func (tm *TrustMetric) TrustValue() float64 {
	resp := make(chan float64, 1)

	tm.trustValue <- &reqTrustValue{Resp: resp}
	return <-resp
}

func NewMetric() *TrustMetric {
	return NewMetricWithConfig(defaultConfig())
}

func NewMetricWithConfig(tmc *TrustMetricConfig) *TrustMetric {
	tm := new(TrustMetric)
	dc := defaultConfig()

	if tmc.ProportionalWeight != 0 {
		tm.proportionalWeight = tmc.ProportionalWeight
	} else {
		tm.proportionalWeight = dc.ProportionalWeight
	}

	if tmc.IntegralWeight != 0 {
		tm.integralWeight = tmc.IntegralWeight
	} else {
		tm.integralWeight = dc.IntegralWeight
	}

	if tmc.HistoryMaxSize != 0 {
		tm.historyMaxSize = tmc.HistoryMaxSize
	} else {
		tm.historyMaxSize = dc.HistoryMaxSize
	}

	if tmc.IntervalLen != time.Duration(0) {
		tm.intervalLen = tmc.IntervalLen
	} else {
		tm.intervalLen = dc.IntervalLen
	}

	// this gives our metric a tracking window of days
	tm.maxIntervals = int(math.Pow(2, float64(tm.historyMaxSize)))
	tm.historyValue = 1.0
	tm.update = make(chan *updateBadGood, 10)
	tm.trustValue = make(chan *reqTrustValue, 10)
	tm.stop = make(chan int, 1)

	go tm.processRequests()
	return tm
}
