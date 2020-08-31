package query_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/libs/pubsub/query"
)

func TestEmptyQueryMatchesAnything(t *testing.T) {
	q := query.Empty{}

	testCases := []struct {
		query map[string][]string
	}{
		{map[string][]string{}},
		{map[string][]string{"Asher": {"Roth"}}},
		{map[string][]string{"Route": {"66"}}},
		{map[string][]string{"Route": {"66"}, "Billy": {"Blue"}}},
	}

	for _, tc := range testCases {
		match, err := q.Matches(tc.query)
		assert.Nil(t, err)
		assert.True(t, match)
	}
}
