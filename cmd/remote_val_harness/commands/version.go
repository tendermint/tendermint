package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/version"
)

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show Tendermint version",
		Long: `
Shows the Tendermint version for which this test harness was built.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.Version)
		},
	}
}
