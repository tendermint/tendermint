package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	tmpubsub "github.com/tendermint/tendermint/internal/pubsub"
	tmquery "github.com/tendermint/tendermint/internal/pubsub/query"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

const (
	// Buffer on the Tendermint (server) side to allow some slowness in clients.
	subBufferSize = 100

	// maxQueryLength is the maximum length of a query string that will be
	// accepted. This is just a safety check to avoid outlandish queries.
	maxQueryLength = 512
)

// Subscribe for events via WebSocket.
// More: https://docs.tendermint.com/master/rpc/#/Websocket/subscribe
func (env *Environment) Subscribe(ctx context.Context, query string) (*coretypes.ResultSubscribe, error) {
	callInfo := rpctypes.GetCallInfo(ctx)
	addr := callInfo.RemoteAddr()

	if env.EventBus.NumClients() >= env.Config.MaxSubscriptionClients {
		return nil, fmt.Errorf("max_subscription_clients %d reached", env.Config.MaxSubscriptionClients)
	} else if env.EventBus.NumClientSubscriptions(addr) >= env.Config.MaxSubscriptionsPerClient {
		return nil, fmt.Errorf("max_subscriptions_per_client %d reached", env.Config.MaxSubscriptionsPerClient)
	} else if len(query) > maxQueryLength {
		return nil, errors.New("maximum query length exceeded")
	}

	env.Logger.Info("Subscribe to query", "remote", addr, "query", query)

	q, err := tmquery.New(query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	subCtx, cancel := context.WithTimeout(ctx, SubscribeTimeout)
	defer cancel()

	sub, err := env.EventBus.SubscribeWithArgs(subCtx, tmpubsub.SubscribeArgs{
		ClientID: addr,
		Query:    q,
		Limit:    subBufferSize,
	})
	if err != nil {
		return nil, err
	}

	// Capture the current ID, since it can change in the future.
	subscriptionID := callInfo.RPCRequest.ID
	go func() {
		opctx, opcancel := context.WithCancel(context.Background())
		defer opcancel()

		for {
			msg, err := sub.Next(opctx)
			if errors.Is(err, tmpubsub.ErrUnsubscribed) {
				// The subscription was removed by the client.
				return
			} else if errors.Is(err, tmpubsub.ErrTerminated) {
				// The subscription was terminated by the publisher.
				resp := callInfo.RPCRequest.MakeError(err)
				ok := callInfo.WSConn.TryWriteRPCResponse(opctx, resp)
				if !ok {
					env.Logger.Info("Unable to write response (slow client)",
						"to", addr, "subscriptionID", subscriptionID, "err", err)
				}
				return
			}

			// We have a message to deliver to the client.
			resp := callInfo.RPCRequest.MakeResponse(&coretypes.ResultEvent{
				Query:  query,
				Data:   msg.Data(),
				Events: msg.Events(),
			})
			wctx, cancel := context.WithTimeout(opctx, 10*time.Second)
			err = callInfo.WSConn.WriteRPCResponse(wctx, resp)
			cancel()
			if err != nil {
				env.Logger.Info("Unable to write response (slow client)",
					"to", addr, "subscriptionID", subscriptionID, "err", err)
			}
		}
	}()

	return &coretypes.ResultSubscribe{}, nil
}

// Unsubscribe from events via WebSocket.
// More: https://docs.tendermint.com/master/rpc/#/Websocket/unsubscribe
func (env *Environment) Unsubscribe(ctx context.Context, query string) (*coretypes.ResultUnsubscribe, error) {
	args := tmpubsub.UnsubscribeArgs{Subscriber: rpctypes.GetCallInfo(ctx).RemoteAddr()}
	env.Logger.Info("Unsubscribe from query", "remote", args.Subscriber, "subscription", query)

	var err error
	args.Query, err = tmquery.New(query)

	if err != nil {
		args.ID = query
	}

	err = env.EventBus.Unsubscribe(ctx, args)
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultUnsubscribe{}, nil
}

// UnsubscribeAll from all events via WebSocket.
// More: https://docs.tendermint.com/master/rpc/#/Websocket/unsubscribe_all
func (env *Environment) UnsubscribeAll(ctx context.Context) (*coretypes.ResultUnsubscribe, error) {
	addr := rpctypes.GetCallInfo(ctx).RemoteAddr()
	env.Logger.Info("Unsubscribe from all", "remote", addr)
	err := env.EventBus.UnsubscribeAll(ctx, addr)
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultUnsubscribe{}, nil
}
