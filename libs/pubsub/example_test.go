package pubsub_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/libs/pubsub/query"
)

func TestExample(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	sub := newTestSub(t).must(s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: "example-client",
		Query:    query.MustParse("abci.account.name='John'"),
	}))

	events := []abci.Event{
		{
			Type:       "abci.account",
			Attributes: []abci.EventAttribute{{Key: "name", Value: "John"}},
		},
	}
	require.NoError(t, s.PublishWithEvents(ctx, "Tombstone", events))
	sub.mustReceive(ctx, "Tombstone")
}
