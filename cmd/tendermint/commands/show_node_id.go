package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/config"
)

// MakeShowNodeIDCmd dumps node's ID to the standard output.
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
