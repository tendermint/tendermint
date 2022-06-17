package pubsub

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/gogo/protobuf/jsonpb"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
)

const (
	credsEnvVar = "GOOGLE_APPLICATION_CREDENTIALS"

	AttrKeyChainID     = "chain_id"
	AttrKeyBlockHeight = "block_height"

	MsgType           = "message_type"
	MsgTypeBeginBlock = "begin_block"
	MsgTypeEndBlock   = "end_block"
	// MsgTypeTxResult         = "tx_result"
	// MsgTypeTxEvents         = "tx_events"
)

var jsonpbMarshaller = jsonpb.Marshaler{
	EnumsAsInts:  true,
	EmitDefaults: true,
}

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
	buf := new(bytes.Buffer)
	blockHeightStr := strconv.Itoa(int(h.Header.Height))

	// publish BeginBlock Events
	if err := jsonpbMarshaller.Marshal(buf, &h.ResultBeginBlock); err != nil {
		return fmt.Errorf("failed to JSON marshal ResultBeginBlock: %w", err)
	}

	res := es.topic.Publish(
		context.Background(),
		&pubsub.Message{
			Data: buf.Bytes(),
			Attributes: map[string]string{
				MsgType:            MsgTypeBeginBlock,
				AttrKeyChainID:     es.chainID,
				AttrKeyBlockHeight: blockHeightStr,
			},
		},
	)

	// TODO: Should we wait for the write to complete or just fire and forget?
	if _, err := res.Get(context.Background()); err != nil {
		return fmt.Errorf("failed to publish pubsub message: %w", err)
	}

	buf.Reset()

	// publish EndBlock Events
	if err := jsonpbMarshaller.Marshal(buf, &h.ResultEndBlock); err != nil {
		return fmt.Errorf("failed to JSON marshal ResultBeginBlock: %w", err)
	}

	res = es.topic.Publish(
		context.Background(),
		&pubsub.Message{
			Data: buf.Bytes(),
			Attributes: map[string]string{
				MsgType:            MsgTypeEndBlock,
				AttrKeyChainID:     es.chainID,
				AttrKeyBlockHeight: blockHeightStr,
			},
		},
	)

	// TODO: Should we wait for the write to complete or just fire and forget?
	if _, err := res.Get(context.Background()); err != nil {
		return fmt.Errorf("failed to publish pubsub message: %w", err)
	}

	return nil
}

func (es *EventSink) IndexTxs(txrs []*abci.TxResult) error {
	panic("implement me!")
}
