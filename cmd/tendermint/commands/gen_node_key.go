package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/p2p"
)

// GenNodeKeyCmd allows the generation of a node key. It prints JSON-encoded
// NodeKey to the standard output.
var GenNodeKeyCmd = &cobra.Command{
	Use:     "gen-node-key",
	Aliases: []string{"gen_node_key"},
	Short:   "Generate a new node key",
	RunE:    genNodeKey,
	PreRun:  deprecateSnakeCase,
}

func genNodeKey(cmd *cobra.Command, args []string) error {
	nodeKey := p2p.GenNodeKey()

	bz, err := tmjson.Marshal(nodeKey)
	if err != nil {
		return fmt.Errorf("nodeKey -> json: %w", err)
	}

	fmt.Printf(`%v
`, string(bz))

	return nil
}
