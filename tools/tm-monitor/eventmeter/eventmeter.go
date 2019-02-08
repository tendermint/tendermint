// eventmeter - generic system to subscribe to events and record their frequency.
package eventmeter

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	metrics "github.com/rcrowley/go-metrics"

	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	client "github.com/tendermint/tendermint/rpc/lib/client"
)

const (
	// Get ping/pong latency and call LatencyCallbackFunc with this period.
	latencyPeriod = 1 * time.Second

	// Check if the WS client is connected every
	connectionCheckPeriod = 100 * time.Millisecond
)

// EventMetric exposes metrics for an event.
type EventMetric struct {
	ID          string    `json:"id"`
	Started     time.Time `json:"start_time"`
	LastHeard   time.Time `json:"last_heard"`
	MinDuration int64     `json:"min_duration"`
	MaxDuration int64     `json:"max_duration"`

	// tracks event count and rate
	meter metrics.Meter

	// filled in from the Meter
	Count    int64   `json:"count"`
	Rate1    float64 `json:"rate_1" amino:"unsafe"`
	Rate5    float64 `json:"rate_5" amino:"unsafe"`
	Rate15   float64 `json:"rate_15" amino:"unsafe"`
	RateMean float64 `json:"rate_mean" amino:"unsafe"`

	// so the event can have effects in the eventmeter's consumer. runs in a go
	// routine.
	callback EventCallbackFunc
}

func (metric *EventMetric) Copy() *EventMetric {
	metricCopy := *metric
	metricCopy.meter = metric.meter.Snapshot()
	return &metricCopy
}

// called on GetMetric
func (metric *EventMetric) fillMetric() *EventMetric {
	metric.Count = metric.meter.Count()
	metric.Rate1 = metric.meter.Rate1()
	metric.Rate5 = metric.meter.Rate5()
	metric.Rate15 = metric.meter.Rate15()
	metric.RateMean = metric.meter.RateMean()
	return metric
}

// EventCallbackFunc is a closure to enable side effects from receiving an
// event.
type EventCallbackFunc func(em *EventMetric, data interface{})

// EventUnmarshalFunc is a closure to get the query and data out of the raw
// JSON received over the RPC WebSocket.
type EventUnmarshalFunc func(b json.RawMessage) (string, events.EventData, error)

// LatencyCallbackFunc is a closure to enable side effects from receiving a latency.
type LatencyCallbackFunc func(meanLatencyNanoSeconds float64)

// DisconnectCallbackFunc is a closure to notify a consumer that the connection
// has died.
type DisconnectCallbackFunc func()

// EventMeter tracks events, reports latency and disconnects.
type EventMeter struct {
	wsc *client.WSClient

	mtx              sync.Mutex
	queryToMetricMap map[string]*EventMetric

	unmarshalEvent     EventUnmarshalFunc
	latencyCallback    LatencyCallbackFunc
	disconnectCallback DisconnectCallbackFunc
	subscribed         bool

	quit chan struct{}

	logger log.Logger
}

func NewEventMeter(addr string, unmarshalEvent EventUnmarshalFunc) *EventMeter {
	return &EventMeter{
		wsc:              client.NewWSClient(addr, "/websocket", client.PingPeriod(1*time.Second)),
		queryToMetricMap: make(map[string]*EventMetric),
		unmarshalEvent:   unmarshalEvent,
		logger:           log.NewNopLogger(),
	}
}

// SetLogger lets you set your own logger.
func (em *EventMeter) SetLogger(l log.Logger) {
	em.logger = l
	em.wsc.SetLogger(l.With("module", "rpcclient"))
}

// String returns a string representation of event meter.
func (em *EventMeter) String() string {
	return em.wsc.Address
}

// Start boots up event meter.
func (em *EventMeter) Start() error {
	if err := em.wsc.Start(); err != nil {
		return err
	}

	em.quit = make(chan struct{})
	go em.receiveRoutine()
	go em.disconnectRoutine()

	err := em.subscribe()
	if err != nil {
		return err
	}
	em.subscribed = true
	return nil
}

// Stop stops event meter.
func (em *EventMeter) Stop() {
	close(em.quit)

	if em.wsc.IsRunning() {
		em.wsc.Stop()
	}
}

