package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/types"
)

var GenValidatorCmd = &cobra.Command{
	Use:   "gen_validator",
	Short: "Generate new validator keypair",
	Run:   genValidator,
}

func genValidator(cmd *cobra.Command, args []string) {
	privValidator := types.GenPrivValidator()
	privValidatorJSONBytes, _ := json.MarshalIndent(privValidator, "", "\t")
	fmt.Printf(`%v
`, string(privValidatorJSONBytes))
}
