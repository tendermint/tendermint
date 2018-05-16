package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/p2p"
)

// ShowNodeIDCmd dumps node's ID to the standard output.
var ShowNodeIDCmd = &cobra.Command{
	Use:   "show_node_id",
	Short: "Show this node's ID",
	RunE:  showNodeID,
}

func showNodeID(cmd *cobra.Command, args []string) error {

	nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		return err
	}
	fmt.Println(nodeKey.ID())

	return nil
}
