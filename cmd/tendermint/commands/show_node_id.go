package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/config"
)

// MakeShowNodeIDCommand constructs a command to dump the node ID to stdout.
func MakeShowNodeIDCommand(conf *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "show-node-id",
		Short: "Show this node's ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeKeyID, err := conf.LoadNodeKeyID()
			if err != nil {
				return err
			}

			fmt.Println(nodeKeyID)
			return nil
		},
	}
}
