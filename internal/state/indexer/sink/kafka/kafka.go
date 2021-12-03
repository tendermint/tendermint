// Package kafka implements an event sink backed by a Kafka message producer.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/gogo/protobuf/proto"
	"github.com/google/orderedcode"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/types"
)

// EventSink is an indexer backend providing the tx/block index services.
// This implementation emit messages using the kafka producer.
type EventSink struct {
	chainID  string
	producer sarama.SyncProducer
}

// NewEventSink constructs an event sink associated with the kafka producer
// specified by connStr. Events written to the sink are attributed to
// the specified chainID.
func NewEventSink(connStr, chainID string) (*EventSink, error) {
	p, err := sarama.NewSyncProducer([]string{connStr}, nil)
	if err != nil {
		return nil, err
	}

	return &EventSink{
		chainID:  chainID,
		producer: p,
	}, nil
}

// Connection returns the underlying kafka connection used by the sink.
// This is exported to support testing.
func (es *EventSink) Producer() sarama.SyncProducer { return es.producer }

// Type returns the structure type for this sink, which is KAFKA.
func (es *EventSink) Type() indexer.EventSinkType { return indexer.KAFKA }

// covertEventsToMessages transform the abci events and associate with the block height or the tx hash
// to the kafka messages.
func covertEventsToMessages(chainID string, height int64, txhash string, evts []abci.Event) ([]*sarama.ProducerMessage, error) {
	msgs := make([]*sarama.ProducerMessage, 0)

	for _, evt := range evts {
		// Skip events with an empty type.
		if evt.Type == "" {
			continue
		}

		// Add any attributes flagged for indexing.
		for _, attr := range evt.Attributes {
			if !attr.Index {
				continue
			}

			var key []byte
			var err error
			// Composite the message key with event attribute key, and/or block height, and/or tx hash.
			if height == -1 {
				key, err = orderedcode.Append(nil, attr.Key)
			} else if len(txhash) == 0 {
				key, err = orderedcode.Append(nil, attr.Key, height)
			} else {
				key, err = orderedcode.Append(nil, attr.Key, height, txhash)
			}

			if err != nil {
				return nil, fmt.Errorf("failed to create the index key: %w", err)
			}

			msgs = append(msgs, &sarama.ProducerMessage{
				Topic: fmt.Sprintf("%s.%s", chainID, evt.Type),
				Key:   sarama.ByteEncoder(key),
				Value: sarama.ByteEncoder(attr.Value),
			})
		}
	}
	return msgs, nil
}

// makeIndexedEvent constructs an event from the specified composite key and
// value. If the key has the form "type.name", the event will have a single
// attribute with that name and the value; otherwise the event will have only
// a type and no attributes.
func makeIndexedEvent(compositeKey, value string) abci.Event {
	i := strings.Index(compositeKey, ".")
	if i < 0 {
		return abci.Event{Type: compositeKey}
	}
	return abci.Event{Type: compositeKey[:i], Attributes: []abci.EventAttribute{
		{Key: compositeKey[i+1:], Value: value, Index: true},
	}}
}

// IndexBlockEvents indexes the specified block header, part of the
// indexer.EventSink interface.
func (es *EventSink) IndexBlockEvents(h types.EventDataNewBlockHeader) error {
	kmsgs := make([]*sarama.ProducerMessage, 0)

	// Index the block height event and convert it to an kafka message.
	blockHeightEvent := makeIndexedEvent(types.BlockHeightKey, fmt.Sprint(h.Header.Height))
	m, err := covertEventsToMessages(es.chainID, -1, "", []abci.Event{blockHeightEvent})
	if err != nil {
		return err
	}
	kmsgs = append(kmsgs, m...)

	// Convert the begin block events to the kafka messages.
	m, err = covertEventsToMessages(es.chainID, h.Header.Height, "", h.ResultBeginBlock.Events)
	if err != nil {
		return err
	}
	kmsgs = append(kmsgs, m...)

	// Convert the end block events to the kafka messages.
	m, err = covertEventsToMessages(es.chainID, h.Header.Height, "", h.ResultEndBlock.Events)
	if err != nil {
		return err
	}
	kmsgs = append(kmsgs, m...)

	return es.producer.SendMessages(kmsgs)
}

// IndexTxEvents indexes the specified transaction results, part of the
// indexer.EventSink interface.
func (es *EventSink) IndexTxEvents(txrs []*abci.TxResult) error {
	kmsgs := make([]*sarama.ProducerMessage, 0)

	txHeightEventIndexed := false
	for _, txr := range txrs {
		// Assume the batched tx_result will be the same block height.
		if !txHeightEventIndexed {
			txHeightEvent := makeIndexedEvent(types.TxHeightKey, fmt.Sprint(txr.Height))
			m, err := covertEventsToMessages(es.chainID, -1, "", []abci.Event{txHeightEvent})
			if err != nil {
				return err
			}
			kmsgs = append(kmsgs, m...)
			txHeightEventIndexed = true
		}

		// Index the hash of the underlying transaction as a hex string.
		txHash := fmt.Sprintf("%X", types.Tx(txr.Tx).Hash())

		// Index the txhash event with the tx height.
		txHashEvent := makeIndexedEvent(types.TxHashKey, txHash)
		m, err := covertEventsToMessages(es.chainID, txr.Height, "", []abci.Event{txHashEvent})
		if err != nil {
			return err
		}
		kmsgs = append(kmsgs, m...)

		// Encode the result message in protobuf wire format for indexing.
		resultData, err := proto.Marshal(txr)
		if err != nil {
			return fmt.Errorf("marshaling tx_result: %w", err)
		}

		// Index the tx raw data with txHash
		key, err := orderedcode.Append(
			nil,
			"hash",
			txHash,
		)
		if err != nil {
			return fmt.Errorf("failed to create the index key: %w", err)
		}

		msg := &sarama.ProducerMessage{
			Topic: fmt.Sprintf("%s.%s", es.chainID, "tx"),
			Key:   sarama.ByteEncoder(key),
			Value: sarama.ByteEncoder(resultData),
		}

		kmsgs = append(kmsgs, msg)

		// Index the tx event data.
		m, err = covertEventsToMessages(es.chainID, txr.Height, txHash, txr.Result.Events)
		if err != nil {
			return err
		}
		kmsgs = append(kmsgs, m...)
	}

	return es.producer.SendMessages(kmsgs)
}

// SearchBlockEvents is not implemented by this sink, and reports an error for all queries.
func (es *EventSink) SearchBlockEvents(ctx context.Context, q *query.Query) ([]int64, error) {
	return nil, errors.New("block search is not supported via the kafka event sink")
}

// SearchTxEvents is not implemented by this sink, and reports an error for all queries.
func (es *EventSink) SearchTxEvents(ctx context.Context, q *query.Query) ([]*abci.TxResult, error) {
	return nil, errors.New("tx search is not supported via the kafka event sink")
}

// GetTxByHash is not implemented by this sink, and reports an error for all queries.
func (es *EventSink) GetTxByHash(hash []byte) (*abci.TxResult, error) {
	return nil, errors.New("getTxByHash is not supported via the kafka event sink")
}

// HasBlock is not implemented by this sink, and reports an error for all queries.
func (es *EventSink) HasBlock(h int64) (bool, error) {
	return false, errors.New("hasBlock is not supported via the kafka event sink")
}

// Stop closes the underlying kafka producer.
func (es *EventSink) Stop() error {
	return es.producer.Close()
}
