package query_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/libs/pubsub/query"
)

func TestMatches(t *testing.T) {
	var (
		txDate = "2017-01-01"
		txTime = "2018-05-03T14:45:00Z"
	)

	testCases := []struct {
		s       string
		tags    map[string]string
		err     bool
		matches bool
	}{
		{"tm.events.type='NewBlock'", map[string]string{"tm.events.type": "NewBlock"}, false, true},

		{"tx.gas > 7", map[string]string{"tx.gas": "8"}, false, true},
		{"tx.gas > 7 AND tx.gas < 9", map[string]string{"tx.gas": "8"}, false, true},
		{"body.weight >= 3.5", map[string]string{"body.weight": "3.5"}, false, true},
		{"account.balance < 1000.0", map[string]string{"account.balance": "900"}, false, true},
		{"apples.kg <= 4", map[string]string{"apples.kg": "4.0"}, false, true},
		{"body.weight >= 4.5", map[string]string{"body.weight": fmt.Sprintf("%v", float32(4.5))}, false, true},
		{"oranges.kg < 4 AND watermellons.kg > 10", map[string]string{"oranges.kg": "3", "watermellons.kg": "12"}, false, true},
		{"peaches.kg < 4", map[string]string{"peaches.kg": "5"}, false, false},

		{"tx.date > DATE 2017-01-01", map[string]string{"tx.date": time.Now().Format(query.DateLayout)}, false, true},
		{"tx.date = DATE 2017-01-01", map[string]string{"tx.date": txDate}, false, true},
		{"tx.date = DATE 2018-01-01", map[string]string{"tx.date": txDate}, false, false},

		{"tx.time >= TIME 2013-05-03T14:45:00Z", map[string]string{"tx.time": time.Now().Format(query.TimeLayout)}, false, true},
		{"tx.time = TIME 2013-05-03T14:45:00Z", map[string]string{"tx.time": txTime}, false, false},

		{"abci.owner.name CONTAINS 'Igor'", map[string]string{"abci.owner.name": "Igor,Ivan"}, false, true},
		{"abci.owner.name CONTAINS 'Igor'", map[string]string{"abci.owner.name": "Pavel,Ivan"}, false, false},
	}

	for _, tc := range testCases {
		q, err := query.New(tc.s)
		if !tc.err {
			require.Nil(t, err)
		}

		if tc.matches {
			assert.True(t, q.Matches(pubsub.NewTagMap(tc.tags)), "Query '%s' should match %v", tc.s, tc.tags)
		} else {
			assert.False(t, q.Matches(pubsub.NewTagMap(tc.tags)), "Query '%s' should not match %v", tc.s, tc.tags)
		}
	}
}

func TestMustParse(t *testing.T) {
	assert.Panics(t, func() { query.MustParse("=") })
	assert.NotPanics(t, func() { query.MustParse("tm.events.type='NewBlock'") })
}

func TestConditions(t *testing.T) {
	txTime, err := time.Parse(time.RFC3339, "2013-05-03T14:45:00Z")
	require.NoError(t, err)

	testCases := []struct {
		s          string
		conditions []query.Condition
	}{
		{s: "tm.events.type='NewBlock'", conditions: []query.Condition{{Tag: "tm.events.type", Op: query.OpEqual, Operand: "NewBlock"}}},
		{s: "tx.gas > 7 AND tx.gas < 9", conditions: []query.Condition{{Tag: "tx.gas", Op: query.OpGreater, Operand: int64(7)}, {Tag: "tx.gas", Op: query.OpLess, Operand: int64(9)}}},
		{s: "tx.time >= TIME 2013-05-03T14:45:00Z", conditions: []query.Condition{{Tag: "tx.time", Op: query.OpGreaterEqual, Operand: txTime}}},
	}

	for _, tc := range testCases {
		q, err := query.New(tc.s)
		require.Nil(t, err)

		assert.Equal(t, tc.conditions, q.Conditions())
	}
}
