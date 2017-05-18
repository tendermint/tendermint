package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/go-wire/data"
	"github.com/tendermint/tendermint/types"
)

var showValidatorCmd = &cobra.Command{
	Use:   "show_validator",
	Short: "Show this node's validator info",
	Run:   showValidator,
}

func init() {
	RootCmd.AddCommand(showValidatorCmd)
}

func showValidator(cmd *cobra.Command, args []string) {
	privValidator := types.LoadOrGenPrivValidator(config.PrivValidatorFile(), logger)
	pubKeyJSONBytes, _ := data.ToJSON(privValidator.PubKey)
	fmt.Println(string(pubKeyJSONBytes))
}
