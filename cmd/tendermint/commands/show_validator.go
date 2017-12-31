package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/go-wire/data"
	priv_val "github.com/tendermint/tendermint/types/priv_validator"
)

// ShowValidatorCmd adds capabilities for showing the validator info.
var ShowValidatorCmd = &cobra.Command{
	Use:   "show_validator",
	Short: "Show this node's validator info",
	Run:   showValidator,
}

func showValidator(cmd *cobra.Command, args []string) {
	privValidator := priv_val.LoadOrGenDefaultPrivValidator(config.PrivValidatorFile())
	pubKeyJSONBytes, _ := data.ToJSON(privValidator.PubKey)
	fmt.Println(string(pubKeyJSONBytes))
}
