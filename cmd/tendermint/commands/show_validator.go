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
	// TODO(ismail): add a flag and check if we actually want to see the pub key
	//  of the remote signer instead of the FilePV
	privValidator := privval.LoadOrGenFilePV(config.PrivValidatorFile())
	pubKeyJSONBytes, _ := cdc.MarshalJSON(privValidator.GetPubKey())
	fmt.Println(string(pubKeyJSONBytes))
}
