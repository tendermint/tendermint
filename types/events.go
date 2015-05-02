package types

import (
	"fmt"
)

// Functions to generate eventId strings

func EventStringAccInput(addr []byte) string {
	return fmt.Sprintf("Acc/%X/Input", addr)
}

func EventStringAccOutput(addr []byte) string {
	return fmt.Sprintf("Acc/%X/Output", addr)
}

func EventStringAccReceive(addr []byte) string {
	return fmt.Sprintf("Acc/%X/Receive", addr)
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
	Tx        Tx     `json:"tx"`
	Return    []byte `json:"return"`
	Exception string `json:"exception"`
}

type CallData struct {
	Caller []byte `json:"caller"`
	Callee []byte `json:"callee"`
	Data   []byte `json:"data"`
	Value  uint64 `json:"value"`
	Gas    uint64 `json:"gas"`
}

type EventMsgCall struct {
	CallData  *CallData `json:"call_data"`
	Origin    []byte    `json:"origin"`
	TxId      []byte    `json:"tx_id"`
	Return    []byte    `json:"return"`
	Exception string    `json:"exception"`
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
