package query_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tmlibs/pubsub/query"
)

func TestMatches(t *testing.T) {
	const shortForm = "2006-Jan-02"
	txDate, err := time.Parse(shortForm, "2017-Jan-01")
	require.NoError(t, err)
	txTime, err := time.Parse(time.RFC3339, "2018-05-03T14:45:00Z")
	require.NoError(t, err)

	testCases := []struct {
		s       string
		tags    map[string]interface{}
		err     bool
		matches bool
	}{
		{"tm.events.type='NewBlock'", map[string]interface{}{"tm.events.type": "NewBlock"}, false, true},

		{"tx.gas > 7", map[string]interface{}{"tx.gas": 8}, false, true},
		{"tx.gas > 7 AND tx.gas < 9", map[string]interface{}{"tx.gas": 8}, false, true},
		{"body.weight >= 3.5", map[string]interface{}{"body.weight": 3.5}, false, true},
		{"account.balance < 1000.0", map[string]interface{}{"account.balance": 900}, false, true},
		{"apples.kg <= 4", map[string]interface{}{"apples.kg": 4.0}, false, true},
		{"body.weight >= 4.5", map[string]interface{}{"body.weight": float32(4.5)}, false, true},
		{"oranges.kg < 4 AND watermellons.kg > 10", map[string]interface{}{"oranges.kg": 3, "watermellons.kg": 12}, false, true},
		{"peaches.kg < 4", map[string]interface{}{"peaches.kg": 5}, false, false},

		{"tx.date > DATE 2017-01-01", map[string]interface{}{"tx.date": time.Now()}, false, true},
		{"tx.date = DATE 2017-01-01", map[string]interface{}{"tx.date": txDate}, false, true},
		{"tx.date = DATE 2018-01-01", map[string]interface{}{"tx.date": txDate}, false, false},

		{"tx.time >= TIME 2013-05-03T14:45:00Z", map[string]interface{}{"tx.time": time.Now()}, false, true},
		{"tx.time = TIME 2013-05-03T14:45:00Z", map[string]interface{}{"tx.time": txTime}, false, false},

		{"abci.owner.name CONTAINS 'Igor'", map[string]interface{}{"abci.owner.name": "Igor,Ivan"}, false, true},
		{"abci.owner.name CONTAINS 'Igor'", map[string]interface{}{"abci.owner.name": "Pavel,Ivan"}, false, false},
	}

	for _, tc := range testCases {
		query, err := query.New(tc.s)
		if !tc.err {
			require.Nil(t, err)
		}

		if tc.matches {
			assert.True(t, query.Matches(tc.tags), "Query '%s' should match %v", tc.s, tc.tags)
		} else {
			assert.False(t, query.Matches(tc.tags), "Query '%s' should not match %v", tc.s, tc.tags)
		}
	}
}

func TestMustParse(t *testing.T) {
	assert.Panics(t, func() { query.MustParse("=") })
	assert.NotPanics(t, func() { query.MustParse("tm.events.type='NewBlock'") })
}
