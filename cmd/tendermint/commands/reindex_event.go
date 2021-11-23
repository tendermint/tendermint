package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	dbm "github.com/tendermint/tm-db"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmcfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/libs/progressbar"
	"github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/state/indexer/sink/kv"
	"github.com/tendermint/tendermint/internal/state/indexer/sink/psql"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

const (
	reindexFailed = "event re-index failed: "
)

// ReIndexEventCmd allows re-index the event by given block height interval
var ReIndexEventCmd = &cobra.Command{
	Use:   "reindex-event",
	Short: "reindex events to the event store backends",
	Long: `
reindex-event is an offline tooling to re-index block and tx events to the eventsinks,
you can run this command when the event store backend dropped/disconnected or you want to 
replace the backend. The default start-height is 0, meaning the tooling will start 
reindex from the base block height(inclusive); and the default end-height is 0, meaning 
the tooling will reindex until the latest block height(inclusive). User can omit
either or both arguments.
	`,
	Example: `
	tendermint reindex-event
	tendermint reindex-event --start-height 2
	tendermint reindex-event --end-height 10
	tendermint reindex-event --start-height 2 --end-height 10
	`,
	Run: func(cmd *cobra.Command, args []string) {
		bs, ss, err := loadStateAndBlockStore(config)
		if err != nil {
			fmt.Println(reindexFailed, err)
			return
		}

		if err := checkValidHeight(bs); err != nil {
			fmt.Println(reindexFailed, err)
			return
		}

		es, err := loadEventSinks(config)
		if err != nil {
			fmt.Println(reindexFailed, err)
			return
		}

		if err = eventReIndex(cmd, es, bs, ss); err != nil {
			fmt.Println(reindexFailed, err)
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
	ReIndexEventCmd.Flags().Int64Var(&startHeight, "start-height", 0, "the block height would like to start for re-index")
	ReIndexEventCmd.Flags().Int64Var(&endHeight, "end-height", 0, "the block height would like to finish for re-index")
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
			es, err := psql.NewEventSink(conn, chainID)
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

	if !indexer.IndexingEnabled(eventSinks) {
		return nil, fmt.Errorf("no event sink has been enabled")
	}

	return eventSinks, nil
}

func loadStateAndBlockStore(cfg *tmcfg.Config) (*store.BlockStore, state.Store, error) {
	dbType := dbm.BackendType(cfg.DBBackend)

	// Get BlockStore
	blockStoreDB, err := dbm.NewDB("blockstore", dbType, cfg.DBDir())
	if err != nil {
		return nil, nil, err
	}
	blockStore := store.NewBlockStore(blockStoreDB)

	// Get StateStore
	stateDB, err := dbm.NewDB("state", dbType, cfg.DBDir())
	if err != nil {
		return nil, nil, err
	}
	stateStore := state.NewStore(stateDB)

	return blockStore, stateStore, nil
}

func eventReIndex(cmd *cobra.Command, es []indexer.EventSink, bs state.BlockStore, ss state.Store) error {

	var bar progressbar.Bar
	bar.NewOption(startHeight-1, endHeight)

	fmt.Println("start re-indexing events:")
	defer bar.Finish()
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

	return nil
}

func checkValidHeight(bs state.BlockStore) error {
	base := bs.Base()

	if startHeight == 0 {
		startHeight = base
		fmt.Printf("set the start block height to the base height of the blockstore %d \n", base)
	}

	if startHeight < base {
		return fmt.Errorf("%s (requested start height: %d, base height: %d)",
			coretypes.ErrHeightNotAvailable, startHeight, base)
	}

	height := bs.Height()

	if startHeight > height {
		return fmt.Errorf(
			"%s (requested start height: %d, store height: %d)", coretypes.ErrHeightNotAvailable, startHeight, height)
	}

	if endHeight == 0 || endHeight > height {
		endHeight = height
		fmt.Printf("set the end block height to the latest height of the blockstore %d \n", height)
	}

	if endHeight < base {
		return fmt.Errorf(
			"%s (requested end height: %d, base height: %d)", coretypes.ErrHeightNotAvailable, endHeight, base)
	}

	if endHeight < startHeight {
		return fmt.Errorf(
			"%s (requested the end height: %d is less than the start height: %d)",
			coretypes.ErrInvalidRequest, startHeight, endHeight)
	}

	return nil
}
