package pubsub_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/libs/pubsub/query"
)

func TestExample(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())

	require.NoError(t, s.Start())

	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	ctx := context.Background()

	subscription, err := s.Subscribe(ctx, "example-client", query.MustParse("abci.account.name='John'"))
	require.NoError(t, err)

	events := []abci.Event{
		{
			Type:       "abci.account",
			Attributes: []abci.EventAttribute{{Key: "name", Value: "John"}},
		},
	}
	err = s.PublishWithEvents(ctx, "Tombstone", events)
	require.NoError(t, err)

	assertReceive(t, "Tombstone", subscription.Out())
}
