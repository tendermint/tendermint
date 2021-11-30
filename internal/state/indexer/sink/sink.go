package sink

import (
	"errors"
	"strings"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/state/indexer/sink/kv"
	"github.com/tendermint/tendermint/internal/state/indexer/sink/null"
	"github.com/tendermint/tendermint/internal/state/indexer/sink/psql"
)

// EventSinksFromConfig constructs a slice of indexer.EventSink using the provided
// configuration.
func EventSinksFromConfig(cfg *config.Config, dbProvider config.DBProvider, chainID string) ([]indexer.EventSink, error) {
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
		switch indexer.EventSinkType(k) {
		case indexer.NULL:
			// When we see null in the config, the eventsinks will be reset with the
			// nullEventSink.
			return []indexer.EventSink{null.NewEventSink()}, nil

		case indexer.KV:
			store, err := dbProvider(&config.DBContext{ID: "tx_index", Config: cfg})
			if err != nil {
				return nil, err
			}

			eventSinks = append(eventSinks, kv.NewEventSink(store))

		case indexer.PSQL:
			conn := cfg.TxIndex.PsqlConn
			if conn == "" {
				return nil, errors.New("the psql connection settings cannot be empty")
			}

			es, err := psql.NewEventSink(conn, chainID)
			if err != nil {
				return nil, err
			}
			eventSinks = append(eventSinks, es)
		default:
			return nil, errors.New("unsupported event sink type")
		}
	}
	return eventSinks, nil

}
