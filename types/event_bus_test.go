package types

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/abci/types"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	cmn "github.com/tendermint/tmlibs/common"
)

func TestEventBusPublishEventTx(t *testing.T) {
	eventBus := NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	defer eventBus.Stop()

	tx := Tx("foo")
	result := abci.ResponseDeliverTx{Data: []byte("bar"), Tags: []cmn.KVPair{{[]byte("baz"), []byte("1")}}, Fee: cmn.KI64Pair{Key: []uint8{}, Value: 0}}

	txEventsCh := make(chan interface{})

	// PublishEventTx adds all these 3 tags, so the query below should work
	query := fmt.Sprintf("tm.event='Tx' AND tx.height=1 AND tx.hash='%X' AND baz=1", tx.Hash())
	err = eventBus.Subscribe(context.Background(), "test", tmquery.MustParse(query), txEventsCh)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		for e := range txEventsCh {
			edt := e.(EventDataTx)
			assert.Equal(t, int64(1), edt.Height)
			assert.Equal(t, uint32(0), edt.Index)
			assert.Equal(t, tx, edt.Tx)
			assert.Equal(t, result, edt.Result)
			close(done)
		}
	}()

	err = eventBus.PublishEventTx(EventDataTx{TxResult{
		Height: 1,
		Index:  0,
		Tx:     tx,
		Result: result,
	}})
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("did not receive a transaction after 1 sec.")
	}
}

func BenchmarkEventBus(b *testing.B) {
	benchmarks := []struct {
		name        string
		numClients  int
		randQueries bool
		randEvents  bool
	}{
		{"10Clients1Query1Event", 10, false, false},
		{"100Clients", 100, false, false},
		{"1000Clients", 1000, false, false},

		{"10ClientsRandQueries1Event", 10, true, false},
		{"100Clients", 100, true, false},
		{"1000Clients", 1000, true, false},

		{"10ClientsRandQueriesRandEvents", 10, true, true},
		{"100Clients", 100, true, true},
		{"1000Clients", 1000, true, true},

		{"10Clients1QueryRandEvents", 10, false, true},
		{"100Clients", 100, false, true},
		{"1000Clients", 1000, false, true},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			benchmarkEventBus(bm.numClients, bm.randQueries, bm.randEvents, b)
		})
	}
}

func benchmarkEventBus(numClients int, randQueries bool, randEvents bool, b *testing.B) {
	// for random* functions
	rand.Seed(time.Now().Unix())

	eventBus := NewEventBusWithBufferCapacity(0) // set buffer capacity to 0 so we are not testing cache
	eventBus.Start()
	defer eventBus.Stop()

	ctx := context.Background()
	q := EventQueryNewBlock

	for i := 0; i < numClients; i++ {
		ch := make(chan interface{})
		go func() {
			for range ch {
			}
		}()
		if randQueries {
			q = randQuery()
		}
		eventBus.Subscribe(ctx, fmt.Sprintf("client-%d", i), q, ch)
	}

	eventType := EventNewBlock

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if randEvents {
			eventType = randEvent()
		}

		eventBus.Publish(eventType, EventDataString("Gamora"))
	}
}

var events = []string{EventBond,
	EventUnbond,
	EventRebond,
	EventDupeout,
	EventFork,
	EventNewBlock,
	EventNewBlockHeader,
	EventNewRound,
	EventNewRoundStep,
	EventTimeoutPropose,
	EventCompleteProposal,
	EventPolka,
	EventUnlock,
	EventLock,
	EventRelock,
	EventTimeoutWait,
	EventVote}

func randEvent() string {
	return events[rand.Intn(len(events))]
}

var queries = []tmpubsub.Query{EventQueryBond,
	EventQueryUnbond,
	EventQueryRebond,
	EventQueryDupeout,
	EventQueryFork,
	EventQueryNewBlock,
	EventQueryNewBlockHeader,
	EventQueryNewRound,
	EventQueryNewRoundStep,
	EventQueryTimeoutPropose,
	EventQueryCompleteProposal,
	EventQueryPolka,
	EventQueryUnlock,
	EventQueryLock,
	EventQueryRelock,
	EventQueryTimeoutWait,
	EventQueryVote}

func randQuery() tmpubsub.Query {
	return queries[rand.Intn(len(queries))]
}
