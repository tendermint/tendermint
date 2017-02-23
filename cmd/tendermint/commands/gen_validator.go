package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

var genValidatorCmd = &cobra.Command{
	Use:   "gen_validator",
	Short: "Generate new validator keypair",
	Run:   genValidator,
}

func init() {
	RootCmd.AddCommand(genValidatorCmd)
}

func genValidator(cmd *cobra.Command, args []string) {
	privValidator := types.GenPrivValidator()
	privValidatorJSONBytes := wire.JSONBytesPretty(privValidator)
	fmt.Printf(`%v
`, string(privValidatorJSONBytes))
}
