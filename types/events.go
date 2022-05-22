package types

import (
	"fmt"
	"strings"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/jsontypes"
	tmquery "github.com/tendermint/tendermint/internal/pubsub/query"
)

// Reserved event types (alphabetically sorted).
const (
	// Block level events for mass consumption by users.
	// These events are triggered from the state package,
	// after a block has been committed.
	// These are also used by the tx indexer for async indexing.
	// All of this data can be fetched through the rpc.
	EventNewBlockValue            = "NewBlock"
	EventNewBlockHeaderValue      = "NewBlockHeader"
	EventNewEvidenceValue         = "NewEvidence"
	EventTxValue                  = "Tx"
	EventValidatorSetUpdatesValue = "ValidatorSetUpdates"

	// Internal consensus events.
	// These are used for testing the consensus state machine.
	// They can also be used to build real-time consensus visualizers.
	EventCompleteProposalValue = "CompleteProposal"
	// The BlockSyncStatus event will be emitted when the node switching
	// state sync mechanism between the consensus reactor and the blocksync reactor.
	EventBlockSyncStatusValue = "BlockSyncStatus"
	EventLockValue            = "Lock"
	EventNewRoundValue        = "NewRound"
	EventNewRoundStepValue    = "NewRoundStep"
	EventPolkaValue           = "Polka"
	EventRelockValue          = "Relock"
	EventStateSyncStatusValue = "StateSyncStatus"
	EventTimeoutProposeValue  = "TimeoutPropose"
	EventTimeoutWaitValue     = "TimeoutWait"
	EventValidBlockValue      = "ValidBlock"
	EventVoteValue            = "Vote"

	// Events emitted by the evidence reactor when evidence is validated
	// and before it is committed
	EventEvidenceValidatedValue = "EvidenceValidated"
)

// Pre-populated ABCI Tendermint-reserved events
var (
	EventNewBlock = abci.Event{
		Type: strings.Split(EventTypeKey, ".")[0],
		Attributes: []abci.EventAttribute{
			{
				Key:   strings.Split(EventTypeKey, ".")[1],
				Value: EventNewBlockValue,
			},
		},
	}

	EventNewBlockHeader = abci.Event{
		Type: strings.Split(EventTypeKey, ".")[0],
		Attributes: []abci.EventAttribute{
			{
				Key:   strings.Split(EventTypeKey, ".")[1],
				Value: EventNewBlockHeaderValue,
			},
		},
	}

	EventNewEvidence = abci.Event{
		Type: strings.Split(EventTypeKey, ".")[0],
		Attributes: []abci.EventAttribute{
			{
				Key:   strings.Split(EventTypeKey, ".")[1],
				Value: EventNewEvidenceValue,
			},
		},
	}

	EventTx = abci.Event{
		Type: strings.Split(EventTypeKey, ".")[0],
		Attributes: []abci.EventAttribute{
			{
				Key:   strings.Split(EventTypeKey, ".")[1],
				Value: EventTxValue,
			},
		},
	}
)

// ENCODING / DECODING

// EventData is satisfied by types that can be published as event data.
//
// Implementations of this interface that contain ABCI event metadata should
// also implement the eventlog.ABCIEventer extension interface to expose those
// metadata to the event log machinery. Event data that do not contain ABCI
// metadata can safely omit this.
type EventData interface {
	// The value must support encoding as a type-tagged JSON object.
	jsontypes.Tagged
}

func init() {
	jsontypes.MustRegister(EventDataBlockSyncStatus{})
	jsontypes.MustRegister(EventDataCompleteProposal{})
	jsontypes.MustRegister(EventDataNewBlock{})
	jsontypes.MustRegister(EventDataNewBlockHeader{})
	jsontypes.MustRegister(EventDataNewEvidence{})
	jsontypes.MustRegister(EventDataNewRound{})
	jsontypes.MustRegister(EventDataRoundState{})
	jsontypes.MustRegister(EventDataStateSyncStatus{})
	jsontypes.MustRegister(EventDataTx{})
	jsontypes.MustRegister(EventDataValidatorSetUpdates{})
	jsontypes.MustRegister(EventDataVote{})
	jsontypes.MustRegister(EventDataEvidenceValidated{})
	jsontypes.MustRegister(EventDataString(""))
}

