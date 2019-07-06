package query_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/libs/pubsub/query"
)

func TestEmptyQueryMatchesAnything(t *testing.T) {
	q := query.Empty{}
	assert.True(t, q.Matches(map[string][]string{}))
	assert.True(t, q.Matches(map[string][]string{"Asher": {"Roth"}}))
	assert.True(t, q.Matches(map[string][]string{"Route": {"66"}}))
	assert.True(t, q.Matches(map[string][]string{"Route": {"66"}, "Billy": {"Blue"}}))
}
