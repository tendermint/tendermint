package eventmeter

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	metrics "github.com/rcrowley/go-metrics"
	client "github.com/tendermint/tendermint/rpc/lib/client"
	"github.com/tendermint/tmlibs/events"
	"github.com/tendermint/tmlibs/log"
)

//------------------------------------------------------
// Generic system to subscribe to events and record their frequency
//------------------------------------------------------

//------------------------------------------------------
// Meter for a particular event

// Closure to enable side effects from receiving an event
type EventCallbackFunc func(em *EventMetric, data interface{})

// Metrics for a given event
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

	// so the event can have effects in the event-meter's consumer.
	// runs in a go routine
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

//------------------------------------------------------
// Websocket client and event meter for many events

const maxPingsPerPong = 30 // if we haven't received a pong in this many attempted pings we kill the conn

// Get the eventID and data out of the raw json received over the go-rpc websocket
type EventUnmarshalFunc func(b json.RawMessage) (string, events.EventData, error)

// Closure to enable side effects from receiving a pong
type LatencyCallbackFunc func(meanLatencyNanoSeconds float64)

// Closure to notify consumer that the connection died
type DisconnectCallbackFunc func()

// Each node gets an event meter to track events for that node
type EventMeter struct {
	wsc *client.WSClient

	mtx    sync.Mutex
	events map[string]*EventMetric

	// to record ws latency
	timer              metrics.Timer
	lastPing           time.Time
	receivedPong       bool
	latencyCallback    LatencyCallbackFunc
	disconnectCallback DisconnectCallbackFunc

	unmarshalEvent EventUnmarshalFunc

	quit chan struct{}

	logger log.Logger
}

func NewEventMeter(addr string, unmarshalEvent EventUnmarshalFunc) *EventMeter {
	em := &EventMeter{
		wsc:            client.NewWSClient(addr, "/websocket"),
		events:         make(map[string]*EventMetric),
		timer:          metrics.NewTimer(),
		receivedPong:   true,
		unmarshalEvent: unmarshalEvent,
		logger:         log.NewNopLogger(),
	}
	return em
}

// SetLogger lets you set your own logger
func (em *EventMeter) SetLogger(l log.Logger) {
	em.logger = l
}

func (em *EventMeter) String() string {
	return em.wsc.Address
}

func (em *EventMeter) Start() error {
	if _, err := em.wsc.Reset(); err != nil {
		return err
	}

	if _, err := em.wsc.Start(); err != nil {
		return err
	}

	em.wsc.Conn.SetPongHandler(func(m string) error {
		// NOTE: https://github.com/gorilla/websocket/issues/97
		em.mtx.Lock()
		defer em.mtx.Unlock()
		em.receivedPong = true
		em.timer.UpdateSince(em.lastPing)
		if em.latencyCallback != nil {
			go em.latencyCallback(em.timer.Mean())
		}
		return nil
	})

	em.quit = make(chan struct{})
	go em.receiveRoutine()

	return em.resubscribe()
}

// Stop stops the EventMeter.
func (em *EventMeter) Stop() {
	close(em.quit)

	if em.wsc.IsRunning() {
		em.wsc.Stop()
	}
}

// StopAndCallDisconnectCallback stops the EventMeter and calls
// disconnectCallback if present.
func (em *EventMeter) StopAndCallDisconnectCallback() {
	if em.wsc.IsRunning() {
		em.wsc.Stop()
	}

	em.mtx.Lock()
	defer em.mtx.Unlock()
	if em.disconnectCallback != nil {
		go em.disconnectCallback()
	}
}

func (em *EventMeter) Subscribe(eventID string, cb EventCallbackFunc) error {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	if _, ok := em.events[eventID]; ok {
		return fmt.Errorf("subscribtion already exists")
	}
	if err := em.wsc.Subscribe(eventID); err != nil {
		return err
	}

	metric := &EventMetric{
		ID:          eventID,
		Started:     time.Now(),
		MinDuration: 1 << 62,
		meter:       metrics.NewMeter(),
		callback:    cb,
	}
	em.events[eventID] = metric
	return nil
}

func (em *EventMeter) Unsubscribe(eventID string) error {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	if err := em.wsc.Unsubscribe(eventID); err != nil {
		return err
	}
	// XXX: should we persist or save this info first?
	delete(em.events, eventID)
	return nil
}

// Fill in the latest data for an event and return a copy
func (em *EventMeter) GetMetric(eventID string) (*EventMetric, error) {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	metric, ok := em.events[eventID]
	if !ok {
		return nil, fmt.Errorf("Unknown event %s", eventID)
	}
	return metric.fillMetric().Copy(), nil
}

// Return the average latency over the websocket
func (em *EventMeter) Latency() float64 {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	return em.timer.Mean()
}

func (em *EventMeter) RegisterLatencyCallback(f LatencyCallbackFunc) {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	em.latencyCallback = f
}

func (em *EventMeter) RegisterDisconnectCallback(f DisconnectCallbackFunc) {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	em.disconnectCallback = f
}

//------------------------------------------------------

func (em *EventMeter) resubscribe() error {
	for eventID, _ := range em.events {
		if err := em.wsc.Subscribe(eventID); err != nil {
			return err
		}
	}
	return nil
}

func (em *EventMeter) receiveRoutine() {
	pingTime := time.Second * 1
	pingTicker := time.NewTicker(pingTime)
	pingAttempts := 0 // if this hits maxPingsPerPong we kill the conn

	var err error
	for {
		select {
		case <-pingTicker.C:
			if pingAttempts, err = em.pingForLatency(pingAttempts); err != nil {
				em.logger.Error("err", errors.Wrap(err, "failed to write ping message on websocket"))
				em.StopAndCallDisconnectCallback()
				return
			} else if pingAttempts >= maxPingsPerPong {
				em.logger.Error("err", errors.Errorf("Have not received a pong in %v", time.Duration(pingAttempts)*pingTime))
				em.StopAndCallDisconnectCallback()
				return
			}
		case r := <-em.wsc.ResultsCh:
			if r == nil {
				em.logger.Error("err", errors.New("Expected some event, received nil"))
				em.StopAndCallDisconnectCallback()
				return
			}
			eventID, data, err := em.unmarshalEvent(r)
			if err != nil {
				em.logger.Error("err", errors.Wrap(err, "failed to unmarshal event"))
				continue
			}
			if eventID != "" {
				em.updateMetric(eventID, data)
			}
		case <-em.wsc.Quit:
			em.logger.Error("err", errors.New("WSClient closed unexpectedly"))
			em.StopAndCallDisconnectCallback()
			return
		case <-em.quit:
			return
		}
	}
}

func (em *EventMeter) pingForLatency(pingAttempts int) (int, error) {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	// ping to record latency
	if !em.receivedPong {
		return pingAttempts + 1, nil
	}

	em.lastPing = time.Now()
	em.receivedPong = false
	err := em.wsc.Conn.WriteMessage(websocket.PingMessage, []byte{})
	if err != nil {
		return pingAttempts, err
	}
	return 0, nil
}

func (em *EventMeter) updateMetric(eventID string, data events.EventData) {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	metric, ok := em.events[eventID]
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
