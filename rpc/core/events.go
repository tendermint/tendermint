package core

import (
	"github.com/tendermint/go-rpc/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

func Subscribe(wsCtx rpctypes.WSRPCContext, event string) (*ctypes.ResultSubscribe, error) {
	log.Notice("Subscribe to event", "remote", wsCtx.GetRemoteAddr(), "event", event)
	types.AddListenerForEvent(wsCtx.GetEventSwitch(), wsCtx.GetRemoteAddr(), event, func(msg types.TMEventData) {
		// NOTE: EventSwitch callbacks must be nonblocking
		// NOTE: RPCResponses of subscribed events have id suffix "#event"
		tmResult := ctypes.TMResult(&ctypes.ResultEvent{event, msg})
		wsCtx.TryWriteRPCResponse(rpctypes.NewRPCResponse(wsCtx.Request.ID+"#event", &tmResult, ""))
	})
	return &ctypes.ResultSubscribe{}, nil
}

func Unsubscribe(wsCtx rpctypes.WSRPCContext, event string) (*ctypes.ResultUnsubscribe, error) {
	log.Notice("Unsubscribe to event", "remote", wsCtx.GetRemoteAddr(), "event", event)
	wsCtx.GetEventSwitch().RemoveListenerForEvent(event, wsCtx.GetRemoteAddr())
	return &ctypes.ResultUnsubscribe{}, nil
}
