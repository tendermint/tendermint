package types

import (
	// for registering TMEventData as events.EventData
	"github.com/tendermint/go-events"
	"github.com/tendermint/go-wire"
)

// Functions to generate eventId strings

// Reserved
func EventStringBond() string    { return "Bond" }
func EventStringUnbond() string  { return "Unbond" }
func EventStringRebond() string  { return "Rebond" }
func EventStringDupeout() string { return "Dupeout" }
func EventStringFork() string    { return "Fork" }

func EventStringNewBlock() string         { return "NewBlock" }
func EventStringNewRound() string         { return "NewRound" }
func EventStringNewRoundStep() string     { return "NewRoundStep" }
func EventStringTimeoutPropose() string   { return "TimeoutPropose" }
func EventStringCompleteProposal() string { return "CompleteProposal" }
func EventStringPolka() string            { return "Polka" }
func EventStringUnlock() string           { return "Unlock" }
func EventStringLock() string             { return "Lock" }
func EventStringRelock() string           { return "Relock" }
func EventStringTimeoutWait() string      { return "TimeoutWait" }
func EventStringVote() string             { return "Vote" }

//----------------------------------------

// implements events.EventData
type TMEventData interface {
	events.EventData
	//	AssertIsTMEventData()
}

const (
	EventDataTypeNewBlock = byte(0x01)
	EventDataTypeFork     = byte(0x02)
	EventDataTypeTx       = byte(0x03)

	EventDataTypeRoundState = byte(0x11)
	EventDataTypeVote       = byte(0x12)
)

var _ = wire.RegisterInterface(
	struct{ TMEventData }{},
	wire.ConcreteType{EventDataNewBlock{}, EventDataTypeNewBlock},
	// wire.ConcreteType{EventDataFork{}, EventDataTypeFork },
	wire.ConcreteType{EventDataTx{}, EventDataTypeTx},
	wire.ConcreteType{EventDataRoundState{}, EventDataTypeRoundState},
	wire.ConcreteType{EventDataVote{}, EventDataTypeVote},
)

// Most event messages are basic types (a block, a transaction)
// but some (an input to a call tx or a receive) are more exotic

type EventDataNewBlock struct {
	Block *Block `json:"block"`
}

// All txs fire EventDataTx
type EventDataTx struct {
	Tx     Tx     `json:"tx"`
	Result []byte `json:"result"`
	Log    string `json:"log"`
	Error  string `json:"error"`
}

// NOTE: This goes into the replay WAL
type EventDataRoundState struct {
	Height int    `json:"height"`
	Round  int    `json:"round"`
	Step   string `json:"step"`

	// private, not exposed to websockets
	RoundState interface{} `json:"-"`
}

type EventDataVote struct {
	Index   int
	Address []byte
	Vote    *Vote
}

func (_ EventDataNewBlock) AssertIsTMEventData()   {}
func (_ EventDataTx) AssertIsTMEventData()         {}
func (_ EventDataRoundState) AssertIsTMEventData() {}
func (_ EventDataVote) AssertIsTMEventData()       {}
