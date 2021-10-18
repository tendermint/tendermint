package null

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/types"
)

func TestNullEventSink(t *testing.T) {
	nullIndexer := NewEventSink()

	assert.Nil(t, nullIndexer.IndexTxEvents(nil))
	assert.Nil(t, nullIndexer.IndexBlockEvents(types.EventDataNewBlockHeader{}))
	val1, err1 := nullIndexer.SearchBlockEvents(context.TODO(), nil)
	assert.Nil(t, val1)
	assert.Nil(t, err1)
	val2, err2 := nullIndexer.SearchTxEvents(context.TODO(), nil)
	assert.Nil(t, val2)
	assert.Nil(t, err2)
	val3, err3 := nullIndexer.GetTxByHash(nil)
	assert.Nil(t, val3)
	assert.Nil(t, err3)
	val4, err4 := nullIndexer.HasBlock(0)
	assert.False(t, val4)
	assert.Nil(t, err4)
}

func TestType(t *testing.T) {
	nullIndexer := NewEventSink()
	assert.Equal(t, indexer.NULL, nullIndexer.Type())
}

func TestStop(t *testing.T) {
	nullIndexer := NewEventSink()
	assert.Nil(t, nullIndexer.Stop())
}
