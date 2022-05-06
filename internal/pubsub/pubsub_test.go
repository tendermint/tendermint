package pubsub_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/pubsub"
	"github.com/tendermint/tendermint/internal/pubsub/query"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

const (
	clientID = "test-client"
)

// pubstring is a trivial implementation of the EventData interface for
// string-valued test data.
type pubstring string

func (pubstring) TypeTag() string { return "pubstring" }

func TestSubscribeWithArgs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	s := newTestServer(ctx, t, logger)

	t.Run("DefaultLimit", func(t *testing.T) {
		sub := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
			ClientID: clientID,
			Query:    query.All,
		}))

		require.Equal(t, 1, s.NumClients())
		require.Equal(t, 1, s.NumClientSubscriptions(clientID))

		require.NoError(t, s.Publish(pubstring("Ka-Zar")))
		sub.mustReceive(ctx, pubstring("Ka-Zar"))
	})
	t.Run("PositiveLimit", func(t *testing.T) {
		sub := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
			ClientID: clientID + "-2",
			Query:    query.All,
			Limit:    10,
		}))
		require.NoError(t, s.Publish(pubstring("Aggamon")))
		sub.mustReceive(ctx, pubstring("Aggamon"))
	})
}

func TestObserver(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := log.NewNopLogger()

	s := newTestServer(ctx, t, logger)

	done := make(chan struct{})
	var got interface{}
	require.NoError(t, s.Observe(ctx, func(msg pubsub.Message) error {
		defer close(done)
		got = msg.Data()
		return nil
	}))

	const input = pubstring("Lions and tigers and bears, oh my!")
	require.NoError(t, s.Publish(input))
	<-done
	require.Equal(t, got, input)
}

func TestObserverErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	s := newTestServer(ctx, t, logger)

	require.Error(t, s.Observe(ctx, nil, query.All))
	require.NoError(t, s.Observe(ctx, func(pubsub.Message) error { return nil }))
	require.Error(t, s.Observe(ctx, func(pubsub.Message) error { return nil }, query.All))
}

func TestPublishDoesNotBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	s := newTestServer(ctx, t, logger)

	sub := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: clientID,
		Query:    query.All,
	}))
	published := make(chan struct{})
	go func() {
		defer close(published)

		require.NoError(t, s.Publish(pubstring("Quicksilver")))
		require.NoError(t, s.Publish(pubstring("Asylum")))
		require.NoError(t, s.Publish(pubstring("Ivan")))
	}()

	select {
	case <-published:
		sub.mustReceive(ctx, pubstring("Quicksilver"))
		sub.mustFail(ctx, pubsub.ErrTerminated)
	case <-time.After(3 * time.Second):
		t.Fatal("Publishing should not have blocked")
	}
}

func TestSubscribeErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	s := newTestServer(ctx, t, logger)

	t.Run("NegativeLimitErr", func(t *testing.T) {
		_, err := s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
			ClientID: clientID,
			Query:    query.All,
			Limit:    -5,
		})
		require.Error(t, err)
	})
}

func TestSlowSubscriber(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	s := newTestServer(ctx, t, logger)

	sub := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: clientID,
		Query:    query.All,
	}))

	require.NoError(t, s.Publish(pubstring("Fat Cobra")))
	require.NoError(t, s.Publish(pubstring("Viper")))
	require.NoError(t, s.Publish(pubstring("Black Panther")))

	// We had capacity for one item, so we should get that item, but after that
	// the subscription should have been terminated by the publisher.
	sub.mustReceive(ctx, pubstring("Fat Cobra"))
	sub.mustFail(ctx, pubsub.ErrTerminated)
}

