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
	ch := make(chan interface{}, 1)
	err := s.Subscribe(ctx, clientID, query.Empty{}, ch)
	require.NoError(t, err)
	err = s.Publish(ctx, "Ka-Zar")
	require.NoError(t, err)
	assertReceive(t, "Ka-Zar", ch)

	err = s.Publish(ctx, "Quicksilver")
	require.NoError(t, err)
	assertReceive(t, "Quicksilver", ch)
}

func TestDifferentClients(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	ch1 := make(chan interface{}, 1)
	err := s.Subscribe(ctx, "client-1", query.MustParse("tm.events.type='NewBlock'"), ch1)
	require.NoError(t, err)
	err = s.PublishWithTags(ctx, "Iceman", pubsub.NewTagMap(map[string]string{"tm.events.type": "NewBlock"}))
	require.NoError(t, err)
	assertReceive(t, "Iceman", ch1)

	ch2 := make(chan interface{}, 1)
	err = s.Subscribe(ctx, "client-2", query.MustParse("tm.events.type='NewBlock' AND abci.account.name='Igor'"), ch2)
	require.NoError(t, err)
	err = s.PublishWithTags(ctx, "Ultimo", pubsub.NewTagMap(map[string]string{"tm.events.type": "NewBlock", "abci.account.name": "Igor"}))
	require.NoError(t, err)
	assertReceive(t, "Ultimo", ch1)
	assertReceive(t, "Ultimo", ch2)

	ch3 := make(chan interface{}, 1)
	err = s.Subscribe(ctx, "client-3", query.MustParse("tm.events.type='NewRoundStep' AND abci.account.name='Igor' AND abci.invoice.number = 10"), ch3)
	require.NoError(t, err)
	err = s.PublishWithTags(ctx, "Valeria Richards", pubsub.NewTagMap(map[string]string{"tm.events.type": "NewRoundStep"}))
	require.NoError(t, err)
	assert.Zero(t, len(ch3))
}

func TestClientSubscribesTwice(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	q := query.MustParse("tm.events.type='NewBlock'")

	ch1 := make(chan interface{}, 1)
	err := s.Subscribe(ctx, clientID, q, ch1)
	require.NoError(t, err)
	err = s.PublishWithTags(ctx, "Goblin Queen", pubsub.NewTagMap(map[string]string{"tm.events.type": "NewBlock"}))
	require.NoError(t, err)
	assertReceive(t, "Goblin Queen", ch1)

	ch2 := make(chan interface{}, 1)
	err = s.Subscribe(ctx, clientID, q, ch2)
	require.Error(t, err)

	err = s.PublishWithTags(ctx, "Spider-Man", pubsub.NewTagMap(map[string]string{"tm.events.type": "NewBlock"}))
	require.NoError(t, err)
	assertReceive(t, "Spider-Man", ch1)
}

func TestUnsubscribe(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	ch := make(chan interface{})
	err := s.Subscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"), ch)
	require.NoError(t, err)
	err = s.Unsubscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"))
	require.NoError(t, err)

	err = s.Publish(ctx, "Nick Fury")
	require.NoError(t, err)
	assert.Zero(t, len(ch), "Should not receive anything after Unsubscribe")

	_, ok := <-ch
	assert.False(t, ok)
}

func TestClientUnsubscribesTwice(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	ch := make(chan interface{})
	err := s.Subscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"), ch)
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
	ch := make(chan interface{})
	err := s.Subscribe(ctx, clientID, query.Empty{}, ch)
	require.NoError(t, err)
	err = s.Unsubscribe(ctx, clientID, query.Empty{})
	require.NoError(t, err)
	ch = make(chan interface{})
	err = s.Subscribe(ctx, clientID, query.Empty{}, ch)
	require.NoError(t, err)

	err = s.Publish(ctx, "Cable")
	require.NoError(t, err)
	assertReceive(t, "Cable", ch)
}

func TestUnsubscribeAll(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	ch1, ch2 := make(chan interface{}, 1), make(chan interface{}, 1)
	err := s.Subscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlock'"), ch1)
	require.NoError(t, err)
	err = s.Subscribe(ctx, clientID, query.MustParse("tm.events.type='NewBlockHeader'"), ch2)
	require.NoError(t, err)

	err = s.UnsubscribeAll(ctx, clientID)
	require.NoError(t, err)

	err = s.Publish(ctx, "Nick Fury")
	require.NoError(t, err)
	assert.Zero(t, len(ch1), "Should not receive anything after UnsubscribeAll")
	assert.Zero(t, len(ch2), "Should not receive anything after UnsubscribeAll")

	_, ok := <-ch1
	assert.False(t, ok)
	_, ok = <-ch2
	assert.False(t, ok)
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
		ch := make(chan interface{})
		go func() {
			for range ch {
			}
		}()
		s.Subscribe(ctx, clientID, query.MustParse(fmt.Sprintf("abci.Account.Owner = 'Ivan' AND abci.Invoices.Number = %d", i)), ch)
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
		ch := make(chan interface{})
		go func() {
			for range ch {
			}
		}()
		s.Subscribe(ctx, clientID, q, ch)
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

func assertReceive(t *testing.T, expected interface{}, ch <-chan interface{}, msgAndArgs ...interface{}) {
	select {
	case actual := <-ch:
		if actual != nil {
			assert.Equal(t, expected, actual, msgAndArgs...)
		}
	case <-time.After(1 * time.Second):
		t.Errorf("Expected to receive %v from the channel, got nothing after 1s", expected)
		debug.PrintStack()
	}
}
