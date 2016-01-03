package core

import (
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/rpc/types"
	"github.com/tendermint/tendermint/types"
)

func Subscribe(wsCtx rpctypes.WSRPCContext, event string) (*ctypes.ResultSubscribe, error) {
	log.Notice("Subscribe to event", "remote", wsCtx.GetRemoteAddr(), "event", event)
	wsCtx.GetEventSwitch().AddListenerForEvent(wsCtx.GetRemoteAddr(), event, func(msg types.EventData) {
		// NOTE: EventSwitch callbacks must be nonblocking
		// NOTE: RPCResponses of subscribed events have id suffix "#event"
		wsCtx.TryWriteRPCResponse(rpctypes.NewRPCResponse(wsCtx.Request.ID+"#event", ctypes.ResultEvent{event, msg}, ""))
	})
	return &ctypes.ResultSubscribe{}, nil
}

func Unsubscribe(wsCtx rpctypes.WSRPCContext, event string) (*ctypes.ResultUnsubscribe, error) {
	log.Notice("Unsubscribe to event", "remote", wsCtx.GetRemoteAddr(), "event", event)
	wsCtx.GetEventSwitch().AddListenerForEvent(wsCtx.GetRemoteAddr(), event, func(msg types.EventData) {
		// NOTE: EventSwitch callbacks must be nonblocking
		// NOTE: RPCResponses of subscribed events have id suffix "#event"
		wsCtx.TryWriteRPCResponse(rpctypes.NewRPCResponse(wsCtx.Request.ID+"#event", ctypes.ResultEvent{event, msg}, ""))
	})
	return &ctypes.ResultUnsubscribe{}, nil
}
