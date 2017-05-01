package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tendermint/log15"
	"github.com/tendermint/tendermint/types"
)

var resetAllCmd = &cobra.Command{
	Use:   "unsafe_reset_all",
	Short: "(unsafe) Remove all the data and WAL, reset this node's validator",
	Run:   resetAll,
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
func resetAll(cmd *cobra.Command, args []string) {
	ResetAll(config.GetString("db_dir"), config.GetString("priv_validator_file"), log)
}

// XXX: this is totally unsafe.
// it's only suitable for testnets.
func resetPrivValidator(cmd *cobra.Command, args []string) {
	resetPrivValidatorLocal(config.GetString("priv_validator_file"), log)
}

// Exported so other CLI tools can use  it
func ResetAll(dbDir, privValFile string, l log15.Logger) {
	resetPrivValidatorLocal(privValFile, l)
	os.RemoveAll(dbDir)
	l.Notice("Removed all data", "dir", dbDir)
}

func resetPrivValidatorLocal(privValFile string, l log15.Logger) {

	// Get PrivValidator
	var privValidator *types.PrivValidator
	if _, err := os.Stat(privValFile); err == nil {
		privValidator = types.LoadPrivValidator(privValFile)
		privValidator.Reset()
		l.Notice("Reset PrivValidator", "file", privValFile)
	} else {
		privValidator = types.GenPrivValidator()
		privValidator.SetFile(privValFile)
		privValidator.Save()
		l.Notice("Generated PrivValidator", "file", privValFile)
	}
}
