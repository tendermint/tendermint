package main

import (
	"fmt"

	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/config"
	sm "github.com/tendermint/tendermint/state"
)

func gen_validator() {

	privValidator := sm.GenPrivValidator()
	privValidatorJSONBytes := binary.JSONBytes(privValidator)
	fmt.Printf(`Generated a new validator!
Paste the following JSON into your %v file

%v

`,
		config.App().GetString("PrivValidatorFile"),
		string(privValidatorJSONBytes),
	)
}
