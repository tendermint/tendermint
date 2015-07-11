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

func EventStringLogEvent(addr []byte) string {
	return fmt.Sprintf("Log/%X", addr)
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
	Value  int64  `json:"value"`
	Gas    int64  `json:"gas"`
}

type EventMsgCall struct {
	CallData  *CallData `json:"call_data"`
	Origin    []byte    `json:"origin"`
	TxID      []byte    `json:"tx_id"`
	Return    []byte    `json:"return"`
	Exception string    `json:"exception"`
}
