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

const (
	credsEnvVar = "GOOGLE_APPLICATION_CREDENTIALS"

	MsgTypeBlock       = "block"
	MsgTypeTxResult    = "tx_result"
	MsgTypeTxEvents    = "tx_events"
	MsgTypeBlockEvents = "block_events"
)

type EventSink struct {
	client  *pubsub.Client
	topic   *pubsub.Topic
	chainID string
}

func NewEventSink(projectID, topic, chainID string) (*EventSink, error) {
	if s := os.Getenv(credsEnvVar); len(s) == 0 {
		return nil, fmt.Errorf("missing '%s' environment variable", credsEnvVar)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	c, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create a Google Cloud Pubsub client: %w", err)
	}

	// attempt to get the topic. If that fails, we attempt to create it
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	t := c.Topic(topic)
	ok, err := t.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check for topic '%s': %w", topic, err)
	}
	if !ok {
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		t, err = c.CreateTopic(ctx, topic)
		if err != nil {
			return nil, fmt.Errorf("failed to create topic '%s': %w", topic, err)
		}
	}

	return &EventSink{
		client:  c,
		topic:   t,
		chainID: chainID,
	}, nil
}

func (es *EventSink) IndexBlock(h types.EventDataNewBlockHeader) error {
	panic("implement me!")
}

func (es *EventSink) IndexTxs(txrs []*abci.TxResult) error {
	panic("implement me!")
}