// Subscribe for the given query. Callback function will be called upon
// receiving an event.
func (em *EventMeter) Subscribe(query string, cb EventCallbackFunc) error {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	if err := em.wsc.Subscribe(context.TODO(), query); err != nil {
		return err
	}

	metric := &EventMetric{
		meter:    metrics.NewMeter(),
		callback: cb,
	}
	em.queryToMetricMap[query] = metric
	return nil
}

// Unsubscribe from the given query.
func (em *EventMeter) Unsubscribe(query string) error {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	return em.wsc.Unsubscribe(context.TODO(), query)
}

// GetMetric fills in the latest data for an query and return a copy.
func (em *EventMeter) GetMetric(query string) (*EventMetric, error) {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	metric, ok := em.queryToMetricMap[query]
	if !ok {
		return nil, fmt.Errorf("unknown query: %s", query)
	}
	return metric.fillMetric().Copy(), nil
}

// RegisterLatencyCallback allows you to set latency callback.
func (em *EventMeter) RegisterLatencyCallback(f LatencyCallbackFunc) {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	em.latencyCallback = f
}

// RegisterDisconnectCallback allows you to set disconnect callback.
func (em *EventMeter) RegisterDisconnectCallback(f DisconnectCallbackFunc) {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	em.disconnectCallback = f
}

///////////////////////////////////////////////////////////////////////////////
// Private

func (em *EventMeter) subscribe() error {
	for query := range em.queryToMetricMap {
		if err := em.wsc.Subscribe(context.TODO(), query); err != nil {
			return err
		}
	}
	return nil
}

func (em *EventMeter) receiveRoutine() {
	latencyTicker := time.NewTicker(latencyPeriod)
	for {
		select {
		case resp := <-em.wsc.ResponsesCh:
			if resp.Error != nil {
				em.logger.Error("expected some event, got error", "err", resp.Error.Error())
				continue
			}
			query, data, err := em.unmarshalEvent(resp.Result)
			if err != nil {
				em.logger.Error("failed to unmarshal event", "err", err)
				continue
			}
			if query != "" { // FIXME how can it be an empty string?
				em.updateMetric(query, data)
			}
		case <-latencyTicker.C:
			if em.wsc.IsActive() {
				em.callLatencyCallback(em.wsc.PingPongLatencyTimer.Mean())
			}
		case <-em.wsc.Quit():
			return
		case <-em.quit:
			return
		}
	}
}

func (em *EventMeter) disconnectRoutine() {
	ticker := time.NewTicker(connectionCheckPeriod)
	for {
		select {
		case <-ticker.C:
			if em.wsc.IsReconnecting() && em.subscribed { // notify user about disconnect only once
				em.callDisconnectCallback()
				em.subscribed = false
			} else if !em.wsc.IsReconnecting() && !em.subscribed { // resubscribe
				em.subscribe()
				em.subscribed = true
			}
		case <-em.wsc.Quit():
			return
		case <-em.quit:
			return
		}
	}
}

func (em *EventMeter) updateMetric(query string, data events.EventData) {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	metric, ok := em.queryToMetricMap[query]
	if !ok {
		// we already unsubscribed, or got an unexpected query
		return
	}

	last := metric.LastHeard
	metric.LastHeard = time.Now()
	metric.meter.Mark(1)
	dur := int64(metric.LastHeard.Sub(last))
	if dur < metric.MinDuration {
		metric.MinDuration = dur
	}
	if !last.IsZero() && dur > metric.MaxDuration {
		metric.MaxDuration = dur
	}

	if metric.callback != nil {
		go metric.callback(metric.Copy(), data)
	}
}

func (em *EventMeter) callDisconnectCallback() {
	em.mtx.Lock()
	if em.disconnectCallback != nil {
		go em.disconnectCallback()
	}
	em.mtx.Unlock()
}

func (em *EventMeter) callLatencyCallback(meanLatencyNanoSeconds float64) {
	em.mtx.Lock()
	if em.latencyCallback != nil {
		go em.latencyCallback(meanLatencyNanoSeconds)
	}
	em.mtx.Unlock()
}
