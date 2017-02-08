package main

import (
	"fmt"

	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

func gen_validator() {
	privValidator := types.GenPrivValidator()
	privValidatorJSONBytes := wire.JSONBytesPretty(privValidator)
	fmt.Printf(`%v
`, string(privValidatorJSONBytes))
}
