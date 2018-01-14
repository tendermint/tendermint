package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/types"
)

// ShowValidatorCmd adds capabilities for showing the validator info.
var ShowValidatorCmd = &cobra.Command{
	Use:   "show_validator",
	Short: "Show this node's validator info",
	Run:   showValidator,
}

func showValidator(cmd *cobra.Command, args []string) {
	privValidator := types.LoadOrGenPrivValidatorFS(config.PrivValidatorFile())
	pubKeyJSONBytes, _ := json.Marshal(privValidator.PubKey) // TODO: use go-wire ?
	fmt.Println(string(pubKeyJSONBytes))
}
