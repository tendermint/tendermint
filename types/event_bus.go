package types

import (
	"context"
	"fmt"

	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
	tmpubsub "github.com/tendermint/tmlibs/pubsub"
)

const defaultCapacity = 1000

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
	b.pubsub.OnStop()
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
	if b.pubsub != nil {
		// no explicit deadline for publishing events
		ctx := context.Background()
		b.pubsub.PublishWithTags(ctx, eventData, map[string]interface{}{EventTypeKey: eventType})
	}
	return nil
}

//--- block, tx, and vote events

func (b *EventBus) PublishEventNewBlock(block EventDataNewBlock) error {
	return b.Publish(EventNewBlock, TMEventData{block})
}

func (b *EventBus) PublishEventNewBlockHeader(header EventDataNewBlockHeader) error {
	return b.Publish(EventNewBlockHeader, TMEventData{header})
}

func (b *EventBus) PublishEventVote(vote EventDataVote) error {
	return b.Publish(EventVote, TMEventData{vote})
}

func (b *EventBus) PublishEventTx(tx EventDataTx) error {
	if b.pubsub != nil {
		// no explicit deadline for publishing events
		ctx := context.Background()
		b.pubsub.PublishWithTags(ctx, TMEventData{tx}, map[string]interface{}{EventTypeKey: EventTx, TxHashKey: fmt.Sprintf("%X", tx.Tx.Hash())})
	}
	return nil
}

func (b *EventBus) PublishEventProposalHeartbeat(ph EventDataProposalHeartbeat) error {
	return b.Publish(EventProposalHeartbeat, TMEventData{ph})
}

//--- EventDataRoundState events

func (b *EventBus) PublishEventNewRoundStep(rs EventDataRoundState) error {
	return b.Publish(EventNewRoundStep, TMEventData{rs})
}

func (b *EventBus) PublishEventTimeoutPropose(rs EventDataRoundState) error {
	return b.Publish(EventTimeoutPropose, TMEventData{rs})
}

func (b *EventBus) PublishEventTimeoutWait(rs EventDataRoundState) error {
	return b.Publish(EventTimeoutWait, TMEventData{rs})
}

func (b *EventBus) PublishEventNewRound(rs EventDataRoundState) error {
	return b.Publish(EventNewRound, TMEventData{rs})
}

func (b *EventBus) PublishEventCompleteProposal(rs EventDataRoundState) error {
	return b.Publish(EventCompleteProposal, TMEventData{rs})
}

func (b *EventBus) PublishEventPolka(rs EventDataRoundState) error {
	return b.Publish(EventPolka, TMEventData{rs})
}

func (b *EventBus) PublishEventUnlock(rs EventDataRoundState) error {
	return b.Publish(EventUnlock, TMEventData{rs})
}

func (b *EventBus) PublishEventRelock(rs EventDataRoundState) error {
	return b.Publish(EventRelock, TMEventData{rs})
}

func (b *EventBus) PublishEventLock(rs EventDataRoundState) error {
	return b.Publish(EventLock, TMEventData{rs})
}
