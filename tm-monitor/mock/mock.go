package mock

import (
	em "github.com/tendermint/go-event-meter"
)

type EventMeter struct {
	latencyCallback    em.LatencyCallbackFunc
	disconnectCallback em.DisconnectCallbackFunc
	eventCallback      em.EventCallbackFunc
}

func (e *EventMeter) Start() (bool, error)                              { return true, nil }
func (e *EventMeter) Stop() bool                                        { return true }
func (e *EventMeter) RegisterLatencyCallback(cb em.LatencyCallbackFunc) { e.latencyCallback = cb }
func (e *EventMeter) RegisterDisconnectCallback(cb em.DisconnectCallbackFunc) {
	e.disconnectCallback = cb
}
func (e *EventMeter) Subscribe(eventID string, cb em.EventCallbackFunc) error {
	e.eventCallback = cb
	return nil
}
func (e *EventMeter) Unsubscribe(eventID string) error {
	e.eventCallback = nil
	return nil
}

func (e *EventMeter) Call(callback string, args ...interface{}) {
	switch callback {
	case "latencyCallback":
		e.latencyCallback(args[0].(float64))
	case "disconnectCallback":
		e.disconnectCallback()
	case "eventCallback":
		e.eventCallback(args[0].(*em.EventMetric), args[1])
	}
}
