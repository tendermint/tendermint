package core

import (
	"context"
	"fmt"
	"time"

	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

const (
	// Buffer on the Tendermint (server) side to allow some slowness in clients.
	subBufferSize = 100
)

// Subscribe for events via WebSocket.
// More: https://docs.tendermint.com/master/rpc/#/Websocket/subscribe
func (env *Environment) Subscribe(ctx *rpctypes.Context, query string) (*coretypes.ResultSubscribe, error) {
	addr := ctx.RemoteAddr()

	if env.EventBus.NumClients() >= env.Config.MaxSubscriptionClients {
		return nil, fmt.Errorf("max_subscription_clients %d reached", env.Config.MaxSubscriptionClients)
	} else if env.EventBus.NumClientSubscriptions(addr) >= env.Config.MaxSubscriptionsPerClient {
		return nil, fmt.Errorf("max_subscriptions_per_client %d reached", env.Config.MaxSubscriptionsPerClient)
	}

	env.Logger.Info("Subscribe to query", "remote", addr, "query", query)

	q, err := tmquery.New(query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	subCtx, cancel := context.WithTimeout(ctx.Context(), SubscribeTimeout)
	defer cancel()

	sub, err := env.EventBus.Subscribe(subCtx, addr, q, subBufferSize)
	if err != nil {
		return nil, err
	}

	// Capture the current ID, since it can change in the future.
	subscriptionID := ctx.JSONReq.ID
	go func() {
		for {
			select {
			case msg := <-sub.Out():
				var (
					resultEvent = &coretypes.ResultEvent{Query: query, Data: msg.Data(), Events: msg.Events()}
					resp        = rpctypes.NewRPCSuccessResponse(subscriptionID, resultEvent)
				)
				writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := ctx.WSConn.WriteRPCResponse(writeCtx, resp); err != nil {
					env.Logger.Info("Can't write response (slow client)",
						"to", addr, "subscriptionID", subscriptionID, "err", err)
				}
			case <-sub.Canceled():
				if sub.Err() != tmpubsub.ErrUnsubscribed {
					var reason string
					if sub.Err() == nil {
						reason = "Tendermint exited"
					} else {
						reason = sub.Err().Error()
					}
					var (
						err  = fmt.Errorf("subscription was canceled (reason: %s)", reason)
						resp = rpctypes.RPCServerError(subscriptionID, err)
					)
					if ok := ctx.WSConn.TryWriteRPCResponse(resp); !ok {
						env.Logger.Info("Can't write response (slow client)",
							"to", addr, "subscriptionID", subscriptionID, "err", err)
					}
				}
				return
			}
		}
	}()

	return &coretypes.ResultSubscribe{}, nil
}

// Unsubscribe from events via WebSocket.
// More: https://docs.tendermint.com/master/rpc/#/Websocket/unsubscribe
func (env *Environment) Unsubscribe(ctx *rpctypes.Context, query string) (*coretypes.ResultUnsubscribe, error) {
	args := tmpubsub.UnsubscribeArgs{Subscriber: ctx.RemoteAddr()}
	env.Logger.Info("Unsubscribe from query", "remote", args.Subscriber, "subscription", query)

	var err error
	args.Query, err = tmquery.New(query)

	if err != nil {
		args.ID = query
	}

	err = env.EventBus.Unsubscribe(ctx.Context(), args)
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultUnsubscribe{}, nil
}

// UnsubscribeAll from all events via WebSocket.
// More: https://docs.tendermint.com/master/rpc/#/Websocket/unsubscribe_all
func (env *Environment) UnsubscribeAll(ctx *rpctypes.Context) (*coretypes.ResultUnsubscribe, error) {
	addr := ctx.RemoteAddr()
	env.Logger.Info("Unsubscribe from all", "remote", addr)
	err := env.EventBus.UnsubscribeAll(ctx.Context(), addr)
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultUnsubscribe{}, nil
}
