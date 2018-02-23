package types

import (
	"fmt"

	tmpubsub "github.com/tendermint/tmlibs/pubsub"
	tmquery "github.com/tendermint/tmlibs/pubsub/query"

	"github.com/tendermint/tendermint/wire"
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

// TMEventData implements events.EventData
type TMEventData interface {
	// empty interface
}

func init() {
	wire.RegisterInterface((*TMEventData)(nil), nil)
	wire.RegisterConcrete(EventDataNewBlock{}, "com.tendermint.types.event_new_block", nil)
	wire.RegisterConcrete(EventDataNewBlockHeader{}, "com.tendermint.types.event_new_block_header", nil)
	wire.RegisterConcrete(EventDataTx{}, "com.tendermint.types.event_tx", nil)
	wire.RegisterConcrete(EventDataRoundState{}, "com.tendermint.types.event_round_state", nil)
	wire.RegisterConcrete(EventDataVote{}, "com.tendermint.types.event_vote", nil)
	wire.RegisterConcrete(EventDataProposalHeartbeat{}, "com.tendermint.types.event_proposal_heartbeat", nil)
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