// Most event messages are basic types (a block, a transaction)
// but some (an input to a call tx or a receive) are more exotic

type EventDataNewBlock struct {
	Block   *Block  `json:"block"`
	BlockID BlockID `json:"block_id"`

	ResultFinalizeBlock abci.ResponseFinalizeBlock `json:"result_finalize_block"`
}

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataNewBlock) TypeTag() string { return "tendermint/event/NewBlock" }

// ABCIEvents implements the eventlog.ABCIEventer interface.
func (e EventDataNewBlock) ABCIEvents() []abci.Event {
	base := []abci.Event{eventWithAttr(BlockHeightKey, fmt.Sprint(e.Block.Header.Height))}
	return append(base, e.ResultFinalizeBlock.Events...)
}

type EventDataNewBlockHeader struct {
	Header Header `json:"header"`

	NumTxs              int64                      `json:"num_txs,string"` // Number of txs in a block
	ResultFinalizeBlock abci.ResponseFinalizeBlock `json:"result_finalize_block"`
}

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataNewBlockHeader) TypeTag() string { return "tendermint/event/NewBlockHeader" }

// ABCIEvents implements the eventlog.ABCIEventer interface.
func (e EventDataNewBlockHeader) ABCIEvents() []abci.Event {
	base := []abci.Event{eventWithAttr(BlockHeightKey, fmt.Sprint(e.Header.Height))}
	return append(base, e.ResultFinalizeBlock.Events...)
}

type EventDataNewEvidence struct {
	Evidence Evidence `json:"evidence"`

	Height int64 `json:"height,string"`
}

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataNewEvidence) TypeTag() string { return "tendermint/event/NewEvidence" }

// All txs fire EventDataTx
type EventDataTx struct {
	abci.TxResult
}

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataTx) TypeTag() string { return "tendermint/event/Tx" }

// ABCIEvents implements the eventlog.ABCIEventer interface.
func (e EventDataTx) ABCIEvents() []abci.Event {
	base := []abci.Event{
		eventWithAttr(TxHashKey, fmt.Sprintf("%X", Tx(e.Tx).Hash())),
		eventWithAttr(TxHeightKey, fmt.Sprintf("%d", e.Height)),
	}
	return append(base, e.Result.Events...)
}

// NOTE: This goes into the replay WAL
type EventDataRoundState struct {
	Height int64  `json:"height,string"`
	Round  int32  `json:"round"`
	Step   string `json:"step"`
}

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataRoundState) TypeTag() string { return "tendermint/event/RoundState" }

type ValidatorInfo struct {
	Address Address `json:"address"`
	Index   int32   `json:"index"`
}

type EventDataNewRound struct {
	Height int64  `json:"height,string"`
	Round  int32  `json:"round"`
	Step   string `json:"step"`

	Proposer ValidatorInfo `json:"proposer"`
}

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataNewRound) TypeTag() string { return "tendermint/event/NewRound" }

type EventDataCompleteProposal struct {
	Height int64  `json:"height,string"`
	Round  int32  `json:"round"`
	Step   string `json:"step"`

	BlockID BlockID `json:"block_id"`
}

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataCompleteProposal) TypeTag() string { return "tendermint/event/CompleteProposal" }

type EventDataVote struct {
	Vote *Vote
}

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataVote) TypeTag() string { return "tendermint/event/Vote" }

type EventDataString string

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataString) TypeTag() string { return "tendermint/event/ProposalString" }

type EventDataValidatorSetUpdates struct {
	ValidatorUpdates []*Validator `json:"validator_updates"`
}

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataValidatorSetUpdates) TypeTag() string { return "tendermint/event/ValidatorSetUpdates" }

// EventDataBlockSyncStatus shows the fastsync status and the
// height when the node state sync mechanism changes.
type EventDataBlockSyncStatus struct {
	Complete bool  `json:"complete"`
	Height   int64 `json:"height,string"`
}

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataBlockSyncStatus) TypeTag() string { return "tendermint/event/FastSyncStatus" }

