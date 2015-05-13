package main

import (
	"os"

	"github.com/tendermint/tendermint/config"
	sm "github.com/tendermint/tendermint/state"
)

// NOTE: this is totally unsafe.
// it's only suitable for testnets.
func reset_priv_validator() {
	// Get PrivValidator
	var privValidator *sm.PrivValidator
	privValidatorFile := config.App().GetString("priv_validator_file")
	if _, err := os.Stat(privValidatorFile); err == nil {
		privValidator = sm.LoadPrivValidator(privValidatorFile)
		privValidator.LastHeight = 0
		privValidator.LastRound = 0
		privValidator.LastStep = 0
		privValidator.Save()
		log.Info("Reset PrivValidator", "file", privValidatorFile)
	} else {
		privValidator = sm.GenPrivValidator()
		privValidator.SetFile(privValidatorFile)
		privValidator.Save()
		log.Info("Generated PrivValidator", "file", privValidatorFile)
	}
}
