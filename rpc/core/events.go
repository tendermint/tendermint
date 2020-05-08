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

const (
	// Buffer on the Tendermint (server) side to allow some slowness in clients.
	subBufferSize = 100
)

// Subscribe for events via WebSocket.
// More: https://docs.tendermint.com/master/rpc/#/Websocket/subscribe
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

	sub, err := eventBus.Subscribe(subCtx, addr, q, subBufferSize)
	if err != nil {
		return nil, err
	}

	// Capture the current ID, since it can change in the future.
	subscriptionID := ctx.JSONReq.ID
	go func() {
		for {
			select {
			case msg := <-sub.Out():
				resultEvent := &ctypes.ResultEvent{Query: query, Data: msg.Data(), Events: msg.Events()}
				ctx.WSConn.TryWriteRPCResponse(
					rpctypes.NewRPCSuccessResponse(
						ctx.WSConn.Codec(),
						subscriptionID,
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
						rpctypes.RPCServerError(
							subscriptionID,
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
// More: https://docs.tendermint.com/master/rpc/#/Websocket/unsubscribe
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

// UnsubscribeAll from all events via WebSocket.
// More: https://docs.tendermint.com/master/rpc/#/Websocket/unsubscribe_all
func UnsubscribeAll(ctx *rpctypes.Context) (*ctypes.ResultUnsubscribe, error) {
	addr := ctx.RemoteAddr()
	logger.Info("Unsubscribe from all", "remote", addr)
	err := eventBus.UnsubscribeAll(context.Background(), addr)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsubscribe{}, nil
}
