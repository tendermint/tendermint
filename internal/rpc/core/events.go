package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/internal/eventlog"
	"github.com/tendermint/tendermint/internal/eventlog/cursor"
	"github.com/tendermint/tendermint/internal/jsontypes"
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
func (env *Environment) Subscribe(ctx context.Context, req *coretypes.RequestSubscribe) (*coretypes.ResultSubscribe, error) {
	callInfo := rpctypes.GetCallInfo(ctx)
	addr := callInfo.RemoteAddr()

	if env.EventBus.NumClients() >= env.Config.MaxSubscriptionClients {
		return nil, fmt.Errorf("max_subscription_clients %d reached", env.Config.MaxSubscriptionClients)
	} else if env.EventBus.NumClientSubscriptions(addr) >= env.Config.MaxSubscriptionsPerClient {
		return nil, fmt.Errorf("max_subscriptions_per_client %d reached", env.Config.MaxSubscriptionsPerClient)
	} else if len(req.Query) > maxQueryLength {
		return nil, errors.New("maximum query length exceeded")
	}

	env.Logger.Info("WARNING: Websocket subscriptions are deprecated and will be removed " +
		"in Tendermint v0.37. See https://tinyurl.com/adr075 for more information.")
	env.Logger.Info("Subscribe to query", "remote", addr, "query", req.Query)

	q, err := tmquery.New(req.Query)
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
		opctx, opcancel := context.WithCancel(context.TODO())
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
				Query:  req.Query,
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
func (env *Environment) Unsubscribe(ctx context.Context, req *coretypes.RequestUnsubscribe) (*coretypes.ResultUnsubscribe, error) {
	args := tmpubsub.UnsubscribeArgs{Subscriber: rpctypes.GetCallInfo(ctx).RemoteAddr()}
	env.Logger.Info("Unsubscribe from query", "remote", args.Subscriber, "subscription", req.Query)

	var err error
	args.Query, err = tmquery.New(req.Query)

	if err != nil {
		args.ID = req.Query
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

// Events applies a query to the event log. If an event log is not enabled,
// Events reports an error. Otherwise, it filters the current contents of the
// log to return matching events.
//
// Events returns up to maxItems of the newest eligible event items. An item is
// eligible if it is older than before (or before is zero), it is newer than
// after (or after is zero), and its data matches the filter. A nil filter
// matches all event data.
//
// If before is zero and no eligible event items are available, Events waits
// for up to waitTime for a matching item to become available. The wait is
// terminated early if ctx ends.
//
// If maxItems ≤ 0, a default positive number of events is chosen. The values
// of maxItems and waitTime may be capped to sensible internal maxima without
// reporting an error to the caller.
func (env *Environment) Events(ctx context.Context, req *coretypes.RequestEvents) (*coretypes.ResultEvents, error) {
	if env.EventLog == nil {
		return nil, errors.New("the event log is not enabled")
	}

	// Parse and validate parameters.
	maxItems := req.MaxItems
	if maxItems <= 0 {
		maxItems = 10
	} else if maxItems > 100 {
		maxItems = 100
	}

	const minWaitTime = 1 * time.Second
	const maxWaitTime = 30 * time.Second

	waitTime := req.WaitTime
	if waitTime < minWaitTime {
		waitTime = minWaitTime
	} else if waitTime > maxWaitTime {
		waitTime = maxWaitTime
	}

	query := tmquery.All
	if req.Filter != nil && req.Filter.Query != "" {
		q, err := tmquery.New(req.Filter.Query)
		if err != nil {
			return nil, fmt.Errorf("invalid filter query: %w", err)
		}
		query = q
	}

	var before, after cursor.Cursor
	if err := before.UnmarshalText([]byte(req.Before)); err != nil {
		return nil, fmt.Errorf("invalid cursor %q: %w", req.Before, err)
	}
	if err := after.UnmarshalText([]byte(req.After)); err != nil {
		return nil, fmt.Errorf("invalid cursor %q: %w", req.After, err)
	}

	var info eventlog.Info
	var items []*eventlog.Item
	var err error
	accept := func(itm *eventlog.Item) error {
		// N.B. We accept up to one item more than requested, so we can tell how
		// to set the "more" flag in the response.
		if len(items) > maxItems || itm.Cursor.Before(after) {
			return eventlog.ErrStopScan
		}
		if cursorInRange(itm.Cursor, before, after) && query.Matches(itm.Events) {
			items = append(items, itm)
		}
		return nil
	}

	if before.IsZero() {
		ctx, cancel := context.WithTimeout(ctx, waitTime)
		defer cancel()

		// Long poll. The loop here is because new items may not match the query,
		// and we want to keep waiting until we have relevant results (or time out).
		cur := after
		for len(items) == 0 {
			info, err = env.EventLog.WaitScan(ctx, cur, accept)
			if err != nil {
				// Don't report a timeout as a request failure.
				if errors.Is(err, context.DeadlineExceeded) {
					err = nil
				}
				break
			}
			cur = info.Newest
		}
	} else {
		// Quick poll, return only what is already available.
		info, err = env.EventLog.Scan(accept)
	}
	if err != nil {
		return nil, err
	}

	more := len(items) > maxItems
	if more {
		items = items[:len(items)-1]
	}
	enc, err := marshalItems(items)
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultEvents{
		Items:  enc,
		More:   more,
		Oldest: cursorString(info.Oldest),
		Newest: cursorString(info.Newest),
	}, nil
}

func cursorString(c cursor.Cursor) string {
	if c.IsZero() {
		return ""
	}
	return c.String()
}

func cursorInRange(c, before, after cursor.Cursor) bool {
	return (before.IsZero() || c.Before(before)) && (after.IsZero() || after.Before(c))
}

func marshalItems(items []*eventlog.Item) ([]*coretypes.EventItem, error) {
	out := make([]*coretypes.EventItem, len(items))
	for i, itm := range items {
		v, err := jsontypes.Marshal(itm.Data)
		if err != nil {
			return nil, fmt.Errorf("encoding event data: %w", err)
		}
		out[i] = &coretypes.EventItem{Cursor: itm.Cursor.String(), Event: itm.Type}
		out[i].Data = v
	}
	return out, nil
}
