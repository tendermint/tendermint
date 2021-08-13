package commands

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/inspect"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer/sink"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
)

// InspectCmd is the command for starting an inspect server.
var InspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Run an inspect server for investigating Tendermint state.",
	Long: `
	inspect runs a subset of Tendermint's RPC endpoints that are useful for debugging
	issues with Tendermint. When the tendermint consensus engine detects inconsistent
	state, it will crash the tendermint process. Tendermint will not start up while in this
	inconsistent state. 

	The inspect command can be used to query the block and state store using Tendermint
	RPC calls to debug issues of inconsistent state.
	`,

	RunE: runInspect,
}

func init() {
	InspectCmd.Flags().String("rpc.laddr", config.RPC.ListenAddress, "RPC listenener address. Port required")
}

func runInspect(cmd *cobra.Command, args []string) error {
	ctx, cancelFunc := context.WithCancel(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-c
		cancelFunc()
	}()

	blockStoreDB, err := cfg.DefaultDBProvider(&cfg.DBContext{ID: "blockstore", Config: config})
	if err != nil {
		return err
	}
	blockStore := store.NewBlockStore(blockStoreDB)
	stateDB, err := cfg.DefaultDBProvider(&cfg.DBContext{ID: "statestore", Config: config})
	if err != nil {
		return err
	}
	genDoc, err := types.GenesisDocFromFile(config.GenesisFile())
	if err != nil {
		return err
	}
	sinks, err := sink.EventSinksFromConfig(config, cfg.DefaultDBProvider, genDoc.ChainID)
	if err != nil {
		return err
	}
	stateStore := state.NewStore(stateDB)

	d := inspect.New(config.RPC, blockStore, stateStore, sinks, logger)

	logger.Info("starting inspect server")
	if err := d.Run(ctx); err != nil {
		logger.Error("error encountered while running inspect server", "err", err)
		return err
	}
	return nil
}
