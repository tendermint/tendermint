package eventbus

import (
	"context"
	"fmt"
	"strings"

	abci "github.com/tendermint/tendermint/abci/types"
	tmpubsub "github.com/tendermint/tendermint/internal/pubsub"
	tmquery "github.com/tendermint/tendermint/internal/pubsub/query"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/types"
)

// Subscription is a proxy interface for a pubsub Subscription.
type Subscription interface {
	ID() string
	Next(context.Context) (tmpubsub.Message, error)
}

// EventBus is a common bus for all events going through the system.
// It is a type-aware wrapper around an underlying pubsub server.
// All events should be published via the bus.
type EventBus struct {
	service.BaseService
	pubsub *tmpubsub.Server
}

// NewDefault returns a new event bus with default options.
func NewDefault(l log.Logger) *EventBus {
	logger := l.With("module", "eventbus")
	pubsub := tmpubsub.NewServer(l, tmpubsub.BufferCapacity(0))
	b := &EventBus{pubsub: pubsub}
	b.BaseService = *service.NewBaseService(logger, "EventBus", b)
	return b
}

func (b *EventBus) OnStart(ctx context.Context) error {
	return b.pubsub.Start(ctx)
}

func (b *EventBus) OnStop() {}

func (b *EventBus) NumClients() int {
	return b.pubsub.NumClients()
}

func (b *EventBus) NumClientSubscriptions(clientID string) int {
	return b.pubsub.NumClientSubscriptions(clientID)
}

// Deprecated: Use SubscribeWithArgs instead.
func (b *EventBus) Subscribe(ctx context.Context,
	clientID string, query *tmquery.Query, capacities ...int) (Subscription, error) {

	return b.pubsub.Subscribe(ctx, clientID, query, capacities...)
}

func (b *EventBus) SubscribeWithArgs(ctx context.Context, args tmpubsub.SubscribeArgs) (Subscription, error) {
	return b.pubsub.SubscribeWithArgs(ctx, args)
}

func (b *EventBus) Unsubscribe(ctx context.Context, args tmpubsub.UnsubscribeArgs) error {
	return b.pubsub.Unsubscribe(ctx, args)
}

func (b *EventBus) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return b.pubsub.UnsubscribeAll(ctx, subscriber)
}

func (b *EventBus) Observe(ctx context.Context, observe func(tmpubsub.Message) error, queries ...*tmquery.Query) error {
	return b.pubsub.Observe(ctx, observe, queries...)
}

func (b *EventBus) Publish(ctx context.Context, eventValue string, eventData types.EventData) error {
	tokens := strings.Split(types.EventTypeKey, ".")
	event := abci.Event{
		Type: tokens[0],
		Attributes: []abci.EventAttribute{
			{
				Key:   tokens[1],
				Value: eventValue,
			},
		},
	}

	return b.pubsub.PublishWithEvents(ctx, eventData, []abci.Event{event})
}

func (b *EventBus) PublishEventNewBlock(ctx context.Context, data types.EventDataNewBlock) error {
	events := data.ResultFinalizeBlock.Events

	// add Tendermint-reserved new block event
	events = append(events, types.EventNewBlock)

	return b.pubsub.PublishWithEvents(ctx, data, events)
}

func (b *EventBus) PublishEventNewBlockHeader(ctx context.Context, data types.EventDataNewBlockHeader) error {
	// no explicit deadline for publishing events

	events := data.ResultFinalizeBlock.Events

	// add Tendermint-reserved new block header event
	events = append(events, types.EventNewBlockHeader)

	return b.pubsub.PublishWithEvents(ctx, data, events)
}

func (b *EventBus) PublishEventNewEvidence(ctx context.Context, evidence types.EventDataNewEvidence) error {
	return b.Publish(ctx, types.EventNewEvidenceValue, evidence)
}

func (b *EventBus) PublishEventVote(ctx context.Context, data types.EventDataVote) error {
	return b.Publish(ctx, types.EventVoteValue, data)
}

func (b *EventBus) PublishEventValidBlock(ctx context.Context, data types.EventDataRoundState) error {
	return b.Publish(ctx, types.EventValidBlockValue, data)
}

