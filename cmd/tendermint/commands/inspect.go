package commands

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/internal/inspect"
)

// InspectCmd is the command for starting an inspect server.
var InspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Run an inspect server for investigating Tendermint state",
	Long: `
	inspect runs a subset of Tendermint's RPC endpoints that are useful for debugging
	issues with Tendermint.

	When the Tendermint consensus engine detects inconsistent state, it will crash the
	tendermint process. Tendermint will not start up while in this inconsistent state. 
	The inspect command can be used to query the block and state store using Tendermint
	RPC calls to debug issues of inconsistent state.
	`,

	RunE: runInspect,
}

func init() {
	InspectCmd.Flags().
		String("rpc.laddr",
			config.RPC.ListenAddress, "RPC listenener address. Port required")
	InspectCmd.Flags().
		String("db-backend",
			config.DBBackend, "database backend: goleveldb | cleveldb | boltdb | rocksdb | badgerdb")
	InspectCmd.Flags().
		String("db-dir", config.DBPath, "database directory")
}

func runInspect(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-c
		cancel()
	}()

	ins, err := inspect.NewFromConfig(logger, config)
	if err != nil {
		return err
	}

	logger.Info("starting inspect server")
	if err := ins.Run(ctx); err != nil {
		return err
	}
	return nil
}
