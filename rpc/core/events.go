package core

import (
	"context"
	"time"

	"github.com/pkg/errors"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	tmtypes "github.com/tendermint/tendermint/types"
	tmquery "github.com/tendermint/tmlibs/pubsub/query"
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
func Subscribe(wsCtx rpctypes.WSRPCContext, query string) (*ctypes.ResultSubscribe, error) {
	addr := wsCtx.GetRemoteAddr()

	logger.Info("Subscribe to query", "remote", addr, "query", query)
	q, err := tmquery.New(query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse a query")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	ch := make(chan interface{})
	err = eventBus.Subscribe(ctx, addr, q, ch)
	if err != nil {
		return nil, errors.Wrap(err, "failed to subscribe")
	}

	wsCtx.AddSubscription(query, q)

	go func() {
		for event := range ch {
			tmResult := &ctypes.ResultEvent{query, event.(tmtypes.TMEventData)}
			wsCtx.TryWriteRPCResponse(rpctypes.NewRPCSuccessResponse(wsCtx.Request.ID+"#event", tmResult))
		}
	}()

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
func Unsubscribe(wsCtx rpctypes.WSRPCContext, query string) (*ctypes.ResultUnsubscribe, error) {
	addr := wsCtx.GetRemoteAddr()
	logger.Info("Unsubscribe from query", "remote", addr, "query", query)
	q, ok := wsCtx.DeleteSubscription(query)
	if !ok {
		return nil, errors.New("subscription not found")
	}
	eventBus.Unsubscribe(context.Background(), addr, q.(*tmquery.Query))
	return &ctypes.ResultUnsubscribe{}, nil
}

func UnsubscribeAll(wsCtx rpctypes.WSRPCContext) (*ctypes.ResultUnsubscribe, error) {
	addr := wsCtx.GetRemoteAddr()
	logger.Info("Unsubscribe from all", "remote", addr)
	eventBus.UnsubscribeAll(context.Background(), addr)
	wsCtx.DeleteAllSubscriptions()
	return &ctypes.ResultUnsubscribe{}, nil
}
