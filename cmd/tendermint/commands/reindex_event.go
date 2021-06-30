package commands

import "github.com/spf13/cobra"

// ReindexEventCmd allows re-index the event by given block height interval
// console.
var ReIndexEventCmd = &cobra.Command{
	Use:     "reindex-event",
	Aliases: []string{"reindex_event"},
	Short:   "Reindex the missing events to the event stores",
	Example: "tendermint reindex-event --start-height 1 --end-height 10",
	Run: func(cmd *cobra.Command, args []string) {
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
