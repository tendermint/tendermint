package mock

import (
	stdlog "log"
	"reflect"

	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/libs/log"
	em "github.com/tendermint/tendermint/tools/tm-monitor/eventmeter"
)

type EventMeter struct {
	latencyCallback    em.LatencyCallbackFunc
	disconnectCallback em.DisconnectCallbackFunc
	eventCallback      em.EventCallbackFunc
}

func (e *EventMeter) Start() error                                      { return nil }
func (e *EventMeter) Stop()                                             {}
func (e *EventMeter) SetLogger(l log.Logger)                            {}
func (e *EventMeter) RegisterLatencyCallback(cb em.LatencyCallbackFunc) { e.latencyCallback = cb }
func (e *EventMeter) RegisterDisconnectCallback(cb em.DisconnectCallbackFunc) {
	e.disconnectCallback = cb
}
func (e *EventMeter) Subscribe(query string, cb em.EventCallbackFunc) error {
	e.eventCallback = cb
	return nil
}
func (e *EventMeter) Unsubscribe(query string) error {
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

type RpcClient struct {
	Stubs map[string]interface{}
	cdc   *amino.Codec
}

func (c *RpcClient) Call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	s, ok := c.Stubs[method]
	if !ok {
		stdlog.Fatalf("Call to %s, but no stub is defined for it", method)
	}

	rv, rt := reflect.ValueOf(result), reflect.TypeOf(result)
	rv, rt = rv.Elem(), rt.Elem()
	rv.Set(reflect.ValueOf(s))

	return s, nil
}

func (c *RpcClient) Codec() *amino.Codec {
	return c.cdc
}

func (c *RpcClient) SetCodec(cdc *amino.Codec) {
	c.cdc = cdc
}