// EventDataStateSyncStatus shows the statesync status and the
// height when the node state sync mechanism changes.
type EventDataStateSyncStatus struct {
	Complete bool  `json:"complete"`
	Height   int64 `json:"height,string"`
}

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataStateSyncStatus) TypeTag() string { return "tendermint/event/StateSyncStatus" }

type EventDataEvidenceValidated struct {
	Evidence Evidence `json:"evidence"`

	Height int64 `json:"height,string"`
}

// TypeTag implements the required method of jsontypes.Tagged.
func (EventDataEvidenceValidated) TypeTag() string { return "tendermint/event/EvidenceValidated" }

// PUBSUB

const (
	// EventTypeKey is a reserved composite key for event name.
	EventTypeKey = "tm.event"

	// TxHashKey is a reserved key, used to specify transaction's hash.
	// see EventBus#PublishEventTx
	TxHashKey = "tx.hash"

	// TxHeightKey is a reserved key, used to specify transaction block's height.
	// see EventBus#PublishEventTx
	TxHeightKey = "tx.height"

	// BlockHeightKey is a reserved key used for indexing FinalizeBlock events.
	BlockHeightKey = "block.height"
)

var (
	EventQueryCompleteProposal    = QueryForEvent(EventCompleteProposalValue)
	EventQueryLock                = QueryForEvent(EventLockValue)
	EventQueryNewBlock            = QueryForEvent(EventNewBlockValue)
	EventQueryNewBlockHeader      = QueryForEvent(EventNewBlockHeaderValue)
	EventQueryNewEvidence         = QueryForEvent(EventNewEvidenceValue)
	EventQueryNewRound            = QueryForEvent(EventNewRoundValue)
	EventQueryNewRoundStep        = QueryForEvent(EventNewRoundStepValue)
	EventQueryPolka               = QueryForEvent(EventPolkaValue)
	EventQueryRelock              = QueryForEvent(EventRelockValue)
	EventQueryTimeoutPropose      = QueryForEvent(EventTimeoutProposeValue)
	EventQueryTimeoutWait         = QueryForEvent(EventTimeoutWaitValue)
	EventQueryTx                  = QueryForEvent(EventTxValue)
	EventQueryValidatorSetUpdates = QueryForEvent(EventValidatorSetUpdatesValue)
	EventQueryValidBlock          = QueryForEvent(EventValidBlockValue)
	EventQueryVote                = QueryForEvent(EventVoteValue)
	EventQueryBlockSyncStatus     = QueryForEvent(EventBlockSyncStatusValue)
	EventQueryStateSyncStatus     = QueryForEvent(EventStateSyncStatusValue)
	EventQueryEvidenceValidated   = QueryForEvent(EventEvidenceValidatedValue)
)

func EventQueryTxFor(tx Tx) *tmquery.Query {
	return tmquery.MustCompile(fmt.Sprintf("%s='%s' AND %s='%X'", EventTypeKey, EventTxValue, TxHashKey, tx.Hash()))
}

func QueryForEvent(eventValue string) *tmquery.Query {
	return tmquery.MustCompile(fmt.Sprintf("%s='%s'", EventTypeKey, eventValue))
}

// BlockEventPublisher publishes all block related events
type BlockEventPublisher interface {
	PublishEventNewBlock(EventDataNewBlock) error
	PublishEventNewBlockHeader(EventDataNewBlockHeader) error
	PublishEventNewEvidence(EventDataNewEvidence) error
	PublishEventTx(EventDataTx) error
	PublishEventValidatorSetUpdates(EventDataValidatorSetUpdates) error
}

type TxEventPublisher interface {
	PublishEventTx(EventDataTx) error
}

// eventWithAttr constructs a single abci.Event with a single attribute.
// The type of the event and the name of the attribute are obtained by
// splitting the event type on period (e.g., "foo.bar").
func eventWithAttr(etype, value string) abci.Event {
	parts := strings.SplitN(etype, ".", 2)
	return abci.Event{
		Type: parts[0],
		Attributes: []abci.EventAttribute{{
			Key: parts[1], Value: value,
		}},
	}
}
