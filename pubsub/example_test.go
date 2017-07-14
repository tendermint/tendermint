package pubsub_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tmlibs/log"
	"github.com/tendermint/tmlibs/pubsub"
	"github.com/tendermint/tmlibs/pubsub/query"
)

func TestExample(t *testing.T) {
	s := pubsub.NewServer()
	s.SetLogger(log.TestingLogger())
	s.Start()
	defer s.Stop()

	ch := make(chan interface{}, 1)
	err := s.Subscribe("example-client", query.MustParse("abci.account.name=John"), ch)
	require.NoError(t, err)
	s.PublishWithTags("Tombstone", map[string]interface{}{"abci.account.name": "John"})
	assertReceive(t, "Tombstone", ch)
}
