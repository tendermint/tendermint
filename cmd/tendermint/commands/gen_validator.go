package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	priv_val "github.com/tendermint/tendermint/types/priv_validator"
)

// GenValidatorCmd allows the generation of a keypair for a
// validator.
var GenValidatorCmd = &cobra.Command{
	Use:   "gen_validator",
	Short: "Generate new validator keypair",
	Run:   genValidator,
}

func genValidator(cmd *cobra.Command, args []string) {
	privValidator := priv_val.GenPrivValidatorJSON("")
	privValidatorJSONBytes, err := json.MarshalIndent(privValidator, "", "\t")
	if err != nil {
		panic(err)
	}
	fmt.Printf(`%v
`, string(privValidatorJSONBytes))
}
