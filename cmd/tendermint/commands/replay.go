package commands

import (
	"fmt"

	"github.com/tendermint/tendermint/consensus"

	"github.com/spf13/cobra"
)

var replayCmd = &cobra.Command{
	Use:   "replay [walfile]",
	Short: "Replay messages from WAL",
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) > 1 {
			consensus.RunReplayFile(config, args[1], false)
		} else {
			fmt.Println("replay requires an argument (walfile)")
		}
	},
}

var replayConsoleCmd = &cobra.Command{
	Use:   "replay_console [walfile]",
	Short: "Replay messages from WAL in a console",
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) > 1 {
			consensus.RunReplayFile(config, args[1], true)
		} else {
			fmt.Println("replay_console requires an argument (walfile)")
		}
	},
}

func init() {
	RootCmd.AddCommand(replayCmd)
	RootCmd.AddCommand(replayConsoleCmd)
}
