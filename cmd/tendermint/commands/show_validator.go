package commands

import (
	"fmt"
	"os"

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
	key, err := privValidator.GetPubKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read pubkey from private validator file (%s): %v", config.PrivValidatorFile(), err)
		os.Exit(1)
	}
	pubKeyJSONBytes, _ := cdc.MarshalJSON(key)
	fmt.Println(string(pubKeyJSONBytes))
}
