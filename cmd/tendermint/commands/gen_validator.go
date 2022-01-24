package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

// GenValidatorCmd allows the generation of a keypair for a
// validator.
var GenValidatorCmd = &cobra.Command{
	Use:   "gen-validator",
	Short: "Generate new validator keypair",
	RunE:  genValidator,
}

func init() {
	GenValidatorCmd.Flags().StringVar(&keyType, "key", types.ABCIPubKeyTypeEd25519,
		"Key type to generate privval file with. Options: ed25519, secp256k1")
}

func genValidator(cmd *cobra.Command, args []string) error {
	pv, err := privval.GenFilePV("", "", keyType)
	if err != nil {
		return err
	}

	jsbz, err := json.Marshal(pv)
	if err != nil {
		return fmt.Errorf("validator -> json: %w", err)
	}

	fmt.Printf(`%v
`, string(jsbz))

	return nil
}
