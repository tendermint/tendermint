package eventbus

import (
	"context"
	"fmt"
	"strings"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
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
	pubsub := tmpubsub.NewServer(tmpubsub.BufferCapacity(0),
		func(s *tmpubsub.Server) {
			s.Logger = logger
		})
	b := &EventBus{pubsub: pubsub}
	b.BaseService = *service.NewBaseService(logger, "EventBus", b)
	return b
}

func (b *EventBus) OnStart() error {
	return b.pubsub.Start()
}

func (b *EventBus) OnStop() {
	if err := b.pubsub.Stop(); err != nil {
		b.pubsub.Logger.Error("error trying to stop eventBus", "error", err)
	}
}

func (b *EventBus) NumClients() int {
	return b.pubsub.NumClients()
}

func (b *EventBus) NumClientSubscriptions(clientID string) int {
	return b.pubsub.NumClientSubscriptions(clientID)
}

// Deprecated: Use SubscribeWithArgs instead.
func (b *EventBus) Subscribe(ctx context.Context,
	clientID string, query tmpubsub.Query, capacities ...int) (Subscription, error) {

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

func (b *EventBus) Observe(ctx context.Context, observe func(tmpubsub.Message) error, queries ...tmpubsub.Query) error {
	return b.pubsub.Observe(ctx, observe, queries...)
}

func (b *EventBus) Publish(eventValue string, eventData types.TMEventData) error {
	// no explicit deadline for publishing events
	ctx := context.Background()

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

func (b *EventBus) PublishEventNewBlock(data types.EventDataNewBlock) error {
	// no explicit deadline for publishing events
	ctx := context.Background()
	events := append(data.ResultBeginBlock.Events, data.ResultEndBlock.Events...)

	// add Tendermint-reserved new block event
	events = append(events, types.EventNewBlock)

	return b.pubsub.PublishWithEvents(ctx, data, events)
}

func (b *EventBus) PublishEventNewBlockHeader(data types.EventDataNewBlockHeader) error {
	// no explicit deadline for publishing events
	ctx := context.Background()
	events := append(data.ResultBeginBlock.Events, data.ResultEndBlock.Events...)

	// add Tendermint-reserved new block header event
	events = append(events, types.EventNewBlockHeader)

	return b.pubsub.PublishWithEvents(ctx, data, events)
}

func (b *EventBus) PublishEventNewEvidence(evidence types.EventDataNewEvidence) error {
	return b.Publish(types.EventNewEvidenceValue, evidence)
}

func (b *EventBus) PublishEventVote(data types.EventDataVote) error {
	return b.Publish(types.EventVoteValue, data)
}

func (b *EventBus) PublishEventValidBlock(data types.EventDataRoundState) error {
	return b.Publish(types.EventValidBlockValue, data)
}

func (b *EventBus) PublishEventBlockSyncStatus(data types.EventDataBlockSyncStatus) error {
	return b.Publish(types.EventBlockSyncStatusValue, data)
}

func (b *EventBus) PublishEventStateSyncStatus(data types.EventDataStateSyncStatus) error {
	return b.Publish(types.EventStateSyncStatusValue, data)
}

// PublishEventTx publishes tx event with events from Result. Note it will add
// predefined keys (EventTypeKey, TxHashKey). Existing events with the same keys
// will be overwritten.
func (b *EventBus) PublishEventTx(data types.EventDataTx) error {
	// no explicit deadline for publishing events
	ctx := context.Background()
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

func (b *EventBus) PublishEventNewRoundStep(data types.EventDataRoundState) error {
	return b.Publish(types.EventNewRoundStepValue, data)
}

func (b *EventBus) PublishEventTimeoutPropose(data types.EventDataRoundState) error {
	return b.Publish(types.EventTimeoutProposeValue, data)
}

func (b *EventBus) PublishEventTimeoutWait(data types.EventDataRoundState) error {
	return b.Publish(types.EventTimeoutWaitValue, data)
}

func (b *EventBus) PublishEventNewRound(data types.EventDataNewRound) error {
	return b.Publish(types.EventNewRoundValue, data)
}

func (b *EventBus) PublishEventCompleteProposal(data types.EventDataCompleteProposal) error {
	return b.Publish(types.EventCompleteProposalValue, data)
}

func (b *EventBus) PublishEventPolka(data types.EventDataRoundState) error {
	return b.Publish(types.EventPolkaValue, data)
}

func (b *EventBus) PublishEventUnlock(data types.EventDataRoundState) error {
	return b.Publish(types.EventUnlockValue, data)
}

func (b *EventBus) PublishEventRelock(data types.EventDataRoundState) error {
	return b.Publish(types.EventRelockValue, data)
}

func (b *EventBus) PublishEventLock(data types.EventDataRoundState) error {
	return b.Publish(types.EventLockValue, data)
}

func (b *EventBus) PublishEventValidatorSetUpdates(data types.EventDataValidatorSetUpdates) error {
	return b.Publish(types.EventValidatorSetUpdatesValue, data)
}

//-----------------------------------------------------------------------------

// NopEventBus implements a types.BlockEventPublisher that discards all events.
type NopEventBus struct{}

func (NopEventBus) PublishEventNewBlock(types.EventDataNewBlock) error {
	return nil
}

func (NopEventBus) PublishEventNewBlockHeader(types.EventDataNewBlockHeader) error {
	return nil
}

func (NopEventBus) PublishEventNewEvidence(types.EventDataNewEvidence) error {
	return nil
}

func (NopEventBus) PublishEventTx(types.EventDataTx) error {
	return nil
}

func (NopEventBus) PublishEventValidatorSetUpdates(types.EventDataValidatorSetUpdates) error {
	return nil
}
