package indexer

import (
	"context"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/types"
)

type EventSinkType string

const (
	NULL EventSinkType = "null"
	KV   EventSinkType = "kv"
	PSQL EventSinkType = "psql"
)

type EventSink interface {
	IndexBlockEvents(types.EventDataNewBlockHeader) error
	IndexTxEvents(*abci.TxResult) error

	SearchBlockEvents(context.Context, *query.Query) ([]int64, error)
	SearchTxEvents(context.Context, *query.Query) ([]*abci.TxResult, error)

	GetTxByHash([]byte) (*abci.TxResult, error)
	HasBlock(int64) (bool, error)

	Type() EventSinkType

	Stop() error
}
