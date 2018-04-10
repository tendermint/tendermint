package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/p2p"
	cmn "github.com/tendermint/tmlibs/common"
)

// GenNodeKeyCmd allows the generation of a node key. It prints node's ID to
// the standard output.
var GenNodeKeyCmd = &cobra.Command{
	Use:   "gen_node_key",
	Short: "Generate a node key for this node and print its ID",
	RunE:  genNodeKey,
}

func genNodeKey(cmd *cobra.Command, args []string) error {
	nodeKeyFile := config.NodeKeyFile()
	if cmn.FileExists(nodeKeyFile) {
		return fmt.Errorf("node key at %s already exists", nodeKeyFile)
	}

	nodeKey, err := p2p.LoadOrGenNodeKey(nodeKeyFile)
	if err != nil {
		return err
	}
	fmt.Println(nodeKey.ID())
	return nil
}
