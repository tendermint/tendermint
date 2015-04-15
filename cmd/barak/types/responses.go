package types

import (
	pcm "github.com/tendermint/tendermint/process"
)

type ResponseStatus struct {
	Nonce      uint64
	Validators []Validator
}

type ResponseRunProcess struct {
}

type ResponseStopProcess struct {
}

type ResponseListProcesses struct {
	Processes []*pcm.Process
}
