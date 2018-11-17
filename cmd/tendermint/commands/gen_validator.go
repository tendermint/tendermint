package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/privval"
)

// GenValidatorCmd allows the generation of a keypair for a
// validator.
var GenValidatorCmd = &cobra.Command{
	Use:   "gen_validator",
	Short: "Generate new validator keypair",
	Run:   genValidator,
}

func genValidator(cmd *cobra.Command, args []string) {
	pv := privval.GenFilePV("", "")
	jsbz, err := cdc.MarshalJSON(pv)
	if err != nil {
		panic(err)
	}
	fmt.Printf(`%v
`, string(jsbz))
}
