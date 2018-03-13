package types

import (
	"fmt"

	"github.com/tendermint/tendermint/amino"
	tmpubsub "github.com/tendermint/tmlibs/pubsub"
	tmquery "github.com/tendermint/tmlibs/pubsub/query"
)

// Reserved event types
const (
	EventBond              = "Bond"
	EventCompleteProposal  = "CompleteProposal"
	EventDupeout           = "Dupeout"
	EventFork              = "Fork"
	EventLock              = "Lock"
	EventNewBlock          = "NewBlock"
	EventNewBlockHeader    = "NewBlockHeader"
	EventNewRound          = "NewRound"
	EventNewRoundStep      = "NewRoundStep"
	EventPolka             = "Polka"
	EventRebond            = "Rebond"
	EventRelock            = "Relock"
	EventTimeoutPropose    = "TimeoutPropose"
	EventTimeoutWait       = "TimeoutWait"
	EventTx                = "Tx"
	EventUnbond            = "Unbond"
	EventUnlock            = "Unlock"
	EventVote              = "Vote"
	EventProposalHeartbeat = "ProposalHeartbeat"
)

///////////////////////////////////////////////////////////////////////////////
// ENCODING / DECODING
///////////////////////////////////////////////////////////////////////////////

var (
	EventDataNameNewBlock          = "new_block"
	EventDataNameNewBlockHeader    = "new_block_header"
	EventDataNameTx                = "tx"
	EventDataNameRoundState        = "round_state"
	EventDataNameVote              = "vote"
	EventDataNameProposalHeartbeat = "proposal_heartbeat"
)

// implements events.EventData
type TMEventData interface {
	// empty interface
}

const (
	EventDataTypeNewBlock          = byte(0x01)
	EventDataTypeFork              = byte(0x02)
	EventDataTypeTx                = byte(0x03)
	EventDataTypeNewBlockHeader    = byte(0x04)
	EventDataTypeRoundState        = byte(0x11)
	EventDataTypeVote              = byte(0x12)
	EventDataTypeProposalHeartbeat = byte(0x20)
)

func init() {
	amino.RegisterInterface((*TMEventData)(nil), nil)
	amino.RegisterConcrete(EventDataNewBlock{},
		"com.tendermint.events.new_block", nil)
	amino.RegisterConcrete(EventDataNewBlockHeader{},
		"com.tendermint.events.new_block_header", nil)
	amino.RegisterConcrete(EventDataTx{},
		"com.tendermint.events.tx", nil)
	amino.RegisterConcrete(EventDataRoundState{},
		"com.tendermint.events.round_state", nil)
	amino.RegisterConcrete(EventDataVote{},
		"com.tendermint.events.vote", nil)
	amino.RegisterConcrete(EventDataProposalHeartbeat{},
		"com.tendermint.events.proposal_heartbeat", nil)
}

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
	TxResult
}

type EventDataProposalHeartbeat struct {
	Heartbeat *Heartbeat
}

// NOTE: This goes into the replay WAL
type EventDataRoundState struct {
	Height int64  `json:"height"`
	Round  int    `json:"round"`
	Step   string `json:"step"`

	// private, not exposed to websockets
	RoundState interface{} `json:"-"`
}

type EventDataVote struct {
	Vote *Vote
}

///////////////////////////////////////////////////////////////////////////////
// PUBSUB
///////////////////////////////////////////////////////////////////////////////

const (
	// EventTypeKey is a reserved key, used to specify event type in tags.
	EventTypeKey = "tm.event"
	// TxHashKey is a reserved key, used to specify transaction's hash.
	// see EventBus#PublishEventTx
	TxHashKey = "tx.hash"
	// TxHeightKey is a reserved key, used to specify transaction block's height.
	// see EventBus#PublishEventTx
	TxHeightKey = "tx.height"
)

var (
	EventQueryBond              = QueryForEvent(EventBond)
	EventQueryUnbond            = QueryForEvent(EventUnbond)
	EventQueryRebond            = QueryForEvent(EventRebond)
	EventQueryDupeout           = QueryForEvent(EventDupeout)
	EventQueryFork              = QueryForEvent(EventFork)
	EventQueryNewBlock          = QueryForEvent(EventNewBlock)
	EventQueryNewBlockHeader    = QueryForEvent(EventNewBlockHeader)
	EventQueryNewRound          = QueryForEvent(EventNewRound)
	EventQueryNewRoundStep      = QueryForEvent(EventNewRoundStep)
	EventQueryTimeoutPropose    = QueryForEvent(EventTimeoutPropose)
	EventQueryCompleteProposal  = QueryForEvent(EventCompleteProposal)
	EventQueryPolka             = QueryForEvent(EventPolka)
	EventQueryUnlock            = QueryForEvent(EventUnlock)
	EventQueryLock              = QueryForEvent(EventLock)
	EventQueryRelock            = QueryForEvent(EventRelock)
	EventQueryTimeoutWait       = QueryForEvent(EventTimeoutWait)
	EventQueryVote              = QueryForEvent(EventVote)
	EventQueryProposalHeartbeat = QueryForEvent(EventProposalHeartbeat)
	EventQueryTx                = QueryForEvent(EventTx)
)

func EventQueryTxFor(tx Tx) tmpubsub.Query {
	return tmquery.MustParse(fmt.Sprintf("%s='%s' AND %s='%X'", EventTypeKey, EventTx, TxHashKey, tx.Hash()))
}

func QueryForEvent(eventType string) tmpubsub.Query {
	return tmquery.MustParse(fmt.Sprintf("%s='%s'", EventTypeKey, eventType))
}

// BlockEventPublisher publishes all block related events
type BlockEventPublisher interface {
	PublishEventNewBlock(block EventDataNewBlock) error
	PublishEventNewBlockHeader(header EventDataNewBlockHeader) error
	PublishEventTx(EventDataTx) error
}

type TxEventPublisher interface {
	PublishEventTx(EventDataTx) error
}
