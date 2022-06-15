package pubsub

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/pubsub"

	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/state/txindex"
)

const credsEnvVar = "GOOGLE_APPLICATION_CREDENTIALS"

var (
	_ indexer.BlockIndexer = (*EventSink)(nil)
	_ txindex.TxIndexer    = (*EventSink)(nil)
)

type EventSink struct {
	client  *pubsub.Client
	chainID string
}

func NewEventSink(connStr, chainID string) (*EventSink, error) {
	if s := os.Getenv(credsEnvVar); len(s) == 0 {
		return nil, fmt.Errorf("missing '%s' environment variable", credsEnvVar)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	c, err := pubsub.NewClient(ctx, "project-id")
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Cloud Pubsub client: %w", err)
	}

	return &EventSink{
		client:  c,
		chainID: chainID,
	}, nil
}
