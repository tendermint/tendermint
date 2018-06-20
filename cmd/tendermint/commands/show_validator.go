package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/privval"
)

// ShowValidatorCmd adds capabilities for showing the validator info.
var ShowValidatorCmd = &cobra.Command{
	Use:   "show_validator",
	Short: "Show this node's validator info",
	Run:   showValidator,
}

func showValidator(cmd *cobra.Command, args []string) {
	privValidator := privval.LoadOrGenFilePV(config.PrivValidatorFile())
	pubKeyJSONBytes, _ := cdc.MarshalJSON(privValidator.GetPubKey())
	fmt.Println(string(pubKeyJSONBytes))
}
