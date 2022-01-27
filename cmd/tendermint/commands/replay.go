package commands

import (
	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/libs/log"
)

// ReplayCmd allows replaying of messages from the WAL.
func MakeReplayCommand(conf *config.Config, logger log.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "replay",
		Short: "Replay messages from WAL",
		RunE: func(cmd *cobra.Command, args []string) error {
			return consensus.RunReplayFile(cmd.Context(), logger, conf.BaseConfig, conf.Consensus, false)
		},
	}
}

// ReplayConsoleCmd allows replaying of messages from the WAL in a
// console.
func MakeReplayConsoleCommand(conf *config.Config, logger log.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "replay-console",
		Short: "Replay messages from WAL in a console",
		RunE: func(cmd *cobra.Command, args []string) error {
			return consensus.RunReplayFile(cmd.Context(), logger, conf.BaseConfig, conf.Consensus, true)
		},
	}
}
