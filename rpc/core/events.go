package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/internal/eventlog"
	"github.com/tendermint/tendermint/internal/eventlog/cursor"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

const (
	// maxQueryLength is the maximum length of a query string that will be
	// accepted. This is just a safety check to avoid outlandish queries.
	maxQueryLength = 512
)

// Subscribe for events via WebSocket.
// More: https://docs.tendermint.com/main/rpc/#/Websocket/subscribe
func Subscribe(ctx *rpctypes.Context, query string) (*ctypes.ResultSubscribe, error) {
	addr := ctx.RemoteAddr()

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

	subCtx, cancel := context.WithTimeout(ctx.Context(), SubscribeTimeout)
	defer cancel()

	sub, err := env.EventBus.Subscribe(subCtx, addr, q, env.Config.SubscriptionBufferSize)
	if err != nil {
		return nil, err
	}

	closeIfSlow := env.Config.CloseOnSlowClient

	// Capture the current ID, since it can change in the future.
	subscriptionID := ctx.JSONReq.ID
	go func() {
		for {
			select {
			case msg := <-sub.Out():
				var (
					resultEvent = &ctypes.ResultEvent{Query: query, Data: msg.Data(), Events: msg.Events()}
					resp        = rpctypes.NewRPCSuccessResponse(subscriptionID, resultEvent)
				)
				writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := ctx.WSConn.WriteRPCResponse(writeCtx, resp); err != nil {
					env.Logger.Info("Can't write response (slow client)",
						"to", addr, "subscriptionID", subscriptionID, "err", err)

					if closeIfSlow {
						var (
							err  = errors.New("subscription was canceled (reason: slow client)")
							resp = rpctypes.RPCServerError(subscriptionID, err)
						)
						if !ctx.WSConn.TryWriteRPCResponse(resp) {
							env.Logger.Info("Can't write response (slow client)",
								"to", addr, "subscriptionID", subscriptionID, "err", err)
						}
						return
					}
				}
			case <-sub.Cancelled():
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
					if !ctx.WSConn.TryWriteRPCResponse(resp) {
						env.Logger.Info("Can't write response (slow client)",
							"to", addr, "subscriptionID", subscriptionID, "err", err)
					}
				}
				return
			}
		}
	}()

	return &ctypes.ResultSubscribe{}, nil
}

// Unsubscribe from events via WebSocket.
// More: https://docs.tendermint.com/main/rpc/#/Websocket/unsubscribe
func Unsubscribe(ctx *rpctypes.Context, query string) (*ctypes.ResultUnsubscribe, error) {
	addr := ctx.RemoteAddr()
	env.Logger.Info("Unsubscribe from query", "remote", addr, "query", query)
	q, err := tmquery.New(query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}
	err = env.EventBus.Unsubscribe(context.Background(), addr, q)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsubscribe{}, nil
}

// UnsubscribeAll from all events via WebSocket.
// More: https://docs.tendermint.com/main/rpc/#/Websocket/unsubscribe_all
func UnsubscribeAll(ctx *rpctypes.Context) (*ctypes.ResultUnsubscribe, error) {
	addr := ctx.RemoteAddr()
	env.Logger.Info("Unsubscribe from all", "remote", addr)
	err := env.EventBus.UnsubscribeAll(context.Background(), addr)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsubscribe{}, nil
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
func Events(ctx *rpctypes.Context,
	filter string,
	maxItems int,
	before, after string,
	waitTime time.Duration,
) (*ctypes.ResultEvents, error) {
	var curBefore, curAfter cursor.Cursor
	if err := curBefore.UnmarshalText([]byte(before)); err != nil {
		return nil, err
	}
	if err := curAfter.UnmarshalText([]byte(after)); err != nil {
		return nil, err
	}
	return EventsWithContext(ctx.Context(), &ctypes.EventFilter{
		Query: filter,
	}, maxItems, curBefore, curAfter, waitTime)
}

func EventsWithContext(ctx context.Context,
	filter *ctypes.EventFilter,
	maxItems int,
	before, after cursor.Cursor,
	waitTime time.Duration,
) (*ctypes.ResultEvents, error) {
	if env.EventLog == nil {
		return nil, errors.New("the event log is not enabled")
	}

	// Parse and validate parameters.
	if maxItems <= 0 {
		maxItems = 10
	} else if maxItems > 100 {
		maxItems = 100
	}

	const maxWaitTime = 30 * time.Second
	if waitTime > maxWaitTime {
		waitTime = maxWaitTime
	}

	query := tmquery.All
	if filter != nil && filter.Query != "" {
		q, err := tmquery.New(filter.Query)
		if err != nil {
			return nil, fmt.Errorf("invalid filter query: %w", err)
		}
		query = q
	}

	var info eventlog.Info
	var items []*eventlog.Item
	var err error
	accept := func(itm *eventlog.Item) error {
		// N.B. We accept up to one item more than requested, so we can tell how
		// to set the "more" flag in the response.
		if len(items) > maxItems {
			return eventlog.ErrStopScan
		}
		match, err := query.MatchesEvents(itm.Events)
		if err != nil {
			return fmt.Errorf("matches failed: %v", err)
		}
		if cursorInRange(itm.Cursor, before, after) && match {
			items = append(items, itm)
		}
		return nil
	}

	if waitTime > 0 && before.IsZero() {
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
	return &ctypes.ResultEvents{
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

func marshalItems(items []*eventlog.Item) ([]*ctypes.EventItem, error) {
	out := make([]*ctypes.EventItem, len(items))
	for i, itm := range items {
		// FIXME: align usage after remove type-tag
		v, err := json.Marshal(itm.Data)
		if err != nil {
			return nil, fmt.Errorf("encoding event data: %w", err)
		}
		out[i] = &ctypes.EventItem{Cursor: itm.Cursor.String(), Event: itm.Type}
		out[i].Data = v
	}
	return out, nil
}
