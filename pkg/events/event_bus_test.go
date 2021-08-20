package events_test

import (
	"context"
	"fmt"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/pkg/block"
	"github.com/tendermint/tendermint/pkg/events"
	"github.com/tendermint/tendermint/pkg/evidence"
	"github.com/tendermint/tendermint/pkg/mempool"
	"github.com/tendermint/tendermint/pkg/metadata"
)

func TestEventBusPublishEventTx(t *testing.T) {
	eventBus := events.NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	tx := mempool.Tx("foo")
	result := abci.ResponseDeliverTx{
		Data: []byte("bar"),
		Events: []abci.Event{
			{Type: "testType", Attributes: []abci.EventAttribute{{Key: "baz", Value: "1"}}},
		},
	}

	// PublishEventTx adds 3 composite keys, so the query below should work
	query := fmt.Sprintf("tm.event='Tx' AND tx.height=1 AND tx.hash='%X' AND testType.baz=1", tx.Hash())
	txsSub, err := eventBus.Subscribe(context.Background(), "test", tmquery.MustParse(query))
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		msg := <-txsSub.Out()
		edt := msg.Data().(events.EventDataTx)
		assert.Equal(t, int64(1), edt.Height)
		assert.Equal(t, uint32(0), edt.Index)
		assert.EqualValues(t, tx, edt.Tx)
		assert.Equal(t, result, edt.Result)
		close(done)
	}()

	err = eventBus.PublishEventTx(events.EventDataTx{abci.TxResult{
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
	eventBus := events.NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	block := block.MakeBlock(0, []mempool.Tx{}, nil, []evidence.Evidence{})
	blockID := metadata.BlockID{Hash: block.Hash(), PartSetHeader: block.MakePartSet(metadata.BlockPartSizeBytes).Header()}
	resultBeginBlock := abci.ResponseBeginBlock{
		Events: []abci.Event{
			{Type: "testType", Attributes: []abci.EventAttribute{{Key: "baz", Value: "1"}}},
		},
	}
	resultEndBlock := abci.ResponseEndBlock{
		Events: []abci.Event{
			{Type: "testType", Attributes: []abci.EventAttribute{{Key: "foz", Value: "2"}}},
		},
	}

	// PublishEventNewBlock adds the tm.event compositeKey, so the query below should work
	query := "tm.event='NewBlock' AND testType.baz=1 AND testType.foz=2"
	blocksSub, err := eventBus.Subscribe(context.Background(), "test", tmquery.MustParse(query))
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		msg := <-blocksSub.Out()
		edt := msg.Data().(events.EventDataNewBlock)
		assert.Equal(t, block, edt.Block)
		assert.Equal(t, blockID, edt.BlockID)
		assert.Equal(t, resultBeginBlock, edt.ResultBeginBlock)
		assert.Equal(t, resultEndBlock, edt.ResultEndBlock)
		close(done)
	}()

	err = eventBus.PublishEventNewBlock(events.EventDataNewBlock{
		Block:            block,
		BlockID:          blockID,
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
	eventBus := events.NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	tx := mempool.Tx("foo")
	result := abci.ResponseDeliverTx{
		Data: []byte("bar"),
		Events: []abci.Event{
			{
				Type: "transfer",
				Attributes: []abci.EventAttribute{
					{Key: "sender", Value: "foo"},
					{Key: "recipient", Value: "bar"},
					{Key: "amount", Value: "5"},
				},
			},
			{
				Type: "transfer",
				Attributes: []abci.EventAttribute{
					{Key: "sender", Value: "baz"},
					{Key: "recipient", Value: "cat"},
					{Key: "amount", Value: "13"},
				},
			},
			{
				Type: "withdraw.rewards",
				Attributes: []abci.EventAttribute{
					{Key: "address", Value: "bar"},
					{Key: "source", Value: "iceman"},
					{Key: "amount", Value: "33"},
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
				data := msg.Data().(events.EventDataTx)
				assert.Equal(t, int64(1), data.Height)
				assert.Equal(t, uint32(0), data.Index)
				assert.EqualValues(t, tx, data.Tx)
				assert.Equal(t, result, data.Result)
				close(done)
			case <-time.After(1 * time.Second):
				return
			}
		}()

		err = eventBus.PublishEventTx(events.EventDataTx{abci.TxResult{
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
	eventBus := events.NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	block := block.MakeBlock(0, []mempool.Tx{}, nil, []evidence.Evidence{})
	resultBeginBlock := abci.ResponseBeginBlock{
		Events: []abci.Event{
			{Type: "testType", Attributes: []abci.EventAttribute{{Key: "baz", Value: "1"}}},
		},
	}
	resultEndBlock := abci.ResponseEndBlock{
		Events: []abci.Event{
			{Type: "testType", Attributes: []abci.EventAttribute{{Key: "foz", Value: "2"}}},
		},
	}

	// PublishEventNewBlockHeader adds the tm.event compositeKey, so the query below should work
	query := "tm.event='NewBlockHeader' AND testType.baz=1 AND testType.foz=2"
	headersSub, err := eventBus.Subscribe(context.Background(), "test", tmquery.MustParse(query))
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		msg := <-headersSub.Out()
		edt := msg.Data().(events.EventDataNewBlockHeader)
		assert.Equal(t, block.Header, edt.Header)
		assert.Equal(t, resultBeginBlock, edt.ResultBeginBlock)
		assert.Equal(t, resultEndBlock, edt.ResultEndBlock)
		close(done)
	}()

	err = eventBus.PublishEventNewBlockHeader(events.EventDataNewBlockHeader{
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
	eventBus := events.NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	ev := evidence.NewMockDuplicateVoteEvidence(1, time.Now(), "test-chain-id")

	query := "tm.event='NewEvidence'"
	evSub, err := eventBus.Subscribe(context.Background(), "test", tmquery.MustParse(query))
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		msg := <-evSub.Out()
		edt := msg.Data().(events.EventDataNewEvidence)
		assert.Equal(t, ev, edt.Evidence)
		assert.Equal(t, int64(4), edt.Height)
		close(done)
	}()

	err = eventBus.PublishEventNewEvidence(events.EventDataNewEvidence{
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
	eventBus := events.NewEventBus()
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

	err = eventBus.Publish(events.EventNewBlockHeaderValue, events.EventDataNewBlockHeader{})
	require.NoError(t, err)
	err = eventBus.PublishEventNewBlock(events.EventDataNewBlock{})
	require.NoError(t, err)
	err = eventBus.PublishEventNewBlockHeader(events.EventDataNewBlockHeader{})
	require.NoError(t, err)
	err = eventBus.PublishEventVote(events.EventDataVote{})
	require.NoError(t, err)
	err = eventBus.PublishEventNewRoundStep(events.EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventTimeoutPropose(events.EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventTimeoutWait(events.EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventNewRound(events.EventDataNewRound{})
	require.NoError(t, err)
	err = eventBus.PublishEventCompleteProposal(events.EventDataCompleteProposal{})
	require.NoError(t, err)
	err = eventBus.PublishEventPolka(events.EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventUnlock(events.EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventRelock(events.EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventLock(events.EventDataRoundState{})
	require.NoError(t, err)
	err = eventBus.PublishEventValidatorSetUpdates(events.EventDataValidatorSetUpdates{})
	require.NoError(t, err)
	err = eventBus.PublishEventBlockSyncStatus(events.EventDataBlockSyncStatus{})
	require.NoError(t, err)
	err = eventBus.PublishEventStateSyncStatus(events.EventDataStateSyncStatus{})
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
	mrand.Seed(time.Now().Unix())

	eventBus := events.NewEventBusWithBufferCapacity(0) // set buffer capacity to 0 so we are not testing cache
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
	q := events.EventQueryNewBlock

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
				case <-sub.Canceled():
					return
				}
			}
		}()
	}

	eventValue := events.EventNewBlockValue

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if randEvents {
			eventValue = randEventValue()
		}

		err := eventBus.Publish(eventValue, events.EventDataString("Gamora"))
		if err != nil {
			b.Error(err)
		}
	}
}

var allEvents = []string{
	events.EventNewBlockValue,
	events.EventNewBlockHeaderValue,
	events.EventNewRoundValue,
	events.EventNewRoundStepValue,
	events.EventTimeoutProposeValue,
	events.EventCompleteProposalValue,
	events.EventPolkaValue,
	events.EventUnlockValue,
	events.EventLockValue,
	events.EventRelockValue,
	events.EventTimeoutWaitValue,
	events.EventVoteValue,
	events.EventBlockSyncStatusValue,
	events.EventStateSyncStatusValue,
}

func randEventValue() string {

	return allEvents[mrand.Intn(len(allEvents))]
}

var queries = []tmpubsub.Query{
	events.EventQueryNewBlock,
	events.EventQueryNewBlockHeader,
	events.EventQueryNewRound,
	events.EventQueryNewRoundStep,
	events.EventQueryTimeoutPropose,
	events.EventQueryCompleteProposal,
	events.EventQueryPolka,
	events.EventQueryUnlock,
	events.EventQueryLock,
	events.EventQueryRelock,
	events.EventQueryTimeoutWait,
	events.EventQueryVote,
	events.EventQueryBlockSyncStatus,
	events.EventQueryStateSyncStatus,
}

func randQuery() tmpubsub.Query {
	return queries[mrand.Intn(len(queries))]
}
