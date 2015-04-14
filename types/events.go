package types

import (
	"fmt"
)

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
