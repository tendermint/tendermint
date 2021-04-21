package kvsink

import (
	"context"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

var _ indexer.EventSink = (*KVEventSink)(nil)

type KVEventSink struct {
	store dbm.DB
}

func NewKVEventSink(store dbm.DB) (*KVEventSink, error) {
	return &KVEventSink{store: store}, nil
}

func (kv *KVEventSink) IndexBlockEvents(types.EventDataNewBlockHeader) error {
	return nil
}

func (kv *KVEventSink) IndexTxEvents(*abci.TxResult) error {
	return nil
}

func (kv *KVEventSink) SearchBlockEvents(context.Context, *query.Query) ([]int64, error) {
	return nil, nil
}

func (kv *KVEventSink) SearchTxEvents(context.Context, *query.Query) ([]*abci.TxResult, error) {
	return nil, nil
}

func (kv *KVEventSink) GetTxByHash([]byte) (*abci.TxResult, error) {
	return nil, nil
}

func (kv *KVEventSink) HasBlock(int64) (bool, error) {
	return false, nil
}
