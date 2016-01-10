package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"

	// register rpc and event types with go-
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	//	"github.com/tendermint/tendermint/types"
	client "github.com/tendermint/tendermint/rpc/client"

	"github.com/gorilla/websocket"
	"github.com/rcrowley/go-metrics"
)

//------------------------------------------------------
// Connect to all validators for a blockchain

type Blockchain struct {
	ID         string
	Validators []Validator
}

type Validator struct {
	ID     string
	PubKey crypto.PubKey
	IP     string
	Port   int
}

//------------------------------------------------------
// Generic system to subscribe to events and record their frequency

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
	Rate1    float64 `json:"rate_1"`
	Rate5    float64 `json:"rate_5"`
	Rate15   float64 `json:"rate_15"`
	RateMean float64 `json:"rate_mean"`

	// XXX: move this
	// latency for node itself (not related to event)
	Latency float64 `json:"latency_mean"`
}

// Each node gets an event meter to track events for that node
type EventMeter struct {
	QuitService

	wsc *client.WSClient

	mtx    sync.Mutex
	events map[string]*EventMetric

	// to record latency
	timer        metrics.Timer
	lastPing     time.Time
	receivedPong bool
}

func NewEventMeter(addr string) *EventMeter {
	em := &EventMeter{
		wsc:          client.NewWSClient(addr),
		events:       make(map[string]*EventMetric),
		timer:        metrics.NewTimer(),
		receivedPong: true,
	}
	em.QuitService = *NewQuitService(nil, "EventMeter", em)
	return em
}

func (em *EventMeter) OnStart() error {
	em.QuitService.OnStart()
	if err := em.wsc.OnStart(); err != nil {
		return err
	}

	em.wsc.Conn.SetPongHandler(func(m string) error {
		// NOTE: https://github.com/gorilla/websocket/issues/97
		em.mtx.Lock()
		defer em.mtx.Unlock()
		em.receivedPong = true
		em.timer.UpdateSince(em.lastPing)
		return nil
	})
	go em.receiveRoutine()
	return nil
}

func (em *EventMeter) OnStop() {
	em.wsc.OnStop()
	em.QuitService.OnStop()
}

func (em *EventMeter) Subscribe(eventid string) error {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	if _, ok := em.events[eventid]; ok {
		return fmt.Errorf("Subscription already exists")
	}
	if err := em.wsc.Subscribe(eventid); err != nil {
		return err
	}
	em.events[eventid] = &EventMetric{
		Started:     time.Now(),
		MinDuration: 1 << 62,
		meter:       metrics.NewMeter(),
	}
	return nil
}

func (em *EventMeter) Unsubscribe(eventid string) error {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	if err := em.wsc.Unsubscribe(eventid); err != nil {
		return err
	}
	// XXX: should we persist or save this info first?
	delete(em.events, eventid)
	return nil
}

//------------------------------------------------------

func (em *EventMeter) receiveRoutine() {
	logTicker := time.NewTicker(time.Second * 3)
	pingTicker := time.NewTicker(time.Second * 1)
	for {
		select {
		case <-logTicker.C:
			em.mtx.Lock()
			for _, metric := range em.events {
				metric.Count = metric.meter.Count()
				metric.Rate1 = metric.meter.Rate1()
				metric.Rate5 = metric.meter.Rate5()
				metric.Rate15 = metric.meter.Rate15()
				metric.RateMean = metric.meter.RateMean()

				metric.Latency = em.timer.Mean()

				b, err := json.Marshal(metric)
				if err != nil {
					// TODO
					log.Error(err.Error())
					continue
				}
				var out bytes.Buffer
				json.Indent(&out, b, "", "\t")
				out.WriteTo(os.Stdout)
			}
			em.mtx.Unlock()
		case <-pingTicker.C:
			em.mtx.Lock()

			// ping to record latency
			if !em.receivedPong {
				// XXX: why is the pong taking so long? should we stop the conn?
				em.mtx.Unlock()
				continue
			}

			em.lastPing = time.Now()
			em.receivedPong = false
			err := em.wsc.Conn.WriteMessage(websocket.PingMessage, []byte{})
			if err != nil {
				log.Error("Failed to write ping message on websocket", "error", err)
				em.wsc.Stop()
				return
			}

			em.mtx.Unlock()

		case r := <-em.wsc.ResultsCh:
			em.mtx.Lock()
			switch r := r.(type) {
			case *ctypes.ResultEvent:
				id, _ := r.Event, r.Data
				metric, ok := em.events[id]
				if !ok {
					// we already unsubscribed, or got an unexpected event
					continue
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
			default:
				log.Error("Unknown result event type", "type", reflect.TypeOf(r))
			}

			em.mtx.Unlock()
		case <-em.Quit:
			break
		}

	}
}
