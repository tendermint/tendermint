package core

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/pkg/errors"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmlibs/events"
)

func Subscribe(wsCtx rpctypes.WSRPCContext, event string, channels []string) (*ctypes.ResultSubscribe, error) {
	logger.Info("Subscribe to event", "remote", wsCtx.GetRemoteAddr(), "event", event, "channels", channels)

	onEvent := func(data events.EventData) {
		msg := data.(types.TMEventData)

		matches := false

		// only EventDataTx is supported at the moment
		if eventTx, ok := msg.Unwrap().(types.EventDataTx); ok {
			if len(channels) > 0 {
				// unmarshal tags
				var f interface{}
				if err := json.Unmarshal(eventTx.Data, &f); err != nil {
					err = errors.Wrap(err, "failed to unmarshal tags")
					wsCtx.TryWriteRPCResponse(rpctypes.NewRPCResponse(wsCtx.Request.ID+"#event", nil, err.Error()))
					return
				}
				m := f.(map[string]interface{})

				// try to match channels with those from EventDataTx
				if channelsFromEvent, ok := m["channels"]; ok {
					if _, ok = channelsFromEvent.([]string); ok {
					LOOP:
						for _, c1 := range channels {
							for _, c2 := range channelsFromEvent.([]string) {
								matched, err := regexp.MatchString(c1, c2)
								if err != nil {
									err = errors.Wrap(err, fmt.Sprintf("failed to match %s with %s", c1, c2))
									wsCtx.TryWriteRPCResponse(rpctypes.NewRPCResponse(wsCtx.Request.ID+"#event", nil, err.Error()))
									return
								}
								if matched {
									matches = true
									break LOOP
								}
							}
						}
					}
				}
			}
		} else {
			matches = true
		}

		if matches {
			// NOTE: EventSwitch callbacks must be nonblocking
			// NOTE: RPCResponses of subscribed events have id suffix "#event"
			result := &ctypes.ResultEvent{event, msg}
			wsCtx.TryWriteRPCResponse(rpctypes.NewRPCResponse(wsCtx.Request.ID+"#event", result, ""))
		}
	}

	wsCtx.GetEventSwitch().AddListenerForEvent(wsCtx.GetRemoteAddr(), event, onEvent)
	return &ctypes.ResultSubscribe{}, nil
}

func Unsubscribe(wsCtx rpctypes.WSRPCContext, event string) (*ctypes.ResultUnsubscribe, error) {
	logger.Info("Unsubscribe to event", "remote", wsCtx.GetRemoteAddr(), "event", event)
	wsCtx.GetEventSwitch().RemoveListenerForEvent(event, wsCtx.GetRemoteAddr())
	return &ctypes.ResultUnsubscribe{}, nil
}
