package types

import (
	// for registering TMEventData as events.EventData
	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/events"
)

// Functions to generate eventId strings

// Reserved
func EventStringBond() string    { return "Bond" }
func EventStringUnbond() string  { return "Unbond" }
func EventStringRebond() string  { return "Rebond" }
func EventStringDupeout() string { return "Dupeout" }
func EventStringFork() string    { return "Fork" }
func EventStringTx(tx Tx) string { return cmn.Fmt("Tx:%X", tx.Hash()) }

func EventStringNewBlock() string         { return "NewBlock" }
func EventStringNewBlockHeader() string   { return "NewBlockHeader" }
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

var (
	EventDataNameNewBlock       = "new_block"
	EventDataNameNewBlockHeader = "new_block_header"
	EventDataNameTx             = "tx"
	EventDataNameRoundState     = "round_state"
	EventDataNameVote           = "vote"
)

//----------------------------------------

// implements events.EventData
type TMEventDataInner interface {
	events.EventData
}

type TMEventData struct {
	TMEventDataInner `json:"unwrap"`
}

func (tmr TMEventData) MarshalJSON() ([]byte, error) {
	return tmEventDataMapper.ToJSON(tmr.TMEventDataInner)
}

func (tmr *TMEventData) UnmarshalJSON(data []byte) (err error) {
	parsed, err := tmEventDataMapper.FromJSON(data)
	if err == nil && parsed != nil {
		tmr.TMEventDataInner = parsed.(TMEventDataInner)
	}
	return
}

func (tmr TMEventData) Unwrap() TMEventDataInner {
	tmrI := tmr.TMEventDataInner
	for wrap, ok := tmrI.(TMEventData); ok; wrap, ok = tmrI.(TMEventData) {
		tmrI = wrap.TMEventDataInner
	}
	return tmrI
}

func (tmr TMEventData) Empty() bool {
	return tmr.TMEventDataInner == nil
}

const (
	EventDataTypeNewBlock       = byte(0x01)
	EventDataTypeFork           = byte(0x02)
	EventDataTypeTx             = byte(0x03)
	EventDataTypeNewBlockHeader = byte(0x04)

	EventDataTypeRoundState = byte(0x11)
	EventDataTypeVote       = byte(0x12)
)

var tmEventDataMapper = data.NewMapper(TMEventData{}).
	RegisterImplementation(EventDataNewBlock{}, EventDataNameNewBlock, EventDataTypeNewBlock).
	RegisterImplementation(EventDataNewBlockHeader{}, EventDataNameNewBlockHeader, EventDataTypeNewBlockHeader).
	RegisterImplementation(EventDataTx{}, EventDataNameTx, EventDataTypeTx).
	RegisterImplementation(EventDataRoundState{}, EventDataNameRoundState, EventDataTypeRoundState).
	RegisterImplementation(EventDataVote{}, EventDataNameVote, EventDataTypeVote)

// Most event messages are basic types (a block, a transaction)
// but some (an input to a call tx or a receive) are more exotic

type EventDataNewBlock struct {
	Block *Block `json:"block"`
}

// light weight event for benchmarking
type EventDataNewBlockHeader struct {
	Header *Header `json:"header"`
}

// All txs fire EventDataTx
type EventDataTx struct {
	Height int           `json:"height"`
	Tx     Tx            `json:"tx"`
	Data   data.Bytes    `json:"data"`
	Log    string        `json:"log"`
	Code   abci.CodeType `json:"code"`
	Error  string        `json:"error"` // this is redundant information for now
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
	Vote *Vote
}

func (_ EventDataNewBlock) AssertIsTMEventData()       {}
func (_ EventDataNewBlockHeader) AssertIsTMEventData() {}
func (_ EventDataTx) AssertIsTMEventData()             {}
func (_ EventDataRoundState) AssertIsTMEventData()     {}
func (_ EventDataVote) AssertIsTMEventData()           {}

//----------------------------------------
// Wrappers for type safety

type Fireable interface {
	events.Fireable
}

type Eventable interface {
	SetEventSwitch(EventSwitch)
}

type EventSwitch interface {
	events.EventSwitch
}

type EventCache interface {
	Fireable
	Flush()
}

func NewEventSwitch() EventSwitch {
	return events.NewEventSwitch()
}

func NewEventCache(evsw EventSwitch) EventCache {
	return events.NewEventCache(evsw)
}

// All events should be based on this FireEvent to ensure they are TMEventData
func fireEvent(fireable events.Fireable, event string, data TMEventData) {
	if fireable != nil {
		fireable.FireEvent(event, data)
	}
}

func AddListenerForEvent(evsw EventSwitch, id, event string, cb func(data TMEventData)) {
	evsw.AddListenerForEvent(id, event, func(data events.EventData) {
		cb(data.(TMEventData))
	})

}

//--- block, tx, and vote events

func FireEventNewBlock(fireable events.Fireable, block EventDataNewBlock) {
	fireEvent(fireable, EventStringNewBlock(), TMEventData{block})
}

func FireEventNewBlockHeader(fireable events.Fireable, header EventDataNewBlockHeader) {
	fireEvent(fireable, EventStringNewBlockHeader(), TMEventData{header})
}

func FireEventVote(fireable events.Fireable, vote EventDataVote) {
	fireEvent(fireable, EventStringVote(), TMEventData{vote})
}

func FireEventTx(fireable events.Fireable, tx EventDataTx) {
	fireEvent(fireable, EventStringTx(tx.Tx), TMEventData{tx})
}

//--- EventDataRoundState events

func FireEventNewRoundStep(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringNewRoundStep(), TMEventData{rs})
}

func FireEventTimeoutPropose(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringTimeoutPropose(), TMEventData{rs})
}

func FireEventTimeoutWait(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringTimeoutWait(), TMEventData{rs})
}

func FireEventNewRound(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringNewRound(), TMEventData{rs})
}

func FireEventCompleteProposal(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringCompleteProposal(), TMEventData{rs})
}

func FireEventPolka(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringPolka(), TMEventData{rs})
}

func FireEventUnlock(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringUnlock(), TMEventData{rs})
}

func FireEventRelock(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringRelock(), TMEventData{rs})
}

func FireEventLock(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringLock(), TMEventData{rs})
}
