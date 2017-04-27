package commands

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

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
	ResetAll(config, log)
}

// XXX: this is totally unsafe.
// it's only suitable for testnets.
func resetPrivValidator(cmd *cobra.Command, args []string) {
	resetPrivValidatorLocal(config, log)
}

// Exported so other CLI tools can use  it
func ResetAll(c *viper.Viper, l log15.Logger) {
	resetPrivValidatorLocal(c, l)
	dataDir := c.GetString("db_dir")
	os.RemoveAll(dataDir)
	l.Notice("Removed all data", "dir", dataDir)
}

func resetPrivValidatorLocal(c *viper.Viper, l log15.Logger) {

	// Get PrivValidator
	var privValidator *types.PrivValidator
	privValidatorFile := c.GetString("priv_validator_file")
	if _, err := os.Stat(privValidatorFile); err == nil {
		privValidator = types.LoadPrivValidator(privValidatorFile)
		privValidator.Reset()
		l.Notice("Reset PrivValidator", "file", privValidatorFile)
	} else {
		privValidator = types.GenPrivValidator()
		privValidator.SetFile(privValidatorFile)
		privValidator.Save()
		l.Notice("Generated PrivValidator", "file", privValidatorFile)
	}
}
