package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmcfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/libs/progressbar"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/state/indexer/sink/kv"
	"github.com/tendermint/tendermint/state/indexer/sink/psql"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmdb "github.com/tendermint/tm-db"
)

// ReIndexEventCmd allows re-index the event by given block height interval
// console.
var ReIndexEventCmd = &cobra.Command{
	Use:     "reindex-event",
	Aliases: []string{"reindex_event"},
	Short:   "Reindex the missing events to the event stores",
	Example: "tendermint reindex-event --start-height 2 --end-height 10",
	Run: func(cmd *cobra.Command, args []string) {
		es, err := loadEventSinks(config)
		if err != nil {
			fmt.Println("event re-index failed: ", err)
			return
		}

		bs, ss, err := loadStateAndBlockStore(config)
		if err != nil {
			fmt.Println("event re-index failed: ", err)
			return
		}

		err = eventReIndex(cmd, es, bs, ss)
		if err != nil {
			fmt.Println("event re-index failed: ", err)
			return
		}

		fmt.Println("event re-index finished")
	},
}

var (
	startHeight int64
	endHeight   int64
)

func init() {
	ReIndexEventCmd.Flags().Int64Var(&startHeight, "start-height", 1, "the block height would like to start for re-index")
	ReIndexEventCmd.Flags().Int64Var(&endHeight, "end-height", 1, "the block height would like to finish for re-index")
}

func loadEventSinks(cfg *tmcfg.Config) ([]indexer.EventSink, error) {
	// Check duplicated sinks.
	sinks := map[string]bool{}
	for _, s := range cfg.TxIndex.Indexer {
		sl := strings.ToLower(s)
		if sinks[sl] {
			return nil, errors.New("found duplicated sinks, please check the tx-index section in the config.toml")
		}
		sinks[sl] = true
	}

	eventSinks := []indexer.EventSink{}

	for k := range sinks {
		switch k {
		case string(indexer.NULL):
			return nil, errors.New("found null event sink, please check the tx-index section in the config.toml")
		case string(indexer.KV):
			store, err := tmcfg.DefaultDBProvider(&tmcfg.DBContext{ID: "tx_index", Config: cfg})
			if err != nil {
				return nil, err
			}
			eventSinks = append(eventSinks, kv.NewEventSink(store))
		case string(indexer.PSQL):
			conn := cfg.TxIndex.PsqlConn
			if conn == "" {
				return nil, errors.New("the psql connection settings cannot be empty")
			}
			es, _, err := psql.NewEventSink(conn, chainID)
			if err != nil {
				return nil, err
			}
			eventSinks = append(eventSinks, es)
		default:
			return nil, errors.New("unsupported event sink type")
		}
	}

	if len(eventSinks) == 0 {
		return nil, errors.New("no proper event sink can do event re-indexing," +
			" please check the tx-index section in the config.toml")
	}

	return eventSinks, nil
}

func loadStateAndBlockStore(cfg *tmcfg.Config) (*store.BlockStore, state.Store, error) {
	dbType := tmdb.BackendType(cfg.DBBackend)

	// Get BlockStore
	blockStoreDB, err := tmdb.NewDB("blockstore", dbType, cfg.DBDir())
	if err != nil {
		return nil, nil, err
	}
	blockStore := store.NewBlockStore(blockStoreDB)

	// Get StateStore
	stateDB, err := tmdb.NewDB("state", dbType, cfg.DBDir())
	if err != nil {
		return nil, nil, err
	}
	stateStore := state.NewStore(stateDB)

	return blockStore, stateStore, nil
}

func eventReIndex(cmd *cobra.Command, es []indexer.EventSink, bs *store.BlockStore, ss state.Store) error {

	base := bs.Base()

	if startHeight <= base {
		return fmt.Errorf("%s (requested start height: %d, base height: %d)", ctypes.ErrHeightNotAvailable, startHeight, base)
	}

	height := bs.Height()
	if startHeight > height {
		return fmt.Errorf(
			"%s (requested start height: %d, store height: %d)", ctypes.ErrHeightNotAvailable, startHeight, height)
	}

	if endHeight <= base {
		return fmt.Errorf(
			"%s (requested end height: %d, base height: %d)", ctypes.ErrHeightNotAvailable, endHeight, base)
	}

	if endHeight < startHeight {
		return fmt.Errorf(
			"%s (requested the end height: %d is less than the start height: %d)",
			ctypes.ErrInvalidRequest, startHeight, endHeight)
	}

	if endHeight > height {
		endHeight = height
	}

	if !indexer.IndexingEnabled(es) {
		return fmt.Errorf("no event sink has been enabled")
	}

	var bar progressbar.Bar
	bar.NewOption(startHeight-1, endHeight)

	for i := startHeight; i <= endHeight; i++ {
		select {
		case <-cmd.Context().Done():
			return fmt.Errorf("event re-index terminated at height %d: %w", i, cmd.Context().Err())
		default:
			b := bs.LoadBlock(i)
			if b == nil {
				return fmt.Errorf("not able to load block at height %d from the blockstore", i)
			}

			r, err := ss.LoadABCIResponses(i)
			if err != nil {
				return fmt.Errorf("not able to load ABCI Response at height %d from the statestore", i)
			}

			e := types.EventDataNewBlockHeader{
				Header:           b.Header,
				NumTxs:           int64(len(b.Txs)),
				ResultBeginBlock: *r.BeginBlock,
				ResultEndBlock:   *r.EndBlock,
			}

			var batch *indexer.Batch
			if e.NumTxs > 0 {
				batch = indexer.NewBatch(e.NumTxs)

				for i, tx := range b.Data.Txs {
					tr := abcitypes.TxResult{
						Height: b.Height,
						Index:  uint32(i),
						Tx:     tx,
						Result: *(r.DeliverTxs[i]),
					}

					_ = batch.Add(&tr)
				}
			}

			for _, sink := range es {
				if err := sink.IndexBlockEvents(e); err != nil {
					return fmt.Errorf("block event re-index at height %d failed: %w", i, err)
				}

				if batch != nil {
					if err := sink.IndexTxEvents(batch.Ops); err != nil {
						return fmt.Errorf("tx event re-index at height %d failed: %w", i, err)
					}
				}
			}
		}

		bar.Play(i)
	}
	bar.Finish()

	return nil
}
