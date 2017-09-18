package core

import (
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
)

// Subscribe for events via WebSocket.
//
// ```go
// import "github.com/tendermint/tendermint/types"
//
// client := client.NewHTTP("tcp://0.0.0.0:46657", "/websocket")
// result, err := client.AddListenerForEvent(types.EventStringNewBlock())
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
//
// ### Query Parameters
//
// | Parameter | Type   | Default | Required | Description |
// |-----------+--------+---------+----------+-------------|
// | event     | string | ""      | true     | Event name  |
//
// <aside class="notice">WebSocket only</aside>
func Subscribe(wsCtx rpctypes.WSRPCContext, event string) (*ctypes.ResultSubscribe, error) {
	logger.Info("Subscribe to event", "remote", wsCtx.GetRemoteAddr(), "event", event)
	types.AddListenerForEvent(wsCtx.GetEventSwitch(), wsCtx.GetRemoteAddr(), event, func(msg types.TMEventData) {
		// NOTE: EventSwitch callbacks must be nonblocking
		// NOTE: RPCResponses of subscribed events have id suffix "#event"
		tmResult := &ctypes.ResultEvent{event, msg}
		wsCtx.TryWriteRPCResponse(rpctypes.NewRPCSuccessResponse(wsCtx.Request.ID+"#event", tmResult))
	})
	return &ctypes.ResultSubscribe{}, nil
}

// Unsubscribe from events via WebSocket.
//
// ```go
// import 'github.com/tendermint/tendermint/types'
//
// client := client.NewHTTP("tcp://0.0.0.0:46657", "/websocket")
// result, err := client.RemoveListenerForEvent(types.EventStringNewBlock())
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
//
// ### Query Parameters
//
// | Parameter | Type   | Default | Required | Description |
// |-----------+--------+---------+----------+-------------|
// | event     | string | ""      | true     | Event name  |
//
// <aside class="notice">WebSocket only</aside>
func Unsubscribe(wsCtx rpctypes.WSRPCContext, event string) (*ctypes.ResultUnsubscribe, error) {
	logger.Info("Unsubscribe to event", "remote", wsCtx.GetRemoteAddr(), "event", event)
	wsCtx.GetEventSwitch().RemoveListenerForEvent(event, wsCtx.GetRemoteAddr())
	return &ctypes.ResultUnsubscribe{}, nil
}
