package types

import (
	"context"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"math/rand"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	tmrand "github.com/tendermint/tendermint/libs/rand"
)

func TestEventBusPublishEventTx(t *testing.T) {
	eventBus := NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	tx := Tx("foo")
	result := abci.ResponseDeliverTx{
		Data: []byte("bar"),
		Events: []abci.Event{
			{Type: "testType", Attributes: []abci.EventAttribute{{Key: []byte("baz"), Value: []byte("1")}}},
		},
	}

	// PublishEventTx adds 3 composite keys, so the query below should work
	query := fmt.Sprintf("tm.event='Tx' AND tx.height=1 AND tx.hash='%X' AND testType.baz=1", tx.Hash())
	txsSub, err := eventBus.Subscribe(context.Background(), "test", tmquery.MustParse(query))
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		msg := <-txsSub.Out()
		edt := msg.Data().(EventDataTx)
		assert.Equal(t, int64(1), edt.Height)
		assert.Equal(t, uint32(0), edt.Index)
		assert.EqualValues(t, tx, edt.Tx)
		assert.Equal(t, result, edt.Result)
		close(done)
	}()

	err = eventBus.PublishEventTx(EventDataTx{abci.TxResult{
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
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	block := MakeBlock(0, 0, nil, []Tx{}, nil, []Evidence{})
	resultBeginBlock := abci.ResponseBeginBlock{
		Events: []abci.Event{
			{Type: "testType", Attributes: []abci.EventAttribute{{Key: []byte("baz"), Value: []byte("1")}}},
		},
	}
	resultEndBlock := abci.ResponseEndBlock{
		Events: []abci.Event{
			{Type: "testType", Attributes: []abci.EventAttribute{{Key: []byte("foz"), Value: []byte("2")}}},
		},
	}

	// PublishEventNewBlock adds the tm.event compositeKey, so the query below should work
	query := "tm.event='NewBlock' AND testType.baz=1 AND testType.foz=2"
	blocksSub, err := eventBus.Subscribe(context.Background(), "test", tmquery.MustParse(query))
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		msg := <-blocksSub.Out()
		edt := msg.Data().(EventDataNewBlock)
		assert.Equal(t, block, edt.Block)
		assert.Equal(t, resultBeginBlock, edt.ResultBeginBlock)
		assert.Equal(t, resultEndBlock, edt.ResultEndBlock)
		close(done)
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

func TestEventBusPublishEventTxDuplicateKeys(t *testing.T) {
	eventBus := NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	tx := Tx("foo")
	result := abci.ResponseDeliverTx{
		Data: []byte("bar"),
		Events: []abci.Event{
			{
				Type: "transfer",
				Attributes: []abci.EventAttribute{
					{Key: []byte("sender"), Value: []byte("foo")},
					{Key: []byte("recipient"), Value: []byte("bar")},
					{Key: []byte("amount"), Value: []byte("5")},
				},
			},
			{
				Type: "transfer",
				Attributes: []abci.EventAttribute{
					{Key: []byte("sender"), Value: []byte("baz")},
					{Key: []byte("recipient"), Value: []byte("cat")},
					{Key: []byte("amount"), Value: []byte("13")},
				},
			},
			{
				Type: "withdraw.rewards",
				Attributes: []abci.EventAttribute{
					{Key: []byte("address"), Value: []byte("bar")},
					{Key: []byte("source"), Value: []byte("iceman")},
					{Key: []byte("amount"), Value: []byte("33")},
				},
			},
		},
	}

	testCases := []struct {
		query         string
		expectResults bool
	}{
		{
			"tm.event='Tx' AND tx.height=1 AND transfer.sender='DoesNotExist'",
			false,
		},
		{
			"tm.event='Tx' AND tx.height=1 AND transfer.sender='foo'",
			true,
		},
		{
			"tm.event='Tx' AND tx.height=1 AND transfer.sender='baz'",
			true,
		},
		{
			"tm.event='Tx' AND tx.height=1 AND transfer.sender='foo' AND transfer.sender='baz'",
			true,
		},
		{
			"tm.event='Tx' AND tx.height=1 AND transfer.sender='foo' AND transfer.sender='DoesNotExist'",
			false,
		},
	}

	for i, tc := range testCases {
		sub, err := eventBus.Subscribe(context.Background(), fmt.Sprintf("client-%d", i), tmquery.MustParse(tc.query))
		require.NoError(t, err)

		done := make(chan struct{})

		go func() {
			select {
			case msg := <-sub.Out():
				data := msg.Data().(EventDataTx)
				assert.Equal(t, int64(1), data.Height)
				assert.Equal(t, uint32(0), data.Index)
				assert.EqualValues(t, tx, data.Tx)
				assert.Equal(t, result, data.Result)
				close(done)
			case <-time.After(1 * time.Second):
				return
			}
		}()

		err = eventBus.PublishEventTx(EventDataTx{abci.TxResult{
			Height: 1,
			Index:  0,
			Tx:     tx,
			Result: result,
		}})
		assert.NoError(t, err)

		select {
		case <-done:
			if !tc.expectResults {
				require.Fail(t, "unexpected transaction result(s) from subscription")
			}
		case <-time.After(1 * time.Second):
			if tc.expectResults {
				require.Fail(t, "failed to receive a transaction after 1 second")
			}
		}
	}
}

func TestEventBusPublishEventNewBlockHeader(t *testing.T) {
	eventBus := NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	block := MakeBlock(0, 0, nil, []Tx{}, nil, []Evidence{})
	resultBeginBlock := abci.ResponseBeginBlock{
		Events: []abci.Event{
			{Type: "testType", Attributes: []abci.EventAttribute{{Key: []byte("baz"), Value: []byte("1")}}},
		},
	}
	resultEndBlock := abci.ResponseEndBlock{
		Events: []abci.Event{
			{Type: "testType", Attributes: []abci.EventAttribute{{Key: []byte("foz"), Value: []byte("2")}}},
		},
	}

	// PublishEventNewBlockHeader adds the tm.event compositeKey, so the query below should work
	query := "tm.event='NewBlockHeader' AND testType.baz=1 AND testType.foz=2"
	headersSub, err := eventBus.Subscribe(context.Background(), "test", tmquery.MustParse(query))
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		msg := <-headersSub.Out()
		edt := msg.Data().(EventDataNewBlockHeader)
		assert.Equal(t, block.Header, edt.Header)
		assert.Equal(t, resultBeginBlock, edt.ResultBeginBlock)
		assert.Equal(t, resultEndBlock, edt.ResultEndBlock)
		close(done)
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

func TestEventBusPublishEventNewEvidence(t *testing.T) {
	eventBus := NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	ev := NewMockDuplicateVoteEvidence(1, time.Now(), "test-chain-id", btcjson.LLMQType_5_60, crypto.RandQuorumHash())

	query := "tm.event='NewEvidence'"
	evSub, err := eventBus.Subscribe(context.Background(), "test", tmquery.MustParse(query))
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		msg := <-evSub.Out()
		edt := msg.Data().(EventDataNewEvidence)
		assert.Equal(t, ev, edt.Evidence)
		assert.Equal(t, int64(4), edt.Height)
		close(done)
	}()

	err = eventBus.PublishEventNewEvidence(EventDataNewEvidence{
		Evidence: ev,
		Height:   4,
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
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	const numEventsExpected = 14

	sub, err := eventBus.Subscribe(context.Background(), "test", tmquery.Empty{}, numEventsExpected)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		numEvents := 0
		for range sub.Out() {
			numEvents++
			if numEvents >= numEventsExpected {
				close(done)
				return
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
		bm := bm
		b.Run(bm.name, func(b *testing.B) {
			benchmarkEventBus(bm.numClients, bm.randQueries, bm.randEvents, b)
		})
	}
}

func benchmarkEventBus(numClients int, randQueries bool, randEvents bool, b *testing.B) {
	// for random* functions
	rand.Seed(time.Now().Unix())

	eventBus := NewEventBusWithBufferCapacity(0) // set buffer capacity to 0 so we are not testing cache
	err := eventBus.Start()
	if err != nil {
		b.Error(err)
	}
	b.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			b.Error(err)
		}
	})

	ctx := context.Background()
	q := EventQueryNewBlock

	for i := 0; i < numClients; i++ {
		if randQueries {
			q = randQuery()
		}
		sub, err := eventBus.Subscribe(ctx, fmt.Sprintf("client-%d", i), q)
		if err != nil {
			b.Fatal(err)
		}
		go func() {
			for {
				select {
				case <-sub.Out():
				case <-sub.Cancelled():
					return
				}
			}
		}()
	}

	eventType := EventNewBlock

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if randEvents {
			eventType = randEvent()
		}

		err := eventBus.Publish(eventType, EventDataString("Gamora"))
		if err != nil {
			b.Error(err)
		}
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
	return events[tmrand.Intn(len(events))]
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
	return queries[tmrand.Intn(len(queries))]
}
