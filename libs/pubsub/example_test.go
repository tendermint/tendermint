package pubsub_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/libs/pubsub/query"
)

func TestExample(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	ch := make(chan interface{}, 1)
	err := s.Subscribe(ctx, "example-client", query.MustParse("abci.account.name='John'"), ch)
	require.NoError(t, err)
	err = s.PublishWithTags(ctx, "Tombstone", pubsub.NewTagMap(map[string]string{"abci.account.name": "John"}))
	require.NoError(t, err)
	assertReceive(t, "Tombstone", ch)
}
