package types

import (
	pcm "github.com/eris-ltd/tendermint/process"
)

type ResponseStatus struct {
	Version    string
	Pid        int
	Nonce      int64
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

type ResponseQuit struct {
}
