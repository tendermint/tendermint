package main

import (
	"fmt"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/state"

	. "github.com/tendermint/tendermint/binary"
)

func gen_validator() {

	privValidator := state.GenPrivValidator()
	privValidatorJSONBytes := JSONBytes(privValidator)
	fmt.Printf(`Generated a new validator!
Paste the following JSON into your %v file

%v

`,
		config.App.GetString("PrivValidatorFile"),
		string(privValidatorJSONBytes),
	)
}
