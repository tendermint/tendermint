package pubsub_test

import (
	"context"
	"fmt"
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/libs/pubsub/query"
)

const (
	clientID = "test-client"
)

func TestSubscribe(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	err := s.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	ctx := context.Background()
	subscription, err := s.Subscribe(ctx, clientID, query.Empty{})
	require.NoError(t, err)

	require.Equal(t, 1, s.NumClients())
	require.Equal(t, 1, s.NumClientSubscriptions(clientID))

	err = s.Publish(ctx, "Ka-Zar")
	require.NoError(t, err)
	assertReceive(t, "Ka-Zar", subscription.Out())

	published := make(chan struct{})
	go func() {
		defer close(published)

		err := s.Publish(ctx, "Quicksilver")
		require.NoError(t, err)

		err = s.Publish(ctx, "Asylum")
		require.NoError(t, err)

		err = s.Publish(ctx, "Ivan")
		require.NoError(t, err)
	}()

	select {
	case <-published:
		assertReceive(t, "Quicksilver", subscription.Out())
		assertCanceled(t, subscription, pubsub.ErrOutOfCapacity)
	case <-time.After(3 * time.Second):
		t.Fatal("Expected Publish(Asylum) not to block")
	}
}

func TestSubscribeWithCapacity(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	err := s.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	ctx := context.Background()
	require.Panics(t, func() {
		_, err = s.Subscribe(ctx, clientID, query.Empty{}, -1)
		require.NoError(t, err)
	})
	require.Panics(t, func() {
		_, err = s.Subscribe(ctx, clientID, query.Empty{}, 0)
		require.NoError(t, err)
	})
	subscription, err := s.Subscribe(ctx, clientID, query.Empty{}, 1)
	require.NoError(t, err)
	err = s.Publish(ctx, "Aggamon")
	require.NoError(t, err)
	assertReceive(t, "Aggamon", subscription.Out())
}

func TestSubscribeUnbuffered(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	err := s.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	ctx := context.Background()
	subscription, err := s.SubscribeUnbuffered(ctx, clientID, query.Empty{})
	require.NoError(t, err)

	published := make(chan struct{})
	go func() {
		defer close(published)

		err := s.Publish(ctx, "Ultron")
		require.NoError(t, err)

		err = s.Publish(ctx, "Darkhawk")
		require.NoError(t, err)
	}()

	select {
	case <-published:
		t.Fatal("Expected Publish(Darkhawk) to block")
	case <-time.After(3 * time.Second):
		assertReceive(t, "Ultron", subscription.Out())
		assertReceive(t, "Darkhawk", subscription.Out())
	}
}

func TestSlowClientIsRemovedWithErrOutOfCapacity(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	err := s.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	ctx := context.Background()
	subscription, err := s.Subscribe(ctx, clientID, query.Empty{})
	require.NoError(t, err)
	err = s.Publish(ctx, "Fat Cobra")
	require.NoError(t, err)
	err = s.Publish(ctx, "Viper")
	require.NoError(t, err)

	assertCanceled(t, subscription, pubsub.ErrOutOfCapacity)
}

func TestDifferentClients(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())

	require.NoError(t, s.Start())
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	ctx := context.Background()

	subscription1, err := s.Subscribe(ctx, "client-1", query.MustParse("tm.events.type='NewBlock'"))
	require.NoError(t, err)

	events := []abci.Event{
		{
			Type:       "tm.events",
			Attributes: []abci.EventAttribute{{Key: "type", Value: "NewBlock"}},
		},
	}

	require.NoError(t, s.PublishWithEvents(ctx, "Iceman", events))
	assertReceive(t, "Iceman", subscription1.Out())

	subscription2, err := s.Subscribe(
		ctx,
		"client-2",
		query.MustParse("tm.events.type='NewBlock' AND abci.account.name='Igor'"),
	)
	require.NoError(t, err)

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

	require.NoError(t, s.PublishWithEvents(ctx, "Ultimo", events))
	assertReceive(t, "Ultimo", subscription1.Out())
	assertReceive(t, "Ultimo", subscription2.Out())

	subscription3, err := s.Subscribe(
		ctx,
		"client-3",
		query.MustParse("tm.events.type='NewRoundStep' AND abci.account.name='Igor' AND abci.invoice.number = 10"),
	)
	require.NoError(t, err)

	events = []abci.Event{
		{
			Type:       "tm.events",
			Attributes: []abci.EventAttribute{{Key: "type", Value: "NewRoundStep"}},
		},
	}

	require.NoError(t, s.PublishWithEvents(ctx, "Valeria Richards", events))
	require.Zero(t, len(subscription3.Out()))
}

