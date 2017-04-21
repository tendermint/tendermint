package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/types"
)

var resetAllCmd = &cobra.Command{
	Use:   "unsafe_reset_all",
	Short: "(unsafe) Remove all the data and WAL, reset this node's validator",
	Run:   ResetAll,
}

var resetPrivValidatorCmd = &cobra.Command{
	Use:   "unsafe_reset_priv_validator",
	Short: "(unsafe) Reset this node's validator",
	Run:   resetPrivValidator,
}

func init() {
	RootCmd.AddCommand(resetAllCmd)
	RootCmd.AddCommand(resetPrivValidatorCmd)
}

// XXX: this is totally unsafe.
// it's only suitable for testnets.
func ResetAll(cmd *cobra.Command, args []string) {
	resetPrivValidator(cmd, args)
	os.RemoveAll(config.GetString("db_dir"))
	os.Remove(config.GetString("cs_wal_file"))
}

// XXX: this is totally unsafe.
// it's only suitable for testnets.
func resetPrivValidator(cmd *cobra.Command, args []string) {
	// Get PrivValidator
	var privValidator *types.PrivValidator
	privValidatorFile := config.GetString("priv_validator_file")
	if _, err := os.Stat(privValidatorFile); err == nil {
		privValidator = types.LoadPrivValidator(privValidatorFile)
		privValidator.Reset()
		log.Notice("Reset PrivValidator", "file", privValidatorFile)
	} else {
		privValidator = types.GenPrivValidator()
		privValidator.SetFile(privValidatorFile)
		privValidator.Save()
		log.Notice("Generated PrivValidator", "file", privValidatorFile)
	}
}
