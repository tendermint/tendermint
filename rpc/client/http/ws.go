package http

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/libs/service"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	jsonrpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

var errNotRunning = errors.New("client is not running. Use .Start() method to start")

// WSOptions for the WS part of the HTTP client.
type WSOptions struct {
	Path string // path (e.g. "/ws")

	jsonrpcclient.WSOptions // WSClient options
}

// DefaultWSOptions returns default WS options.
// See jsonrpcclient.DefaultWSOptions.
func DefaultWSOptions() WSOptions {
	return WSOptions{
		Path:      "/websocket",
		WSOptions: jsonrpcclient.DefaultWSOptions(),
	}
}

// Validate performs a basic validation of WSOptions.
func (wso WSOptions) Validate() error {
	if len(wso.Path) <= 1 {
		return errors.New("empty Path")
	}
	if wso.Path[0] != '/' {
		return errors.New("leading slash is missing in Path")
	}

	return nil
}

// wsEvents is a wrapper around WSClient, which implements EventsClient.
type wsEvents struct {
	service.BaseService
	ws *jsonrpcclient.WSClient

	mtx           tmsync.RWMutex
	subscriptions map[string]*wsSubscription
}

type wsSubscription struct {
	res   chan ctypes.ResultEvent
	id    string
	query string
}

var _ rpcclient.EventsClient = (*wsEvents)(nil)

func newWsEvents(remote string, wso WSOptions) (*wsEvents, error) {
	// validate options
	if err := wso.Validate(); err != nil {
		return nil, fmt.Errorf("invalid WSOptions: %w", err)
	}

	// remove the trailing / from the remote else the websocket endpoint
	// won't parse correctly
	if remote[len(remote)-1] == '/' {
		remote = remote[:len(remote)-1]
	}

	w := &wsEvents{
		subscriptions: make(map[string]*wsSubscription),
	}
	w.BaseService = *service.NewBaseService(nil, "wsEvents", w)

	var err error
	w.ws, err = jsonrpcclient.NewWSWithOptions(remote, wso.Path, wso.WSOptions)
	if err != nil {
		return nil, fmt.Errorf("can't create WS client: %w", err)
	}
	w.ws.OnReconnect(func() {
		// resubscribe immediately
		w.redoSubscriptionsAfter(0 * time.Second)
	})
	w.ws.SetLogger(w.Logger)

	return w, nil
}

// OnStart implements service.Service by starting WSClient and event loop.
func (w *wsEvents) OnStart() error {
	if err := w.ws.Start(); err != nil {
		return err
	}

	go w.eventListener()

	return nil
}

// OnStop implements service.Service by stopping WSClient.
func (w *wsEvents) OnStop() {
	if err := w.ws.Stop(); err != nil {
		w.Logger.Error("Can't stop ws client", "err", err)
	}
}

// Subscribe implements EventsClient by using WSClient to subscribe given
// subscriber to query. By default, it returns a channel with cap=1. Error is
// returned if it fails to subscribe.
//
// When reading from the channel, keep in mind there's a single events loop, so
// if you don't read events for this subscription fast enough, other
// subscriptions will slow down in effect.
//
// The channel is never closed to prevent clients from seeing an erroneous
// event.
//
// It returns an error if wsEvents is not running.
func (w *wsEvents) Subscribe(ctx context.Context, subscriber, query string,
	outCapacity ...int) (out <-chan ctypes.ResultEvent, err error) {

	if !w.IsRunning() {
		return nil, errNotRunning
	}

	if err := w.ws.Subscribe(ctx, query); err != nil {
		return nil, err
	}

	outCap := 1
	if len(outCapacity) > 0 {
		outCap = outCapacity[0]
	}

	outc := make(chan ctypes.ResultEvent, outCap)
	w.mtx.Lock()
	defer w.mtx.Unlock()
	// subscriber param is ignored because Tendermint will override it with
	// remote IP anyway.
	w.subscriptions[query] = &wsSubscription{res: outc, query: query}

	return outc, nil
}

// Unsubscribe implements EventsClient by using WSClient to unsubscribe given
// subscriber from query.
//
// It returns an error if wsEvents is not running.
func (w *wsEvents) Unsubscribe(ctx context.Context, subscriber, query string) error {
	if !w.IsRunning() {
		return errNotRunning
	}

	if err := w.ws.Unsubscribe(ctx, query); err != nil {
		return err
	}

	w.mtx.Lock()
	info, ok := w.subscriptions[query]
	if ok {
		if info.id != "" {
			delete(w.subscriptions, info.id)
		}
		delete(w.subscriptions, info.query)
	}
	w.mtx.Unlock()

	return nil
}

// UnsubscribeAll implements EventsClient by using WSClient to unsubscribe
// given subscriber from all the queries.
//
// It returns an error if wsEvents is not running.
func (w *wsEvents) UnsubscribeAll(ctx context.Context, subscriber string) error {
	if !w.IsRunning() {
		return errNotRunning
	}

	if err := w.ws.UnsubscribeAll(ctx); err != nil {
		return err
	}

	w.mtx.Lock()
	w.subscriptions = make(map[string]*wsSubscription)
	w.mtx.Unlock()

	return nil
}

// After being reconnected, it is necessary to redo subscription to server
// otherwise no data will be automatically received.
func (w *wsEvents) redoSubscriptionsAfter(d time.Duration) {
	time.Sleep(d)

	ctx := context.Background()

	w.mtx.Lock()
	defer w.mtx.Unlock()

	for q, info := range w.subscriptions {
		if q != "" && q == info.id {
			continue
		}
		err := w.ws.Subscribe(ctx, q)
		if err != nil {
			w.Logger.Error("failed to resubscribe", "query", q, "err", err)
			delete(w.subscriptions, q)
		}
	}
}

func isErrAlreadySubscribed(err error) bool {
	return strings.Contains(err.Error(), tmpubsub.ErrAlreadySubscribed.Error())
}

func (w *wsEvents) eventListener() {
	for {
		select {
		case resp, ok := <-w.ws.ResponsesCh:
			if !ok {
				return
			}

			if resp.Error != nil {
				w.Logger.Error("WS error", "err", resp.Error.Error())
				// Error can be ErrAlreadySubscribed or max client (subscriptions per
				// client) reached or Tendermint exited.
				// We can ignore ErrAlreadySubscribed, but need to retry in other
				// cases.
				if !isErrAlreadySubscribed(resp.Error) {
					// Resubscribe after 1 second to give Tendermint time to restart (if
					// crashed).
					w.redoSubscriptionsAfter(1 * time.Second)
				}
				continue
			}

			result := new(ctypes.ResultEvent)
			err := tmjson.Unmarshal(resp.Result, result)
			if err != nil {
				w.Logger.Error("failed to unmarshal response", "err", err)
				continue
			}

			w.mtx.RLock()
			out, ok := w.subscriptions[result.Query]
			if ok {
				if _, idOk := w.subscriptions[result.SubscriptionID]; !idOk {
					out.id = result.SubscriptionID
					w.subscriptions[result.SubscriptionID] = out
				}
			}

			w.mtx.RUnlock()
			if ok {
				select {
				case out.res <- *result:
				case <-w.Quit():
					return
				}
			}
		case <-w.Quit():
			return
		}
	}
}
