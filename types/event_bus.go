package types

import (
	"context"
	"fmt"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
)

const defaultCapacity = 0

type EventBusSubscriber interface {
	Subscribe(ctx context.Context, subscriber string, query tmpubsub.Query, out chan<- interface{}) error
	Unsubscribe(ctx context.Context, subscriber string, query tmpubsub.Query) error
	UnsubscribeAll(ctx context.Context, subscriber string) error
}

// EventBus is a common bus for all events going through the system. All calls
// are proxied to underlying pubsub server. All events must be published using
// EventBus to ensure correct data types.
type EventBus struct {
	cmn.BaseService
	pubsub *tmpubsub.Server
}

// NewEventBus returns a new event bus.
func NewEventBus() *EventBus {
	return NewEventBusWithBufferCapacity(defaultCapacity)
}

// NewEventBusWithBufferCapacity returns a new event bus with the given buffer capacity.
func NewEventBusWithBufferCapacity(cap int) *EventBus {
	// capacity could be exposed later if needed
	pubsub := tmpubsub.NewServer(tmpubsub.BufferCapacity(cap))
	b := &EventBus{pubsub: pubsub}
	b.BaseService = *cmn.NewBaseService(nil, "EventBus", b)
	return b
}

func (b *EventBus) SetLogger(l log.Logger) {
	b.BaseService.SetLogger(l)
	b.pubsub.SetLogger(l.With("module", "pubsub"))
}

func (b *EventBus) OnStart() error {
	return b.pubsub.OnStart()
}

func (b *EventBus) OnStop() {
	b.pubsub.Stop()
}

func (b *EventBus) Subscribe(ctx context.Context, subscriber string, query tmpubsub.Query, out chan<- interface{}) error {
	return b.pubsub.Subscribe(ctx, subscriber, query, out)
}

func (b *EventBus) Unsubscribe(ctx context.Context, subscriber string, query tmpubsub.Query) error {
	return b.pubsub.Unsubscribe(ctx, subscriber, query)
}

func (b *EventBus) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return b.pubsub.UnsubscribeAll(ctx, subscriber)
}

func (b *EventBus) Publish(eventType string, eventData TMEventData) error {
	// no explicit deadline for publishing events
	ctx := context.Background()
	b.pubsub.PublishWithTags(ctx, eventData, tmpubsub.NewTagMap(map[string]string{EventTypeKey: eventType}))
	return nil
}

func (b *EventBus) PublishEventNewBlock(data EventDataNewBlock) error {
	return b.Publish(EventNewBlock, data)
}

func (b *EventBus) PublishEventNewBlockHeader(data EventDataNewBlockHeader) error {
	return b.Publish(EventNewBlockHeader, data)
}

func (b *EventBus) PublishEventVote(data EventDataVote) error {
	return b.Publish(EventVote, data)
}

// PublishEventTx publishes tx event with tags from Result. Note it will add
// predefined tags (EventTypeKey, TxHashKey). Existing tags with the same names
// will be overwritten.
func (b *EventBus) PublishEventTx(data EventDataTx) error {
	// no explicit deadline for publishing events
	ctx := context.Background()

	tags := make(map[string]string)

	// validate and fill tags from tx result
	for _, tag := range data.Result.Tags {
		// basic validation
		if len(tag.Key) == 0 {
			b.Logger.Info("Got tag with an empty key (skipping)", "tag", tag, "tx", data.Tx)
			continue
		}
		tags[string(tag.Key)] = string(tag.Value)
	}

	// add predefined tags
	logIfTagExists(EventTypeKey, tags, b.Logger)
	tags[EventTypeKey] = EventTx

	logIfTagExists(TxHashKey, tags, b.Logger)
	tags[TxHashKey] = fmt.Sprintf("%X", data.Tx.Hash())

	logIfTagExists(TxHeightKey, tags, b.Logger)
	tags[TxHeightKey] = fmt.Sprintf("%d", data.Height)

	b.pubsub.PublishWithTags(ctx, data, tmpubsub.NewTagMap(tags))
	return nil
}

func (b *EventBus) PublishEventProposalHeartbeat(data EventDataProposalHeartbeat) error {
	return b.Publish(EventProposalHeartbeat, data)
}

func (b *EventBus) PublishEventNewRoundStep(data EventDataRoundState) error {
	return b.Publish(EventNewRoundStep, data)
}

func (b *EventBus) PublishEventTimeoutPropose(data EventDataRoundState) error {
	return b.Publish(EventTimeoutPropose, data)
}

func (b *EventBus) PublishEventTimeoutWait(data EventDataRoundState) error {
	return b.Publish(EventTimeoutWait, data)
}

func (b *EventBus) PublishEventNewRound(data EventDataRoundState) error {
	return b.Publish(EventNewRound, data)
}

func (b *EventBus) PublishEventCompleteProposal(data EventDataRoundState) error {
	return b.Publish(EventCompleteProposal, data)
}

func (b *EventBus) PublishEventPolka(data EventDataRoundState) error {
	return b.Publish(EventPolka, data)
}

func (b *EventBus) PublishEventUnlock(data EventDataRoundState) error {
	return b.Publish(EventUnlock, data)
}

func (b *EventBus) PublishEventRelock(data EventDataRoundState) error {
	return b.Publish(EventRelock, data)
}

func (b *EventBus) PublishEventLock(data EventDataRoundState) error {
	return b.Publish(EventLock, data)
}

func (b *EventBus) PublishEventValidatorSetUpdates(data EventDataValidatorSetUpdates) error {
	return b.Publish(EventValidatorSetUpdates, data)
}

func logIfTagExists(tag string, tags map[string]string, logger log.Logger) {
	if value, ok := tags[tag]; ok {
		logger.Error("Found predefined tag (value will be overwritten)", "tag", tag, "value", value)
	}
}
