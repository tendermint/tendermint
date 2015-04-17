package types

import (
	"fmt"
)

// Functions to generate eventId strings

func EventStringAccInput(addr []byte) string {
	return fmt.Sprintf("Acc/%x/Input", addr)
}

func EventStringAccOutput(addr []byte) string {
	return fmt.Sprintf("Acc/%x/Output", addr)
}

func EventStringAccReceive(addr []byte) string {
	return fmt.Sprintf("Acc/%x/Receive", addr)
}

func EventStringBond() string {
	return "Bond"
}

func EventStringUnbond() string {
	return "Unbond"
}

func EventStringRebond() string {
	return "Rebond"
}

func EventStringDupeout() string {
	return "Dupeout"
}

func EventStringNewBlock() string {
	return "NewBlock"
}

func EventStringFork() string {
	return "Fork"
}

// Most event messages are basic types (a block, a transaction)
// but some (an input to a call tx or a receive) are more exotic:

type EventMsgCallTx struct {
	Tx        Tx
	Return    []byte
	Exception string
}

type CallData struct {
	Caller []byte
	Callee []byte
	Data   []byte
	Value  uint64
	Gas    uint64
}

type EventMsgCall struct {
	CallData  *CallData
	Origin    []byte
	TxId      []byte
	Return    []byte
	Exception string
}

/*
Acc/XYZ/Input -> full tx or {full tx, return value, exception}
Acc/XYZ/Output -> full tx
Acc/XYZ/Receive -> full tx, return value, exception, (optionally?) calldata
Bond -> full tx
Unbond -> full tx
Rebond -> full tx
Dupeout -> full tx
NewBlock -> full block
Fork -> block A, block B

Log -> Fuck this
NewPeer -> peer
Alert -> alert msg
*/
