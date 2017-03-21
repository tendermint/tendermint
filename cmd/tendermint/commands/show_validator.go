package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	data "github.com/tendermint/go-data"
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
	privValidatorFile := config.GetString("priv_validator_file")
	privValidator := types.LoadOrGenPrivValidator(privValidatorFile)
	pubKeyJSONBytes, _ := data.ToJSON(privValidator.PubKey)
	fmt.Println(string(pubKeyJSONBytes))
}
