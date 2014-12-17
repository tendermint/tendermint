package main

import (
	"fmt"
	"os"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/state"
)

func gen_validator() {

	// If already exists, bail out.
	filename := config.PrivValidatorFile()
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		fmt.Printf("Cannot generate new validator, file already exists at %v\n", filename)
	}

	// Generate private validator
	privValidator := state.GenPrivValidator()
	privValidator.Save()
	fmt.Printf("Generated a new validator at %v\n", filename)
}
