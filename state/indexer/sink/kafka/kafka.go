package kafka

import (
	"cloud.google.com/go/pubsub"

	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/state/txindex"
)

var (
	_ indexer.BlockIndexer = (*EventSink)(nil)
	_ txindex.TxIndexer    = (*EventSink)(nil)
)

type EventSink struct {
	client *pubsub.Client
}
