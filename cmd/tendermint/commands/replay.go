package commands

import (
	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/consensus"
)

var replayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Replay messages from WAL",
	Run: func(cmd *cobra.Command, args []string) {
		consensus.RunReplayFile(config.BaseConfig, config.Consensus, false)
	},
}

var replayConsoleCmd = &cobra.Command{
	Use:   "replay_console",
	Short: "Replay messages from WAL in a console",
	Run: func(cmd *cobra.Command, args []string) {
		consensus.RunReplayFile(config.BaseConfig, config.Consensus, true)
	},
}

func init() {
	RootCmd.AddCommand(replayCmd)
	RootCmd.AddCommand(replayConsoleCmd)
}
