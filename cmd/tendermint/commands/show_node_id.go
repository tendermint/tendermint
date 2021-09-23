package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ShowNodeIDCmd dumps node's ID to the standard output.
var ShowNodeIDCmd = &cobra.Command{
	Use:   "show-node-id",
	Short: "Show this node's ID",
	RunE:  showNodeID,
}

func showNodeID(cmd *cobra.Command, args []string) error {
	nodeKeyID, err := config.LoadNodeKeyID()
	if err != nil {
		return err
	}

	fmt.Println(nodeKeyID)
	return nil
}
