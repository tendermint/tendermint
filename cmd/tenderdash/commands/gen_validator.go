package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/privval"
)

// MakeGenValidatorCommand allows the generation of a keypair for a
// validator.
func MakeGenValidatorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gen-validator",
		Short: "Generate new validator keypair",
		RunE: func(cmd *cobra.Command, args []string) error {
			pv := privval.GenFilePV("", "")

			jsbz, err := json.Marshal(pv)
			if err != nil {
				return fmt.Errorf("validator -> json: %w", err)
			}

			fmt.Printf("%v\n", string(jsbz))

			return nil
		},
	}

	return cmd
}