func TestDifferentClients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	s := newTestServer(ctx, t, logger)

	sub1 := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: "client-1",
		Query:    query.MustCompile(`tm.events.type='NewBlock'`),
	}))

	events := []abci.Event{{
		Type:       "tm.events",
		Attributes: []abci.EventAttribute{{Key: "type", Value: "NewBlock"}},
	}}

	require.NoError(t, s.PublishWithEvents(pubstring("Iceman"), events))
	sub1.mustReceive(ctx, pubstring("Iceman"))

	sub2 := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: "client-2",
		Query:    query.MustCompile(`tm.events.type='NewBlock' AND abci.account.name='Igor'`),
	}))

	events = []abci.Event{
		{
			Type:       "tm.events",
			Attributes: []abci.EventAttribute{{Key: "type", Value: "NewBlock"}},
		},
		{
			Type:       "abci.account",
			Attributes: []abci.EventAttribute{{Key: "name", Value: "Igor"}},
		},
	}

	require.NoError(t, s.PublishWithEvents(pubstring("Ultimo"), events))
	sub1.mustReceive(ctx, pubstring("Ultimo"))
	sub2.mustReceive(ctx, pubstring("Ultimo"))

	sub3 := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: "client-3",
		Query: query.MustCompile(
			`tm.events.type='NewRoundStep' AND abci.account.name='Igor' AND abci.invoice.number = 10`),
	}))

	events = []abci.Event{{
		Type:       "tm.events",
		Attributes: []abci.EventAttribute{{Key: "type", Value: "NewRoundStep"}},
	}}

	require.NoError(t, s.PublishWithEvents(pubstring("Valeria Richards"), events))
	sub3.mustTimeOut(ctx, 100*time.Millisecond)
}

func TestSubscribeDuplicateKeys(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	s := newTestServer(ctx, t, logger)

	testCases := []struct {
		query    string
		expected types.EventData
	}{
		{`withdraw.rewards='17'`, pubstring("Iceman")},
		{`withdraw.rewards='22'`, pubstring("Iceman")},
		{`withdraw.rewards='1' AND withdraw.rewards='22'`, pubstring("Iceman")},
		{`withdraw.rewards='100'`, nil},
	}

	for i, tc := range testCases {
		id := fmt.Sprintf("client-%d", i)
		q := query.MustCompile(tc.query)
		t.Run(id, func(t *testing.T) {
			sub := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
				ClientID: id,
				Query:    q,
			}))

			events := []abci.Event{
				{
					Type: "transfer",
					Attributes: []abci.EventAttribute{
						{Key: "sender", Value: "foo"},
						{Key: "sender", Value: "bar"},
						{Key: "sender", Value: "baz"},
					},
				},
				{
					Type: "withdraw",
					Attributes: []abci.EventAttribute{
						{Key: "rewards", Value: "1"},
						{Key: "rewards", Value: "17"},
						{Key: "rewards", Value: "22"},
					},
				},
			}

			require.NoError(t, s.PublishWithEvents(pubstring("Iceman"), events))

			if tc.expected != nil {
				sub.mustReceive(ctx, tc.expected)
			} else {
				sub.mustTimeOut(ctx, 100*time.Millisecond)
			}
		})
	}
}

func TestClientSubscribesTwice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	s := newTestServer(ctx, t, logger)

	q := query.MustCompile(`tm.events.type='NewBlock'`)
	events := []abci.Event{{
		Type:       "tm.events",
		Attributes: []abci.EventAttribute{{Key: "type", Value: "NewBlock"}},
	}}

	sub1 := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: clientID,
		Query:    q,
	}))

	require.NoError(t, s.PublishWithEvents(pubstring("Goblin Queen"), events))
	sub1.mustReceive(ctx, pubstring("Goblin Queen"))

	// Subscribing a second time with the same client ID and query fails.
	{
		sub2, err := s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
			ClientID: clientID,
			Query:    q,
		})
		require.Error(t, err)
		require.Nil(t, sub2)
	}

	// The attempt to re-subscribe does not disrupt the existing sub.
	require.NoError(t, s.PublishWithEvents(pubstring("Spider-Man"), events))
	sub1.mustReceive(ctx, pubstring("Spider-Man"))
}

func TestUnsubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	s := newTestServer(ctx, t, logger)

	sub := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: clientID,
		Query:    query.MustCompile(`tm.events.type='NewBlock'`),
	}))

	// Removing the subscription we just made should succeed.
	require.NoError(t, s.Unsubscribe(ctx, pubsub.UnsubscribeArgs{
		Subscriber: clientID,
		Query:      query.MustCompile(`tm.events.type='NewBlock'`),
	}))

	// Publishing should still work.
	require.NoError(t, s.Publish(pubstring("Nick Fury")))

	// The unsubscribed subscriber should report as such.
	sub.mustFail(ctx, pubsub.ErrUnsubscribed)
}

