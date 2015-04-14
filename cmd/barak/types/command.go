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

// for binary.readReflect
var _ = binary.RegisterInterface(
	struct{ Command }{},
	binary.ConcreteType{CommandRunProcess{}},
	binary.ConcreteType{CommandStopProcess{}},
	binary.ConcreteType{CommandListProcesses{}},
	binary.ConcreteType{CommandServeFile{}},
)

const (
	typeByteRunProcess    = 0x01
	typeByteStopProcess   = 0x02
	typeByteListProcesses = 0x03
	typeByteServeFile     = 0x04
)

// TODO: This is actually not cleaner than a method call.
// In fact, this is stupid.
func (_ CommandRunProcess) TypeByte() byte    { return typeByteRunProcess }
func (_ CommandStopProcess) TypeByte() byte   { return typeByteStopProcess }
func (_ CommandListProcesses) TypeByte() byte { return typeByteListProcesses }
func (_ CommandServeFile) TypeByte() byte     { return typeByteServeFile }

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
