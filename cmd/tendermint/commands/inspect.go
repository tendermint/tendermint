package commands

import (
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/inspect"
	"github.com/tendermint/tendermint/libs/log"
)

// InspectCmd constructs the command to start an inspect server.
func MakeInspectCommand(conf *config.Config, logger log.Logger) *cobra.Command {
	cmd := &cobra.Command{
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
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGTERM, syscall.SIGINT)
			defer cancel()

			ins, err := inspect.NewFromConfig(logger, conf)
			if err != nil {
				return err
			}

			logger.Info("starting inspect server")
			if err := ins.Run(ctx); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().String("rpc.laddr",
		conf.RPC.ListenAddress, "RPC listenener address. Port required")
	cmd.Flags().String("db-backend",
		conf.DBBackend, "database backend: goleveldb | cleveldb | boltdb | rocksdb | badgerdb")
	cmd.Flags().String("db-dir", conf.DBPath, "database directory")

	return cmd
}
