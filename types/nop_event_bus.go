package types

import (
	"context"

	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
)

type NopEventBus struct{}

func (NopEventBus) Subscribe(ctx context.Context, subscriber string, query tmpubsub.Query, out chan<- interface{}) error {
	return nil
}

func (NopEventBus) Unsubscribe(ctx context.Context, subscriber string, query tmpubsub.Query) error {
	return nil
}

func (NopEventBus) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return nil
}

func (NopEventBus) PublishEventNewBlock(data EventDataNewBlock) error {
	return nil
}

func (NopEventBus) PublishEventNewBlockHeader(data EventDataNewBlockHeader) error {
	return nil
}

func (NopEventBus) PublishEventVote(data EventDataVote) error {
	return nil
}

func (NopEventBus) PublishEventTx(data EventDataTx) error {
	return nil
}

func (NopEventBus) PublishEventNewRoundStep(data EventDataRoundState) error {
	return nil
}

func (NopEventBus) PublishEventTimeoutPropose(data EventDataRoundState) error {
	return nil
}

func (NopEventBus) PublishEventTimeoutWait(data EventDataRoundState) error {
	return nil
}

func (NopEventBus) PublishEventNewRound(data EventDataRoundState) error {
	return nil
}

func (NopEventBus) PublishEventCompleteProposal(data EventDataRoundState) error {
	return nil
}

func (NopEventBus) PublishEventPolka(data EventDataRoundState) error {
	return nil
}

func (NopEventBus) PublishEventUnlock(data EventDataRoundState) error {
	return nil
}

func (NopEventBus) PublishEventRelock(data EventDataRoundState) error {
	return nil
}

func (NopEventBus) PublishEventLock(data EventDataRoundState) error {
	return nil
}

func (NopEventBus) PublishEventValidatorSetUpdates(data EventDataValidatorSetUpdates) error {
	return nil
}
