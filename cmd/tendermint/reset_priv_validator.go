package main

import (
	"os"

	"github.com/tendermint/tendermint/types"
)

// NOTE: this is totally unsafe.
// it's only suitable for testnets.
func reset_all() {
	reset_priv_validator()
	os.RemoveAll(config.GetString("db_dir"))
	os.Remove(config.GetString("cswal"))
}

// NOTE: this is totally unsafe.
// it's only suitable for testnets.
func reset_priv_validator() {
	// Get PrivValidator
	var privValidator *types.PrivValidator
	privValidatorFile := config.GetString("priv_validator_file")
	if _, err := os.Stat(privValidatorFile); err == nil {
		privValidator = types.LoadPrivValidator(privValidatorFile)
		privValidator.LastHeight = 0
		privValidator.LastRound = 0
		privValidator.LastStep = 0
		privValidator.Save()
		log.Notice("Reset PrivValidator", "file", privValidatorFile)
	} else {
		privValidator = types.GenPrivValidator()
		privValidator.SetFile(privValidatorFile)
		privValidator.Save()
		log.Notice("Generated PrivValidator", "file", privValidatorFile)
	}
}
