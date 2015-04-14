package types

import (
	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
)

type AuthCommand struct {
	CommandJSONStr string
	Signatures     []acm.Signature
}

type NoncedCommand struct {
	Nonce uint64
	Command
}

type Command interface{}

const (
	commandTypeRunProcess    = 0x01
	commandTypeStopProcess   = 0x02
	commandTypeListProcesses = 0x03
	commandTypeServeFile     = 0x04
)

// for binary.readReflect
var _ = binary.RegisterInterface(
	struct{ Command }{},
	binary.ConcreteType{CommandRunProcess{}, commandTypeRunProcess},
	binary.ConcreteType{CommandStopProcess{}, commandTypeStopProcess},
	binary.ConcreteType{CommandListProcesses{}, commandTypeListProcesses},
	binary.ConcreteType{CommandServeFile{}, commandTypeServeFile},
)

type CommandRunProcess struct {
	Wait     bool
	Label    string
	ExecPath string
	Args     []string
	Input    string
}

type CommandStopProcess struct {
	Label string
	Kill  bool
}

type CommandListProcesses struct{}

type CommandServeFile struct {
	Path string
}
