package null

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/types"
)

func TestNullEventSink(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nullIndexer := NewEventSink()

	assert.Nil(t, nullIndexer.IndexTxEvents(nil))
	assert.Nil(t, nullIndexer.IndexBlockEvents(types.EventDataNewBlockHeader{}))
	val1, err1 := nullIndexer.SearchBlockEvents(ctx, nil)
	assert.Nil(t, val1)
	assert.NoError(t, err1)
	val2, err2 := nullIndexer.SearchTxEvents(ctx, nil)
	assert.Nil(t, val2)
	assert.NoError(t, err2)
	val3, err3 := nullIndexer.GetTxByHash(nil)
	assert.Nil(t, val3)
	assert.NoError(t, err3)
	val4, err4 := nullIndexer.HasBlock(0)
	assert.False(t, val4)
	assert.NoError(t, err4)
}

func TestType(t *testing.T) {
	nullIndexer := NewEventSink()
	assert.Equal(t, indexer.NULL, nullIndexer.Type())
}

func TestStop(t *testing.T) {
	nullIndexer := NewEventSink()
	assert.Nil(t, nullIndexer.Stop())
}
