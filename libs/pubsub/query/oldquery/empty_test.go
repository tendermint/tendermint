package query_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	query "github.com/tendermint/tendermint/libs/pubsub/query/oldquery"
)

func TestEmptyQueryMatchesAnything(t *testing.T) {
	q := query.Empty{}

	testCases := []struct {
		events []abci.Event
	}{
		{
			[]abci.Event{},
		},
		{
			[]abci.Event{
				{
					Type:       "Asher",
					Attributes: []abci.EventAttribute{{Key: "Roth"}},
				},
			},
		},
		{
			[]abci.Event{
				{
					Type:       "Route",
					Attributes: []abci.EventAttribute{{Key: "66"}},
				},
			},
		},
		{
			[]abci.Event{
				{
					Type:       "Route",
					Attributes: []abci.EventAttribute{{Key: "66"}},
				},
				{
					Type:       "Billy",
					Attributes: []abci.EventAttribute{{Key: "Blue"}},
				},
			},
		},
	}

	for _, tc := range testCases {
		match, err := q.Matches(tc.events)
		require.Nil(t, err)
		require.True(t, match)
	}
}
