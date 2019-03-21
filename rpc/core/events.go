package core

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
)

// Subscribe for events via WebSocket.
//
// To tell which events you want, you need to provide a query. query is a
// string, which has a form: "condition AND condition ..." (no OR at the
// moment). condition has a form: "key operation operand". key is a string with
// a restricted set of possible symbols ( \t\n\r\\()"'=>< are not allowed).
// operation can be "=", "<", "<=", ">", ">=", "CONTAINS". operand can be a
// string (escaped with single quotes), number, date or time.
//
// Examples:
//		tm.event = 'NewBlock'								# new blocks
//		tm.event = 'CompleteProposal'				# node got a complete proposal
//		tm.event = 'Tx' AND tx.hash = 'XYZ' # single transaction
//		tm.event = 'Tx' AND tx.height = 5		# all txs of the fifth block
//		tx.height = 5												# all txs of the fifth block
//
// Tendermint provides a few predefined keys: tm.event, tx.hash and tx.height.
// Note for transactions, you can define additional keys by providing tags with
// DeliverTx response.
//
//		DeliverTx{
//			Tags: []*KVPair{
//				"agent.name": "K",
//			}
//	  }
//
//		tm.event = 'Tx' AND agent.name = 'K'
//		tm.event = 'Tx' AND account.created_at >= TIME 2013-05-03T14:45:00Z
//		tm.event = 'Tx' AND contract.sign_date = DATE 2017-01-01
//		tm.event = 'Tx' AND account.owner CONTAINS 'Igor'
//
// See list of all possible events here
// https://godoc.org/github.com/tendermint/tendermint/types#pkg-constants
//
// For complete query syntax, check out
// https://godoc.org/github.com/tendermint/tendermint/libs/pubsub/query.
//
// ```go
// import "github.com/tendermint/tendermint/libs/pubsub/query"
// import "github.com/tendermint/tendermint/types"
//
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// ctx, cancel := context.WithTimeout(context.Background(), timeout)
// defer cancel()
// query := query.MustParse("tm.event = 'Tx' AND tx.height = 3")
// txs := make(chan interface{})
// err = client.Subscribe(ctx, "test-client", query, txs)
//
// go func() {
//   for e := range txs {
//     fmt.Println("got ", e.(types.EventDataTx))
//	 }
// }()
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
// | query     | string | ""      | true     | Query       |
//
// <aside class="notice">WebSocket only</aside>
func Subscribe(ctx *rpctypes.Context, query string) (*ctypes.ResultSubscribe, error) {
	addr := ctx.RemoteAddr()

	if eventBus.NumClients() >= config.MaxSubscriptionClients {
		return nil, fmt.Errorf("max_subscription_clients %d reached", config.MaxSubscriptionClients)
	} else if eventBus.NumClientSubscriptions(addr) >= config.MaxSubscriptionsPerClient {
		return nil, fmt.Errorf("max_subscriptions_per_client %d reached", config.MaxSubscriptionsPerClient)
	}

	logger.Info("Subscribe to query", "remote", addr, "query", query)

	q, err := tmquery.New(query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse query")
	}
	subCtx, cancel := context.WithTimeout(ctx.Context(), SubscribeTimeout)
	defer cancel()
	sub, err := eventBus.Subscribe(subCtx, addr, q)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case msg := <-sub.Out():
				resultEvent := &ctypes.ResultEvent{Query: query, Data: msg.Data(), Tags: msg.Tags()}
				ctx.WSConn.TryWriteRPCResponse(
					rpctypes.NewRPCSuccessResponse(
						ctx.WSConn.Codec(),
						rpctypes.JSONRPCStringID(fmt.Sprintf("%v#event", ctx.JSONReq.ID)),
						resultEvent,
					))
			case <-sub.Cancelled():
				if sub.Err() != tmpubsub.ErrUnsubscribed {
					var reason string
					if sub.Err() == nil {
						reason = "Tendermint exited"
					} else {
						reason = sub.Err().Error()
					}
					ctx.WSConn.TryWriteRPCResponse(
						rpctypes.RPCServerError(rpctypes.JSONRPCStringID(
							fmt.Sprintf("%v#event", ctx.JSONReq.ID)),
							fmt.Errorf("subscription was cancelled (reason: %s)", reason),
						))
				}
				return
			}
		}
	}()

	return &ctypes.ResultSubscribe{}, nil
}

// Unsubscribe from events via WebSocket.
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// err = client.Unsubscribe("test-client", query)
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
// | query     | string | ""      | true     | Query       |
//
// <aside class="notice">WebSocket only</aside>
func Unsubscribe(ctx *rpctypes.Context, query string) (*ctypes.ResultUnsubscribe, error) {
	addr := ctx.RemoteAddr()
	logger.Info("Unsubscribe from query", "remote", addr, "query", query)
	q, err := tmquery.New(query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse query")
	}
	err = eventBus.Unsubscribe(context.Background(), addr, q)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsubscribe{}, nil
}

// Unsubscribe from all events via WebSocket.
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// err = client.UnsubscribeAll("test-client")
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
// <aside class="notice">WebSocket only</aside>
func UnsubscribeAll(ctx *rpctypes.Context) (*ctypes.ResultUnsubscribe, error) {
	addr := ctx.RemoteAddr()
	logger.Info("Unsubscribe from all", "remote", addr)
	err := eventBus.UnsubscribeAll(context.Background(), addr)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsubscribe{}, nil
}
