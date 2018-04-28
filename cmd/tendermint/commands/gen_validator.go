package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	pvm "github.com/tendermint/tendermint/types/priv_validator"
)

// GenValidatorCmd allows the generation of a keypair for a
// validator.
var GenValidatorCmd = &cobra.Command{
	Use:   "gen_validator",
	Short: "Generate new validator keypair",
	Run:   genValidator,
}

func genValidator(cmd *cobra.Command, args []string) {
	pv := pvm.GenFilePV("")
	jsbz, err := cdc.MarshalJSON(pv)
	if err != nil {
		panic(err)
	}
	fmt.Printf(`%v
`, string(jsbz))
}
