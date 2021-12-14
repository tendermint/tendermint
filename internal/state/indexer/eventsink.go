package indexer

import (
	"context"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/pubsub/query"
	"github.com/tendermint/tendermint/types"
)

type EventSinkType string

const (
	NULL EventSinkType = "null"
	KV   EventSinkType = "kv"
	PSQL EventSinkType = "psql"
)

//go:generate ../../../scripts/mockery_generate.sh EventSink

// EventSink interface is defined the APIs for the IndexerService to interact with the data store,
// including the block/transaction indexing and the search functions.
//
// The IndexerService will accept a list of one or more EventSink types. During the OnStart method
// it will call the appropriate APIs on each EventSink to index both block and transaction events.
type EventSink interface {

	// IndexBlockEvents indexes the blockheader.
	IndexBlockEvents(types.EventDataNewBlockHeader) error

	// IndexTxEvents indexes the given result of transactions. To call it with multi transactions,
	// must guarantee the index of given transactions are in order.
	IndexTxEvents([]*abci.TxResult) error

	// SearchBlockEvents provides the block search by given query conditions. This function only
	// supported by the kvEventSink.
	SearchBlockEvents(context.Context, *query.Query) ([]int64, error)

	// SearchTxEvents provides the transaction search by given query conditions. This function only
	// supported by the kvEventSink.
	SearchTxEvents(context.Context, *query.Query) ([]*abci.TxResult, error)

	// GetTxByHash provides the transaction search by given transaction hash. This function only
	// supported by the kvEventSink.
	GetTxByHash([]byte) (*abci.TxResult, error)

	// HasBlock provides the transaction search by given transaction hash. This function only
	// supported by the kvEventSink.
	HasBlock(int64) (bool, error)

	// Type checks the eventsink structure type.
	Type() EventSinkType

	// Stop will close the data store connection, if the eventsink supports it.
	Stop() error
}
