package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/types"
)

// GenNodeKeyCmd allows the generation of a node key. It prints JSON-encoded
// NodeKey to the standard output.
var GenNodeKeyCmd = &cobra.Command{
	Use:   "gen-node-key",
	Short: "Generate a new node key",
	RunE:  genNodeKey,
}

func genNodeKey(cmd *cobra.Command, args []string) error {
	nodeKey := types.GenNodeKey()

	bz, err := json.Marshal(nodeKey)
	if err != nil {
		return fmt.Errorf("nodeKey -> json: %w", err)
	}

	fmt.Printf(`%v
`, string(bz))

	return nil
}