func TestSubscribeDuplicateKeys(t *testing.T) {
	ctx := context.Background()
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())

	require.NoError(t, s.Start())

	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	testCases := []struct {
		query    string
		expected interface{}
	}{
		{
			"withdraw.rewards='17'",
			"Iceman",
		},
		{
			"withdraw.rewards='22'",
			"Iceman",
		},
		{
			"withdraw.rewards='1' AND withdraw.rewards='22'",
			"Iceman",
		},
		{
			"withdraw.rewards='100'",
			nil,
		},
	}

	for i, tc := range testCases {
		sub, err := s.Subscribe(ctx, fmt.Sprintf("client-%d", i), query.MustParse(tc.query))
		require.NoError(t, err)

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

		require.NoError(t, s.PublishWithEvents(ctx, "Iceman", events))

		if tc.expected != nil {
			assertReceive(t, tc.expected, sub.Out())
		} else {
			require.Zero(t, len(sub.Out()))
		}
	}
}

func TestClientSubscribesTwice(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	err := s.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	ctx := context.Background()
	q := query.MustParse("tm.events.type='NewBlock'")

	subscription1, err := s.Subscribe(ctx, clientID, q)
	require.NoError(t, err)

	events := []abci.Event{
		{
			Type:       "tm.events",
			Attributes: []abci.EventAttribute{{Key: "type", Value: "NewBlock"}},
		},
	}

	require.NoError(t, s.PublishWithEvents(ctx, "Goblin Queen", events))
	assertReceive(t, "Goblin Queen", subscription1.Out())

	subscription2, err := s.Subscribe(ctx, clientID, q)
	require.Error(t, err)
	require.Nil(t, subscription2)

	require.NoError(t, s.PublishWithEvents(ctx, "Spider-Man", events))
	assertReceive(t, "Spider-Man", subscription1.Out())
}

func TestUnsubscribe(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	err := s.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	ctx := context.Background()
	subscription, err := s.Subscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"))
	require.NoError(t, err)
	err = s.Unsubscribe(ctx, pubsub.UnsubscribeArgs{
		Subscriber: clientID,
		Query:      query.MustParse("tm.events.type='NewBlock'")})
	require.NoError(t, err)

	err = s.Publish(ctx, "Nick Fury")
	require.NoError(t, err)
	require.Zero(t, len(subscription.Out()), "Should not receive anything after Unsubscribe")

	assertCanceled(t, subscription, pubsub.ErrUnsubscribed)
}

func TestClientUnsubscribesTwice(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	err := s.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	ctx := context.Background()
	_, err = s.Subscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"))
	require.NoError(t, err)
	err = s.Unsubscribe(ctx, pubsub.UnsubscribeArgs{
		Subscriber: clientID,
		Query:      query.MustParse("tm.events.type='NewBlock'")})
	require.NoError(t, err)

	err = s.Unsubscribe(ctx, pubsub.UnsubscribeArgs{
		Subscriber: clientID,
		Query:      query.MustParse("tm.events.type='NewBlock'")})
	require.Equal(t, pubsub.ErrSubscriptionNotFound, err)
	err = s.UnsubscribeAll(ctx, clientID)
	require.Equal(t, pubsub.ErrSubscriptionNotFound, err)
}

func TestResubscribe(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	err := s.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	ctx := context.Background()
	_, err = s.Subscribe(ctx, clientID, query.Empty{})
	require.NoError(t, err)
	err = s.Unsubscribe(ctx, pubsub.UnsubscribeArgs{Subscriber: clientID, Query: query.Empty{}})
	require.NoError(t, err)
	subscription, err := s.Subscribe(ctx, clientID, query.Empty{})
	require.NoError(t, err)

	err = s.Publish(ctx, "Cable")
	require.NoError(t, err)
	assertReceive(t, "Cable", subscription.Out())
}

func TestUnsubscribeAll(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	err := s.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	ctx := context.Background()
	subscription1, err := s.Subscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"))
	require.NoError(t, err)
	subscription2, err := s.Subscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlockHeader'"))
	require.NoError(t, err)

	err = s.UnsubscribeAll(ctx, clientID)
	require.NoError(t, err)

	err = s.Publish(ctx, "Nick Fury")
	require.NoError(t, err)
	require.Zero(t, len(subscription1.Out()), "Should not receive anything after UnsubscribeAll")
	require.Zero(t, len(subscription2.Out()), "Should not receive anything after UnsubscribeAll")

	assertCanceled(t, subscription1, pubsub.ErrUnsubscribed)
	assertCanceled(t, subscription2, pubsub.ErrUnsubscribed)
}