func TestClientUnsubscribesTwice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	s := newTestServer(ctx, t, logger)

	newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: clientID,
		Query:    query.MustCompile(`tm.events.type='NewBlock'`),
	}))
	require.NoError(t, s.Unsubscribe(ctx, pubsub.UnsubscribeArgs{
		Subscriber: clientID,
		Query:      query.MustCompile(`tm.events.type='NewBlock'`),
	}))
	require.ErrorIs(t, s.Unsubscribe(ctx, pubsub.UnsubscribeArgs{
		Subscriber: clientID,
		Query:      query.MustCompile(`tm.events.type='NewBlock'`),
	}), pubsub.ErrSubscriptionNotFound)
	require.ErrorIs(t, s.UnsubscribeAll(ctx, clientID), pubsub.ErrSubscriptionNotFound)
}

func TestResubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	s := newTestServer(ctx, t, logger)

	args := pubsub.SubscribeArgs{
		ClientID: clientID,
		Query:    query.All,
	}
	newTestSub(t).must(s.SubscribeWithArgs(ctx, args))

	require.NoError(t, s.Unsubscribe(ctx, pubsub.UnsubscribeArgs{
		Subscriber: clientID,
		Query:      query.All,
	}))

	sub := newTestSub(t).must(s.SubscribeWithArgs(ctx, args))

	require.NoError(t, s.Publish(pubstring("Cable")))
	sub.mustReceive(ctx, pubstring("Cable"))
}

func TestUnsubscribeAll(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	s := newTestServer(ctx, t, logger)

	sub1 := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: clientID,
		Query:    query.MustCompile(`tm.events.type='NewBlock'`),
	}))
	sub2 := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: clientID,
		Query:    query.MustCompile(`tm.events.type='NewBlockHeader'`),
	}))

	require.NoError(t, s.UnsubscribeAll(ctx, clientID))
	require.NoError(t, s.Publish(pubstring("Nick Fury")))

	sub1.mustFail(ctx, pubsub.ErrUnsubscribed)
	sub2.mustFail(ctx, pubsub.ErrUnsubscribed)

}

func TestBufferCapacity(t *testing.T) {
	logger := log.NewNopLogger()
	s := pubsub.NewServer(logger, pubsub.BufferCapacity(2))

	require.Equal(t, 2, s.BufferCapacity())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, s.Publish(pubstring("Nighthawk")))
	require.NoError(t, s.Publish(pubstring("Sage")))

	ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	sig := make(chan struct{})

	go func() { defer close(sig); _ = s.Publish(pubstring("Ironclad")) }()

	select {
	case <-sig:
		t.Fatal("should not fire")

	case <-ctx.Done():
		return
	}

}

func newTestServer(ctx context.Context, t testing.TB, logger log.Logger) *pubsub.Server {
	t.Helper()

	s := pubsub.NewServer(logger)

	require.NoError(t, s.Start(ctx))
	t.Cleanup(s.Wait)
	return s
}

type testSub struct {
	t testing.TB
	*pubsub.Subscription
}

func newTestSub(t testing.TB) *testSub { return &testSub{t: t} }

func (s *testSub) must(sub *pubsub.Subscription, err error) *testSub {
	s.t.Helper()
	require.NoError(s.t, err)
	require.NotNil(s.t, sub)
	s.Subscription = sub
	return s
}

func (s *testSub) mustReceive(ctx context.Context, want types.EventData) {
	s.t.Helper()
	got, err := s.Next(ctx)
	require.NoError(s.t, err)
	require.Equal(s.t, want, got.Data())
}

func (s *testSub) mustTimeOut(ctx context.Context, dur time.Duration) {
	s.t.Helper()
	tctx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()
	got, err := s.Next(tctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		s.t.Errorf("Next: got (%+v, %v), want %v", got, err, context.DeadlineExceeded)
	}
}

func (s *testSub) mustFail(ctx context.Context, want error) {
	s.t.Helper()
	got, err := s.Next(ctx)
	if err == nil && want != nil {
		s.t.Fatalf("Next: got (%+v, %v), want error %v", got, err, want)
	}
	require.ErrorIs(s.t, err, want)
}
