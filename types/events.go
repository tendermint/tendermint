package types

import (
	"fmt"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/wire"
)

// Functions to generate eventId strings

func EventStringAccInput(addr []byte) string    { return fmt.Sprintf("Acc/%X/Input", addr) }
func EventStringAccOutput(addr []byte) string   { return fmt.Sprintf("Acc/%X/Output", addr) }
func EventStringAccCall(addr []byte) string     { return fmt.Sprintf("Acc/%X/Call", addr) }
func EventStringLogEvent(addr []byte) string    { return fmt.Sprintf("Log/%X", addr) }
func EventStringPermissions(name string) string { return fmt.Sprintf("Permissions/%s", name) }
func EventStringNameReg(name string) string     { return fmt.Sprintf("NameReg/%s", name) }
func EventStringBond() string                   { return "Bond" }
func EventStringUnbond() string                 { return "Unbond" }
func EventStringRebond() string                 { return "Rebond" }
func EventStringDupeout() string                { return "Dupeout" }
func EventStringNewBlock() string               { return "NewBlock" }
func EventStringFork() string                   { return "Fork" }

//----------------------------------------

const (
	EventDataTypeNewBlock = byte(0x01)
	EventDataTypeFork     = byte(0x02)
	EventDataTypeTx       = byte(0x03)
	EventDataTypeCall     = byte(0x04)
	EventDataTypeLog      = byte(0x05)
)

type EventData interface {
	AssertIsEventData()
}

var _ = wire.RegisterInterface(
	struct{ EventData }{},
	wire.ConcreteType{EventDataNewBlock{}, EventDataTypeNewBlock},
	// wire.ConcreteType{EventDataFork{}, EventDataTypeFork },
	wire.ConcreteType{EventDataTx{}, EventDataTypeTx},
	wire.ConcreteType{EventDataCall{}, EventDataTypeCall},
	wire.ConcreteType{EventDataLog{}, EventDataTypeLog},
)

// Most event messages are basic types (a block, a transaction)
// but some (an input to a call tx or a receive) are more exotic:

type EventDataNewBlock struct {
	Block *Block `json:"block"`
}

// All txs fire EventDataTx, but only CallTx might have Return or Exception
type EventDataTx struct {
	Tx        Tx     `json:"tx"`
	Return    []byte `json:"return"`
	Exception string `json:"exception"`
}

// EventDataCall fires when we call a contract, and when a contract calls another contract
type EventDataCall struct {
	CallData  *CallData `json:"call_data"`
	Origin    []byte    `json:"origin"`
	TxID      []byte    `json:"tx_id"`
	Return    []byte    `json:"return"`
	Exception string    `json:"exception"`
}

type CallData struct {
	Caller []byte `json:"caller"`
	Callee []byte `json:"callee"`
	Data   []byte `json:"data"`
	Value  int64  `json:"value"`
	Gas    int64  `json:"gas"`
}

// EventDataLog fires when a contract executes the LOG opcode
type EventDataLog struct {
	Address Word256   `json:"address"`
	Topics  []Word256 `json:"topics"`
	Data    []byte    `json:"data"`
	Height  int64     `json:"height"`
}

func (_ EventDataNewBlock) AssertIsEventData() {}
func (_ EventDataTx) AssertIsEventData()       {}
func (_ EventDataCall) AssertIsEventData()     {}
func (_ EventDataLog) AssertIsEventData()      {}
