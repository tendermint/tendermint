package eventbus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/internal/pubsub"
)

func TestEventBusNilSubscription(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventBus := new(EventBus)
	eventBus.pubsub = new(pubsub.Server)

	txsSub, err := eventBus.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{})
	assert.Error(t, err)
	assert.True(t, txsSub == nil)

	// This will panic
	if txsSub != nil {
		txsSub.ID()
	}
}
