package pubsub

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
)

const credsEnvVar = "GOOGLE_APPLICATION_CREDENTIALS"

type EventSink struct {
	client  *pubsub.Client
	chainID string
}

func NewEventSink(projectID, chainID string) (*EventSink, error) {
	if s := os.Getenv(credsEnvVar); len(s) == 0 {
		return nil, fmt.Errorf("missing '%s' environment variable", credsEnvVar)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	c, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create a Google Cloud Pubsub client: %w", err)
	}

	return &EventSink{
		client:  c,
		chainID: chainID,
	}, nil
}

func (es *EventSink) IndexBlock(h types.EventDataNewBlockHeader) error {
	panic("implement me!")
}

func (es *EventSink) IndexTxs(txrs []*abci.TxResult) error {
	panic("implement me!")
}
