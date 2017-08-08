// eventmeter - generic system to subscribe to events and record their frequency.
package eventmeter

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	client "github.com/tendermint/tendermint/rpc/lib/client"
	"github.com/tendermint/tmlibs/events"
	"github.com/tendermint/tmlibs/log"
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
	Rate1    float64 `json:"rate_1" wire:"unsafe"`
	Rate5    float64 `json:"rate_5" wire:"unsafe"`
	Rate15   float64 `json:"rate_15" wire:"unsafe"`
	RateMean float64 `json:"rate_mean" wire:"unsafe"`

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

// EventUnmarshalFunc is a closure to get the eventType and data out of the raw
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

	mtx    sync.Mutex
	events map[string]*EventMetric

	unmarshalEvent     EventUnmarshalFunc
	latencyCallback    LatencyCallbackFunc
	disconnectCallback DisconnectCallbackFunc
	subscribed         bool

	quit chan struct{}

	logger log.Logger
}

func NewEventMeter(addr string, unmarshalEvent EventUnmarshalFunc) *EventMeter {
	return &EventMeter{
		wsc:            client.NewWSClient(addr, "/websocket", client.PingPong(1*time.Second, 2*time.Second)),
		events:         make(map[string]*EventMetric),
		unmarshalEvent: unmarshalEvent,
		logger:         log.NewNopLogger(),
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
	if _, err := em.wsc.Start(); err != nil {
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

// Subscribe for the given event type. Callback function will be called upon
// receiving an event.
func (em *EventMeter) Subscribe(eventType string, cb EventCallbackFunc) error {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	if _, ok := em.events[eventType]; ok {
		return fmt.Errorf("subscribtion already exists")
	}
	if err := em.wsc.Subscribe(context.TODO(), eventType); err != nil {
		return err
	}

	metric := &EventMetric{
		meter:    metrics.NewMeter(),
		callback: cb,
	}
	em.events[eventType] = metric
	return nil
}

// Unsubscribe from the given event type.
func (em *EventMeter) Unsubscribe(eventType string) error {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	if err := em.wsc.Unsubscribe(context.TODO(), eventType); err != nil {
		return err
	}
	// XXX: should we persist or save this info first?
	delete(em.events, eventType)
	return nil
}

// GetMetric fills in the latest data for an event and return a copy.
func (em *EventMeter) GetMetric(eventType string) (*EventMetric, error) {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	metric, ok := em.events[eventType]
	if !ok {
		return nil, fmt.Errorf("unknown event: %s", eventType)
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
	for eventType, _ := range em.events {
		if err := em.wsc.Subscribe(context.TODO(), eventType); err != nil {
			return err
		}
	}
	return nil
}

func (em *EventMeter) receiveRoutine() {
	latencyTicker := time.NewTicker(latencyPeriod)
	for {
		select {
		case rawEvent := <-em.wsc.ResultsCh:
			if rawEvent == nil {
				em.logger.Error("expected some event, got nil")
				continue
			}
			eventType, data, err := em.unmarshalEvent(rawEvent)
			if err != nil {
				em.logger.Error("failed to unmarshal event", "err", err)
				continue
			}
			if eventType != "" { // FIXME how can it be an empty string?
				em.updateMetric(eventType, data)
			}
		case err := <-em.wsc.ErrorsCh:
			if err != nil {
				em.logger.Error("expected some event, got error", "err", err)
			}
		case <-latencyTicker.C:
			if em.wsc.IsActive() {
				em.callLatencyCallback(em.wsc.PingPongLatencyTimer.Mean())
			}
		case <-em.wsc.Quit:
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
		case <-em.wsc.Quit:
			return
		case <-em.quit:
			return
		}
	}
}

func (em *EventMeter) updateMetric(eventType string, data events.EventData) {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	metric, ok := em.events[eventType]
	if !ok {
		// we already unsubscribed, or got an unexpected event
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
