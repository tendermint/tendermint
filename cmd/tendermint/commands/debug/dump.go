package debug

import (
	"github.com/spf13/cobra"
)

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Continuously poll a Tendermint process and dump debugging data into a single location",
	RunE: func(cmd *cobra.Command, args []string) error {
		panic("not implemented")
	},
}