func TestBufferCapacity(t *testing.T) {
	s := pubsub.NewServer(pubsub.BufferCapacity(2))
	s.SetLogger(log.TestingLogger())

	require.Equal(t, 2, s.BufferCapacity())

	ctx := context.Background()
	err := s.Publish(ctx, "Nighthawk")
	require.NoError(t, err)
	err = s.Publish(ctx, "Sage")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	err = s.Publish(ctx, "Ironclad")
	if assert.Error(t, err) {
		require.Equal(t, context.DeadlineExceeded, err)
	}
}

func Benchmark10Clients(b *testing.B)   { benchmarkNClients(10, b) }
func Benchmark100Clients(b *testing.B)  { benchmarkNClients(100, b) }
func Benchmark1000Clients(b *testing.B) { benchmarkNClients(1000, b) }

func Benchmark10ClientsOneQuery(b *testing.B)   { benchmarkNClientsOneQuery(10, b) }
func Benchmark100ClientsOneQuery(b *testing.B)  { benchmarkNClientsOneQuery(100, b) }
func Benchmark1000ClientsOneQuery(b *testing.B) { benchmarkNClientsOneQuery(1000, b) }

func benchmarkNClients(n int, b *testing.B) {
	s := pubsub.NewServer()
	err := s.Start()
	require.NoError(b, err)

	b.Cleanup(func() {
		if err := s.Stop(); err != nil {
			b.Error(err)
		}
	})

	ctx := context.Background()
	for i := 0; i < n; i++ {
		subscription, err := s.Subscribe(
			ctx,
			clientID,
			query.MustParse(fmt.Sprintf("abci.Account.Owner = 'Ivan' AND abci.Invoices.Number = %d", i)),
		)
		if err != nil {
			b.Fatal(err)
		}
		go func() {
			for {
				select {
				case <-subscription.Out():
					continue
				case <-subscription.Canceled():
					return
				}
			}
		}()
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		events := []abci.Event{
			{
				Type:       "abci.Account",
				Attributes: []abci.EventAttribute{{Key: "Owner", Value: "Ivan"}},
			},
			{
				Type:       "abci.Invoices",
				Attributes: []abci.EventAttribute{{Key: "Number", Value: string(rune(i))}},
			},
		}

		require.NoError(b, s.PublishWithEvents(ctx, "Gamora", events))
	}
}

func benchmarkNClientsOneQuery(n int, b *testing.B) {
	s := pubsub.NewServer()
	err := s.Start()
	require.NoError(b, err)
	b.Cleanup(func() {
		if err := s.Stop(); err != nil {
			b.Error(err)
		}
	})

	ctx := context.Background()
	q := query.MustParse("abci.Account.Owner = 'Ivan' AND abci.Invoices.Number = 1")
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("clientID-%d", i+1)
		subscription, err := s.Subscribe(ctx, id, q)
		if err != nil {
			b.Fatal(err)
		}
		go func() {
			for {
				select {
				case <-subscription.Out():
					continue
				case <-subscription.Canceled():
					return
				}
			}
		}()
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		events := []abci.Event{
			{
				Type:       "abci.Account",
				Attributes: []abci.EventAttribute{{Key: "Owner", Value: "Ivan"}},
			},
			{
				Type:       "abci.Invoices",
				Attributes: []abci.EventAttribute{{Key: "Number", Value: "1"}},
			},
		}

		require.NoError(b, s.PublishWithEvents(ctx, "Gamora", events))
	}
}

// HELPERS

func assertReceive(t *testing.T, expected interface{}, ch <-chan pubsub.Message, msgAndArgs ...interface{}) {
	select {
	case actual := <-ch:
		require.Equal(t, expected, actual.Data(), msgAndArgs...)
	case <-time.After(1 * time.Second):
		t.Errorf("expected to receive %v from the channel, got nothing after 1s", expected)
		debug.PrintStack()
	}
}

func assertCanceled(t *testing.T, subscription *pubsub.Subscription, err error) {
	_, ok := <-subscription.Canceled()
	require.False(t, ok)
	require.Equal(t, err, subscription.Err())
}
