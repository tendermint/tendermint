package debug

import (
	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/libs/log"
)

const (
	flagNodeRPCAddr = "rpc-laddr"
	flagProfAddr    = "pprof-laddr"
	flagFrequency   = "frequency"
)

func GetDebugCommand(logger log.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "A utility to kill or watch a Tendermint process while aggregating debugging data",
	}
	cmd.PersistentFlags().SortFlags = true
	cmd.PersistentFlags().String(
		flagNodeRPCAddr,
		"tcp://localhost:26657",
		"the Tendermint node's RPC address <host>:<port>)",
	)

	cmd.AddCommand(getKillCmd(logger))
	cmd.AddCommand(getDumpCmd(logger))
	return cmd

}
