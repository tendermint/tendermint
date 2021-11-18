package commands

import (
	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/internal/consensus"
)

// ReplayCmd allows replaying of messages from the WAL.
var ReplayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Replay messages from WAL",
	RunE: func(cmd *cobra.Command, args []string) error {
		return consensus.RunReplayFile(cmd.Context(), logger, config.BaseConfig, config.Consensus, false)
	},
}

// ReplayConsoleCmd allows replaying of messages from the WAL in a
// console.
var ReplayConsoleCmd = &cobra.Command{
	Use:   "replay-console",
	Short: "Replay messages from WAL in a console",
	RunE: func(cmd *cobra.Command, args []string) error {
		return consensus.RunReplayFile(cmd.Context(), logger, config.BaseConfig, config.Consensus, true)
	},
}
