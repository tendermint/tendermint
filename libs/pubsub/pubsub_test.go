package pubsub_test

import (
	"context"
	"fmt"
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	subscription, err := s.Subscribe(ctx, clientID, query.Empty{})
	require.NoError(t, err)
	err = s.Publish(ctx, "Ka-Zar")
	require.NoError(t, err)
	assertReceive(t, "Ka-Zar", subscription.Out())

	err = s.Publish(ctx, "Quicksilver")
	require.NoError(t, err)
	assertReceive(t, "Quicksilver", subscription.Out())
}

func TestSubscribeWithOutCapacity(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	assert.Panics(t, func() {
		s.Subscribe(ctx, clientID, query.Empty{}, -1)
		s.Subscribe(ctx, clientID, query.Empty{}, 0)
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
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	subscription, err := s.SubscribeUnbuffered(ctx, clientID, query.Empty{})
	require.NoError(t, err)
	go func() {
		err = s.Publish(ctx, "Ultron")
		require.NoError(t, err)
	}()
	assertReceive(t, "Ultron", subscription.Out())
}

func TestDifferentClients(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	subscription1, err := s.Subscribe(ctx, "client-1", query.MustParse("tm.events.type='NewBlock'"))
	require.NoError(t, err)
	err = s.PublishWithTags(ctx, "Iceman", pubsub.NewTagMap(map[string]string{"tm.events.type": "NewBlock"}))
	require.NoError(t, err)
	assertReceive(t, "Iceman", subscription1.Out())

	subscription2, err := s.Subscribe(ctx, "client-2", query.MustParse("tm.events.type='NewBlock' AND abci.account.name='Igor'"))
	require.NoError(t, err)
	err = s.PublishWithTags(ctx, "Ultimo", pubsub.NewTagMap(map[string]string{"tm.events.type": "NewBlock", "abci.account.name": "Igor"}))
	require.NoError(t, err)
	assertReceive(t, "Ultimo", subscription1.Out())
	assertReceive(t, "Ultimo", subscription2.Out())

	subscription3, err := s.Subscribe(ctx, "client-3", query.MustParse("tm.events.type='NewRoundStep' AND abci.account.name='Igor' AND abci.invoice.number = 10"))
	require.NoError(t, err)
	err = s.PublishWithTags(ctx, "Valeria Richards", pubsub.NewTagMap(map[string]string{"tm.events.type": "NewRoundStep"}))
	require.NoError(t, err)
	assert.Zero(t, len(subscription3.Out()))
}

func TestClientSubscribesTwice(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	q := query.MustParse("tm.events.type='NewBlock'")

	subscription1, err := s.Subscribe(ctx, clientID, q)
	require.NoError(t, err)
	err = s.PublishWithTags(ctx, "Goblin Queen", pubsub.NewTagMap(map[string]string{"tm.events.type": "NewBlock"}))
	require.NoError(t, err)
	assertReceive(t, "Goblin Queen", subscription1.Out())

	subscription2, err := s.Subscribe(ctx, clientID, q)
	require.Error(t, err)
	require.Nil(t, subscription2)

	err = s.PublishWithTags(ctx, "Spider-Man", pubsub.NewTagMap(map[string]string{"tm.events.type": "NewBlock"}))
	require.NoError(t, err)
	assertReceive(t, "Spider-Man", subscription1.Out())
}

func TestUnsubscribe(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	subscription, err := s.Subscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"))
	require.NoError(t, err)
	err = s.Unsubscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"))
	require.NoError(t, err)

	err = s.Publish(ctx, "Nick Fury")
	require.NoError(t, err)
	assert.Zero(t, len(subscription.Out()), "Should not receive anything after Unsubscribe")

	_, ok := <-subscription.Cancelled()
	assert.False(t, ok)

	assert.Equal(t, pubsub.ErrUnsubscribed, subscription.Err())
}

func TestClientUnsubscribesTwice(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	_, err := s.Subscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"))
	require.NoError(t, err)
	err = s.Unsubscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"))
	require.NoError(t, err)

	err = s.Unsubscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"))
	assert.Equal(t, pubsub.ErrSubscriptionNotFound, err)
	err = s.UnsubscribeAll(ctx, clientID)
	assert.Equal(t, pubsub.ErrSubscriptionNotFound, err)
}

func TestResubscribe(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	subscription, err := s.Subscribe(ctx, clientID, query.Empty{})
	require.NoError(t, err)
	err = s.Unsubscribe(ctx, clientID, query.Empty{})
	require.NoError(t, err)
	subscription, err = s.Subscribe(ctx, clientID, query.Empty{})
	require.NoError(t, err)

	err = s.Publish(ctx, "Cable")
	require.NoError(t, err)
	assertReceive(t, "Cable", subscription.Out())
}

func TestUnsubscribeAll(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	subscription1, err := s.Subscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"))
	require.NoError(t, err)
	subscription2, err := s.Subscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlockHeader'"))
	require.NoError(t, err)

	err = s.UnsubscribeAll(ctx, clientID)
	require.NoError(t, err)

	err = s.Publish(ctx, "Nick Fury")
	require.NoError(t, err)
	assert.Zero(t, len(subscription1.Out()), "Should not receive anything after UnsubscribeAll")
	assert.Zero(t, len(subscription2.Out()), "Should not receive anything after UnsubscribeAll")

	_, ok := <-subscription1.Cancelled()
	assert.False(t, ok)
	assert.Equal(t, pubsub.ErrUnsubscribed, subscription1.Err())
	_, ok = <-subscription2.Cancelled()
	assert.False(t, ok)
	assert.Equal(t, pubsub.ErrUnsubscribed, subscription2.Err())
}

func TestBufferCapacity(t *testing.T) {
	s := pubsub.NewServer(pubsub.BufferCapacity(2))
	s.SetLogger(log.TestingLogger())

	assert.Equal(t, 2, s.BufferCapacity())

	ctx := context.Background()
	err := s.Publish(ctx, "Nighthawk")
	require.NoError(t, err)
	err = s.Publish(ctx, "Sage")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	err = s.Publish(ctx, "Ironclad")
	if assert.Error(t, err) {
		assert.Equal(t, context.DeadlineExceeded, err)
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
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	for i := 0; i < n; i++ {
		subscription, err := s.Subscribe(ctx, clientID, query.MustParse(fmt.Sprintf("abci.Account.Owner = 'Ivan' AND abci.Invoices.Number = %d", i)))
		if err != nil {
			b.Fatal(err)
		}
		go func() {
			for {
				select {
				case <-subscription.Out():
					continue
				case <-subscription.Cancelled():
					return
				}
			}
		}()
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.PublishWithTags(ctx, "Gamora", pubsub.NewTagMap(map[string]string{"abci.Account.Owner": "Ivan", "abci.Invoices.Number": string(i)}))
	}
}

func benchmarkNClientsOneQuery(n int, b *testing.B) {
	s := pubsub.NewServer()
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	q := query.MustParse("abci.Account.Owner = 'Ivan' AND abci.Invoices.Number = 1")
	for i := 0; i < n; i++ {
		subscription, err := s.Subscribe(ctx, clientID, q)
		if err != nil {
			b.Fatal(err)
		}
		go func() {
			for {
				select {
				case <-subscription.Out():
					continue
				case <-subscription.Cancelled():
					return
				}
			}
		}()
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.PublishWithTags(ctx, "Gamora", pubsub.NewTagMap(map[string]string{"abci.Account.Owner": "Ivan", "abci.Invoices.Number": "1"}))
	}
}

///////////////////////////////////////////////////////////////////////////////
/// HELPERS
///////////////////////////////////////////////////////////////////////////////

func assertReceive(t *testing.T, expected interface{}, ch <-chan pubsub.MsgAndTags, msgAndArgs ...interface{}) {
	select {
	case actual := <-ch:
		assert.Equal(t, expected, actual.Msg, msgAndArgs...)
	case <-time.After(1 * time.Second):
		t.Errorf("Expected to receive %v from the channel, got nothing after 1s", expected)
		debug.PrintStack()
	}
}
