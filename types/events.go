package types

import (
	"fmt"

	amino "github.com/tendermint/go-amino"
	abci "github.com/tendermint/tendermint/abci/types"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
)

// Reserved event types (alphabetically sorted).
const (
	EventCompleteProposal    = "CompleteProposal"
	EventLock                = "Lock"
	EventNewBlock            = "NewBlock"
	EventNewBlockHeader      = "NewBlockHeader"
	EventNewRound            = "NewRound"
	EventNewRoundStep        = "NewRoundStep"
	EventPolka               = "Polka"
	EventRelock              = "Relock"
	EventTimeoutPropose      = "TimeoutPropose"
	EventTimeoutWait         = "TimeoutWait"
	EventTx                  = "Tx"
	EventUnlock              = "Unlock"
	EventValidBlock          = "ValidBlock"
	EventValidatorSetUpdates = "ValidatorSetUpdates"
	EventVote                = "Vote"
)

///////////////////////////////////////////////////////////////////////////////
// ENCODING / DECODING
///////////////////////////////////////////////////////////////////////////////

// TMEventData implements events.EventData.
type TMEventData interface {
	// empty interface
}

func RegisterEventDatas(cdc *amino.Codec) {
	cdc.RegisterInterface((*TMEventData)(nil), nil)
	cdc.RegisterConcrete(EventDataNewBlock{}, "tendermint/event/NewBlock", nil)
	cdc.RegisterConcrete(EventDataNewBlockHeader{}, "tendermint/event/NewBlockHeader", nil)
	cdc.RegisterConcrete(EventDataTx{}, "tendermint/event/Tx", nil)
	cdc.RegisterConcrete(EventDataRoundState{}, "tendermint/event/RoundState", nil)
	cdc.RegisterConcrete(EventDataNewRound{}, "tendermint/event/NewRound", nil)
	cdc.RegisterConcrete(EventDataCompleteProposal{}, "tendermint/event/CompleteProposal", nil)
	cdc.RegisterConcrete(EventDataVote{}, "tendermint/event/Vote", nil)
	cdc.RegisterConcrete(EventDataValidatorSetUpdates{}, "tendermint/event/ValidatorSetUpdates", nil)
	cdc.RegisterConcrete(EventDataString(""), "tendermint/event/ProposalString", nil)
}

// Most event messages are basic types (a block, a transaction)
// but some (an input to a call tx or a receive) are more exotic

type EventDataNewBlock struct {
	Block *Block `json:"block"`

	ResultBeginBlock abci.ResponseBeginBlock `json:"result_begin_block"`
	ResultEndBlock   abci.ResponseEndBlock   `json:"result_end_block"`
}

// light weight event for benchmarking
type EventDataNewBlockHeader struct {
	Header Header `json:"header"`

	ResultBeginBlock abci.ResponseBeginBlock `json:"result_begin_block"`
	ResultEndBlock   abci.ResponseEndBlock   `json:"result_end_block"`
}

// All txs fire EventDataTx
type EventDataTx struct {
	TxResult
}

// NOTE: This goes into the replay WAL
type EventDataRoundState struct {
	Height int64  `json:"height"`
	Round  int    `json:"round"`
	Step   string `json:"step"`

	// private, not exposed to websockets
	RoundState interface{} `json:"-"`
}

type ValidatorInfo struct {
	Address Address `json:"address"`
	Index   int     `json:"index"`
}

type EventDataNewRound struct {
	Height int64  `json:"height"`
	Round  int    `json:"round"`
	Step   string `json:"step"`

	Proposer ValidatorInfo `json:"proposer"`
}

type EventDataCompleteProposal struct {
	Height int64  `json:"height"`
	Round  int    `json:"round"`
	Step   string `json:"step"`

	BlockID BlockID `json:"block_id"`
}

type EventDataVote struct {
	Vote *Vote
}

type EventDataString string

type EventDataValidatorSetUpdates struct {
	ValidatorUpdates []*Validator `json:"validator_updates"`
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
	EventQueryCompleteProposal    = QueryForEvent(EventCompleteProposal)
	EventQueryLock                = QueryForEvent(EventLock)
	EventQueryNewBlock            = QueryForEvent(EventNewBlock)
	EventQueryNewBlockHeader      = QueryForEvent(EventNewBlockHeader)
	EventQueryNewRound            = QueryForEvent(EventNewRound)
	EventQueryNewRoundStep        = QueryForEvent(EventNewRoundStep)
	EventQueryPolka               = QueryForEvent(EventPolka)
	EventQueryRelock              = QueryForEvent(EventRelock)
	EventQueryTimeoutPropose      = QueryForEvent(EventTimeoutPropose)
	EventQueryTimeoutWait         = QueryForEvent(EventTimeoutWait)
	EventQueryTx                  = QueryForEvent(EventTx)
	EventQueryUnlock              = QueryForEvent(EventUnlock)
	EventQueryValidatorSetUpdates = QueryForEvent(EventValidatorSetUpdates)
	EventQueryValidBlock          = QueryForEvent(EventValidBlock)
	EventQueryVote                = QueryForEvent(EventVote)
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
	PublishEventValidatorSetUpdates(EventDataValidatorSetUpdates) error
}

type TxEventPublisher interface {
	PublishEventTx(EventDataTx) error
}
