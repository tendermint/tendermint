package pubsub_test

import (
	"fmt"
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tmlibs/log"
	"github.com/tendermint/tmlibs/pubsub"
	"github.com/tendermint/tmlibs/pubsub/query"
)

const (
	clientID = "test-client"
)

func TestSubscribe(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ch := make(chan interface{}, 1)
	s.Subscribe(clientID, query.Empty{}, ch)
	err := s.Publish("Ka-Zar")
	require.NoError(t, err)
	assertReceive(t, "Ka-Zar", ch)

	err = s.Publish("Quicksilver")
	require.NoError(t, err)
	assertReceive(t, "Quicksilver", ch)
}

func TestDifferentClients(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()
	ch1 := make(chan interface{}, 1)
	s.Subscribe("client-1", query.MustParse("tm.events.type=NewBlock"), ch1)
	err := s.PublishWithTags("Iceman", map[string]interface{}{"tm.events.type": "NewBlock"})
	require.NoError(t, err)
	assertReceive(t, "Iceman", ch1)

	ch2 := make(chan interface{}, 1)
	s.Subscribe("client-2", query.MustParse("tm.events.type=NewBlock AND abci.account.name=Igor"), ch2)
	err = s.PublishWithTags("Ultimo", map[string]interface{}{"tm.events.type": "NewBlock", "abci.account.name": "Igor"})
	require.NoError(t, err)
	assertReceive(t, "Ultimo", ch1)
	assertReceive(t, "Ultimo", ch2)

	ch3 := make(chan interface{}, 1)
	s.Subscribe("client-3", query.MustParse("tm.events.type=NewRoundStep AND abci.account.name=Igor AND abci.invoice.number = 10"), ch3)
	err = s.PublishWithTags("Valeria Richards", map[string]interface{}{"tm.events.type": "NewRoundStep"})
	require.NoError(t, err)
	assert.Zero(t, len(ch3))
}

func TestClientResubscribes(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	q := query.MustParse("tm.events.type=NewBlock")

	ch1 := make(chan interface{}, 1)
	s.Subscribe(clientID, q, ch1)
	err := s.PublishWithTags("Goblin Queen", map[string]interface{}{"tm.events.type": "NewBlock"})
	require.NoError(t, err)
	assertReceive(t, "Goblin Queen", ch1)

	ch2 := make(chan interface{}, 1)
	s.Subscribe(clientID, q, ch2)

	_, ok := <-ch1
	assert.False(t, ok)

	err = s.PublishWithTags("Spider-Man", map[string]interface{}{"tm.events.type": "NewBlock"})
	require.NoError(t, err)
	assertReceive(t, "Spider-Man", ch2)
}

func TestUnsubscribe(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ch := make(chan interface{})
	s.Subscribe(clientID, query.Empty{}, ch)
	s.Unsubscribe(clientID, query.Empty{})

	err := s.Publish("Nick Fury")
	require.NoError(t, err)
	assert.Zero(t, len(ch), "Should not receive anything after Unsubscribe")

	_, ok := <-ch
	assert.False(t, ok)
}

func TestUnsubscribeAll(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ch1, ch2 := make(chan interface{}, 1), make(chan interface{}, 1)
	s.Subscribe(clientID, query.MustParse("tm.events.type=NewBlock"), ch1)
	s.Subscribe(clientID, query.MustParse("tm.events.type=NewBlockHeader"), ch2)

	s.UnsubscribeAll(clientID)

	err := s.Publish("Nick Fury")
	require.NoError(t, err)
	assert.Zero(t, len(ch1), "Should not receive anything after UnsubscribeAll")
	assert.Zero(t, len(ch2), "Should not receive anything after UnsubscribeAll")

	_, ok := <-ch1
	assert.False(t, ok)
	_, ok = <-ch2
	assert.False(t, ok)
}

func TestOverflowStrategyDrop(t *testing.T) {
	s := pubsub.NewServer(pubsub.OverflowStrategyDrop())
	s.SetLogger(log.TestingLogger())

	err := s.Publish("Veda")
	if assert.Error(t, err) {
		assert.Equal(t, pubsub.ErrorOverflow, err)
	}
}

func TestOverflowStrategyWait(t *testing.T) {
	s := pubsub.NewServer(pubsub.OverflowStrategyWait())
	s.SetLogger(log.TestingLogger())

	go func() {
		time.Sleep(1 * time.Second)
		s.Start()
		defer s.Stop()
	}()

	err := s.Publish("Veda")
	assert.NoError(t, err)
}

func TestBufferCapacity(t *testing.T) {
	s := pubsub.NewServer(pubsub.BufferCapacity(2))
	s.SetLogger(log.TestingLogger())

	err := s.Publish("Nighthawk")
	require.NoError(t, err)
	err = s.Publish("Sage")
	require.NoError(t, err)
}

func TestWaitSlowClients(t *testing.T) {
	s := pubsub.NewServer(pubsub.WaitSlowClients())
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ch := make(chan interface{})
	s.Subscribe(clientID, query.Empty{}, ch)
	err := s.Publish("Wonderwoman")
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	assertReceive(t, "Wonderwoman", ch)
}

func TestSkipSlowClients(t *testing.T) {
	s := pubsub.NewServer(pubsub.SkipSlowClients())
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ch := make(chan interface{})
	s.Subscribe(clientID, query.Empty{}, ch)
	err := s.Publish("Cyclops")
	require.NoError(t, err)
	assert.Zero(t, len(ch))
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

	for i := 0; i < n; i++ {
		ch := make(chan interface{})
		go func() {
			for range ch {
			}
		}()
		s.Subscribe(clientID, query.MustParse(fmt.Sprintf("abci.Account.Owner = Ivan AND abci.Invoices.Number = %d", i)), ch)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.PublishWithTags("Gamora", map[string]interface{}{"abci.Account.Owner": "Ivan", "abci.Invoices.Number": i})
	}
}

func benchmarkNClientsOneQuery(n int, b *testing.B) {
	s := pubsub.NewServer()
	s.Start()
	defer s.Stop()

	q := query.MustParse("abci.Account.Owner = Ivan AND abci.Invoices.Number = 1")
	for i := 0; i < n; i++ {
		ch := make(chan interface{})
		go func() {
			for range ch {
			}
		}()
		s.Subscribe(clientID, q, ch)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.PublishWithTags("Gamora", map[string]interface{}{"abci.Account.Owner": "Ivan", "abci.Invoices.Number": 1})
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
