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
	Run:   showNodeID,
}

func showNodeID(cmd *cobra.Command, args []string) {
	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
	if err != nil {
		logger.Error("Failed to load (or generate) node key", "err", err)
		return
	}
	fmt.Println(nodeKey.ID())
}
