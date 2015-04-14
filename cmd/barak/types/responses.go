package types

import (
	pcm "github.com/tendermint/tendermint/process"
)

type ResponseRunProcess struct {
}

type ResponseStopProcess struct {
}

type ResponseListProcesses struct {
	Processes []*pcm.Process
}
