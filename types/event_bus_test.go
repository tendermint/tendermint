package types

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
)

func TestEventBusPublishEventTx(t *testing.T) {
	eventBus := NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	defer eventBus.Stop()

	tx := Tx("foo")
	result := abci.ResponseDeliverTx{Data: []byte("bar"), Tags: []cmn.KVPair{{Key: []byte("baz"), Value: []byte("1")}}}

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

func TestEventBusPublishEventNewBlock(t *testing.T) {
	eventBus := NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	defer eventBus.Stop()

	block := MakeBlock(0, []Tx{}, nil, []Evidence{})
	resultBeginBlock := abci.ResponseBeginBlock{Tags: []cmn.KVPair{{Key: []byte("baz"), Value: []byte("1")}}}
	resultEndBlock := abci.ResponseEndBlock{Tags: []cmn.KVPair{{Key: []byte("foz"), Value: []byte("2")}}}

	txEventsCh := make(chan interface{})

	// PublishEventNewBlock adds the tm.event tag, so the query below should work
	query := "tm.event='NewBlock' AND baz=1 AND foz=2"
	err = eventBus.Subscribe(context.Background(), "test", tmquery.MustParse(query), txEventsCh)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		for e := range txEventsCh {
			edt := e.(EventDataNewBlock)
			assert.Equal(t, block, edt.Block)
			assert.Equal(t, resultBeginBlock, edt.ResultBeginBlock)
			assert.Equal(t, resultEndBlock, edt.ResultEndBlock)
			close(done)
		}
	}()

	err = eventBus.PublishEventNewBlock(EventDataNewBlock{
		Block:            block,
		ResultBeginBlock: resultBeginBlock,
		ResultEndBlock:   resultEndBlock,
	})
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("did not receive a block after 1 sec.")
	}
}

func TestEventBusPublishEventNewBlockHeader(t *testing.T) {
	eventBus := NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	defer eventBus.Stop()

	block := MakeBlock(0, []Tx{}, nil, []Evidence{})
	resultBeginBlock := abci.ResponseBeginBlock{Tags: []cmn.KVPair{{Key: []byte("baz"), Value: []byte("1")}}}
	resultEndBlock := abci.ResponseEndBlock{Tags: []cmn.KVPair{{Key: []byte("foz"), Value: []byte("2")}}}

	txEventsCh := make(chan interface{})

	// PublishEventNewBlockHeader adds the tm.event tag, so the query below should work
	query := "tm.event='NewBlockHeader' AND baz=1 AND foz=2"
	err = eventBus.Subscribe(context.Background(), "test", tmquery.MustParse(query), txEventsCh)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		for e := range txEventsCh {
			edt := e.(EventDataNewBlockHeader)
			assert.Equal(t, block.Header, edt.Header)
			assert.Equal(t, resultBeginBlock, edt.ResultBeginBlock)
			assert.Equal(t, resultEndBlock, edt.ResultEndBlock)
			close(done)
		}
	}()

	err = eventBus.PublishEventNewBlockHeader(EventDataNewBlockHeader{
		Header:           block.Header,
		ResultBeginBlock: resultBeginBlock,
		ResultEndBlock:   resultEndBlock,
	})
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("did not receive a block header after 1 sec.")
	}
}

func TestEventBusPublish(t *testing.T) {
	eventBus := NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	defer eventBus.Stop()

	eventsCh := make(chan interface{})
	err = eventBus.Subscribe(context.Background(), "test", tmquery.Empty{}, eventsCh)
	require.NoError(t, err)

	const numEventsExpected = 14
	done := make(chan struct{})
	go func() {
		numEvents := 0
		for range eventsCh {
			numEvents++
			if numEvents >= numEventsExpected {
				close(done)
			}
		}
	}()

	err = eventBus.Publish(EventNewBlockHeader, EventDataNewBlockHeader{})
	require.NoError(t, err)
	err = eventBus.PublishEventNewBlock(EventDataNewBlock{})
	require.NoError(t, err)
	err = eventBus.PublishEventNewBlockHeader(EventDataNewBlockHeader{})
	require.NoError(t, err)
	err = eventBus.PublishEventVote(EventDataVote{})
	require.NoError(t, err)
	err = eventBus.PublishEventNewRoundStep(EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventTimeoutPropose(EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventTimeoutWait(EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventNewRound(EventDataNewRound{})
	require.NoError(t, err)
	err = eventBus.PublishEventCompleteProposal(EventDataCompleteProposal{})
	require.NoError(t, err)
	err = eventBus.PublishEventPolka(EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventUnlock(EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventRelock(EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventLock(EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventValidatorSetUpdates(EventDataValidatorSetUpdates{})
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("expected to receive %d events after 1 sec.", numEventsExpected)
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
	cmn.Seed(time.Now().Unix())

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

var events = []string{
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
	return events[cmn.RandIntn(len(events))]
}

var queries = []tmpubsub.Query{
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
	return queries[cmn.RandIntn(len(queries))]
}
