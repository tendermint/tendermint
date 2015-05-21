package types

import (
	pcm "github.com/tendermint/tendermint/process"
)

type ResponseStatus struct {
	Pid        int
	Nonce      uint64
	Validators []Validator
}

type ResponseRegister struct {
}

type ResponseStartProcess struct {
	Success bool
	Output  string
}

type ResponseStopProcess struct {
}

type ResponseListProcesses struct {
	Processes []*pcm.Process
}

type ResponseOpenListener struct {
	Addr string
}

type ResponseCloseListener struct {
}
