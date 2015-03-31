package main

import (
	"fmt"

	"github.com/tendermint/tendermint2/binary"
	"github.com/tendermint/tendermint2/config"
	sm "github.com/tendermint/tendermint2/state"
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