func (b *EventBus) PublishEventBlockSyncStatus(ctx context.Context, data types.EventDataBlockSyncStatus) error {
	return b.Publish(ctx, types.EventBlockSyncStatusValue, data)
}

func (b *EventBus) PublishEventStateSyncStatus(ctx context.Context, data types.EventDataStateSyncStatus) error {
	return b.Publish(ctx, types.EventStateSyncStatusValue, data)
}

// PublishEventTx publishes tx event with events from Result. Note it will add
// predefined keys (EventTypeKey, TxHashKey). Existing events with the same keys
// will be overwritten.
func (b *EventBus) PublishEventTx(ctx context.Context, data types.EventDataTx) error {
	events := data.Result.Events

	// add Tendermint-reserved events
	events = append(events, types.EventTx)

	tokens := strings.Split(types.TxHashKey, ".")
	events = append(events, abci.Event{
		Type: tokens[0],
		Attributes: []abci.EventAttribute{
			{
				Key:   tokens[1],
				Value: fmt.Sprintf("%X", types.Tx(data.Tx).Hash()),
			},
		},
	})

	tokens = strings.Split(types.TxHeightKey, ".")
	events = append(events, abci.Event{
		Type: tokens[0],
		Attributes: []abci.EventAttribute{
			{
				Key:   tokens[1],
				Value: fmt.Sprintf("%d", data.Height),
			},
		},
	})

	return b.pubsub.PublishWithEvents(ctx, data, events)
}

func (b *EventBus) PublishEventNewRoundStep(ctx context.Context, data types.EventDataRoundState) error {
	return b.Publish(ctx, types.EventNewRoundStepValue, data)
}

func (b *EventBus) PublishEventTimeoutPropose(ctx context.Context, data types.EventDataRoundState) error {
	return b.Publish(ctx, types.EventTimeoutProposeValue, data)
}

func (b *EventBus) PublishEventTimeoutWait(ctx context.Context, data types.EventDataRoundState) error {
	return b.Publish(ctx, types.EventTimeoutWaitValue, data)
}

func (b *EventBus) PublishEventNewRound(ctx context.Context, data types.EventDataNewRound) error {
	return b.Publish(ctx, types.EventNewRoundValue, data)
}

func (b *EventBus) PublishEventCompleteProposal(ctx context.Context, data types.EventDataCompleteProposal) error {
	return b.Publish(ctx, types.EventCompleteProposalValue, data)
}

func (b *EventBus) PublishEventPolka(ctx context.Context, data types.EventDataRoundState) error {
	return b.Publish(ctx, types.EventPolkaValue, data)
}

func (b *EventBus) PublishEventRelock(ctx context.Context, data types.EventDataRoundState) error {
	return b.Publish(ctx, types.EventRelockValue, data)
}

func (b *EventBus) PublishEventLock(ctx context.Context, data types.EventDataRoundState) error {
	return b.Publish(ctx, types.EventLockValue, data)
}

func (b *EventBus) PublishEventValidatorSetUpdates(ctx context.Context, data types.EventDataValidatorSetUpdates) error {
	return b.Publish(ctx, types.EventValidatorSetUpdatesValue, data)
}

func (b *EventBus) PublishEventEvidenceValidated(ctx context.Context, evidence types.EventDataEvidenceValidated) error {
	return b.Publish(ctx, types.EventEvidenceValidatedValue, evidence)
}

//-----------------------------------------------------------------------------

// NopEventBus implements a types.BlockEventPublisher that discards all events.
type NopEventBus struct{}

func (NopEventBus) PublishEventNewBlock(context.Context, types.EventDataNewBlock) error {
	return nil
}

func (NopEventBus) PublishEventNewBlockHeader(context.Context, types.EventDataNewBlockHeader) error {
	return nil
}

func (NopEventBus) PublishEventNewEvidence(context.Context, types.EventDataNewEvidence) error {
	return nil
}

func (NopEventBus) PublishEventTx(context.Context, types.EventDataTx) error {
	return nil
}

func (NopEventBus) PublishEventValidatorSetUpdates(context.Context, types.EventDataValidatorSetUpdates) error {
	return nil
}
