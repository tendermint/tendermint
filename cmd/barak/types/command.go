package types

import (
	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/wire"
)

type AuthCommand struct {
	CommandJSONStr string
	Signatures     []acm.Signature
}

type NoncedCommand struct {
	Nonce int64
	Command
}

type Command interface{}

const (
	commandTypeStartProcess  = 0x01
	commandTypeStopProcess   = 0x02
	commandTypeListProcesses = 0x03
	commandTypeServeFile     = 0x04
	commandTypeOpenListener  = 0x05
	commandTypeCloseListener = 0x06
	commandTypeQuit          = 0x07
)

// for wire.readReflect
var _ = wire.RegisterInterface(
	struct{ Command }{},
	wire.ConcreteType{CommandStartProcess{}, commandTypeStartProcess},
	wire.ConcreteType{CommandStopProcess{}, commandTypeStopProcess},
	wire.ConcreteType{CommandListProcesses{}, commandTypeListProcesses},
	wire.ConcreteType{CommandServeFile{}, commandTypeServeFile},
	wire.ConcreteType{CommandOpenListener{}, commandTypeOpenListener},
	wire.ConcreteType{CommandCloseListener{}, commandTypeCloseListener},
	wire.ConcreteType{CommandQuit{}, commandTypeQuit},
)

type CommandStartProcess struct {
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

type CommandOpenListener struct {
	Addr string
}

type CommandCloseListener struct {
	Addr string
}

type CommandQuit struct{}
