package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/go-wire"
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
	fmt.Println(string(wire.JSONBytesPretty(privValidator.PubKey)))
}
