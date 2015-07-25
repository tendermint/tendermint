package main

import (
	"fmt"

	"github.com/tendermint/tendermint/wire"
	sm "github.com/tendermint/tendermint/state"
)

func gen_validator() {

	privValidator := sm.GenPrivValidator()
	privValidatorJSONBytes := wire.JSONBytes(privValidator)
	fmt.Printf(`Generated a new validator!
Paste the following JSON into your %v file

%v

`,
		config.GetString("priv_validator_file"),
		string(privValidatorJSONBytes),
	)
}
