package sink

import (
	"errors"
	"strings"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/state/indexer/sink/null"
)

// EventSinksFromConfig constructs a slice of indexer.EventSink using the provided
// configuration.
//
//nolint:lll
func EventSinksFromConfig(cfg *config.Config, chainID string) ([]indexer.EventSink, error) {
	if len(cfg.TxIndex.Indexer) == 0 {
		return []indexer.EventSink{null.NewEventSink()}, nil
	}

	// check for duplicated sinks
	sinks := map[string]struct{}{}
	for _, s := range cfg.TxIndex.Indexer {
		sl := strings.ToLower(s)
		if _, ok := sinks[sl]; ok {
			return nil, errors.New("found duplicated sinks, please check the tx-index section in the config.toml")
		}
		sinks[sl] = struct{}{}
	}
	eventSinks := []indexer.EventSink{}

	for k := range sinks {
		sink, err := indexer.CreateSink(k, cfg, chainID)
		if err != nil {
			return nil, err
		}

		if sink.Type() == indexer.NULL {
			// When we see null in the config, the eventsinks will be reset with the
			// nullEventSink.
			return []indexer.EventSink{sink}, nil
		}
		eventSinks = append(eventSinks, sink)
	}
	return eventSinks, nil
}
