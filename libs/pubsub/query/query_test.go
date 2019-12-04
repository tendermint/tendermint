package query_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/pubsub/query"
)

func TestMatches(t *testing.T) {
	var (
		txDate = "2017-01-01"
		txTime = "2018-05-03T14:45:00Z"
	)

	testCases := []struct {
		s        string
		events   map[string][]string
		err      bool
		matches  bool
		matchErr bool
	}{
		{"tm.events.type='NewBlock'", map[string][]string{"tm.events.type": {"NewBlock"}}, false, true, false},
		{"tx.gas > 7", map[string][]string{"tx.gas": {"8"}}, false, true, false},
		{"transfer.amount > 7", map[string][]string{"transfer.amount": {"8stake"}}, false, true, false},
		{"transfer.amount > 7", map[string][]string{"transfer.amount": {"8.045stake"}}, false, true, false},
		{"transfer.amount > 7.043", map[string][]string{"transfer.amount": {"8.045stake"}}, false, true, false},
		{"transfer.amount > 8.045", map[string][]string{"transfer.amount": {"8.045stake"}}, false, false, false},
		{"tx.gas > 7 AND tx.gas < 9", map[string][]string{"tx.gas": {"8"}}, false, true, false},
		{"body.weight >= 3.5", map[string][]string{"body.weight": {"3.5"}}, false, true, false},
		{"account.balance < 1000.0", map[string][]string{"account.balance": {"900"}}, false, true, false},
		{"apples.kg <= 4", map[string][]string{"apples.kg": {"4.0"}}, false, true, false},
		{"body.weight >= 4.5", map[string][]string{"body.weight": {fmt.Sprintf("%v", float32(4.5))}}, false, true, false},
		{
			"oranges.kg < 4 AND watermellons.kg > 10",
			map[string][]string{"oranges.kg": {"3"}, "watermellons.kg": {"12"}},
			false,
			true,
			false,
		},
		{"peaches.kg < 4", map[string][]string{"peaches.kg": {"5"}}, false, false, false},
		{
			"tx.date > DATE 2017-01-01",
			map[string][]string{"tx.date": {time.Now().Format(query.DateLayout)}},
			false,
			true,
			false,
		},
		{"tx.date = DATE 2017-01-01", map[string][]string{"tx.date": {txDate}}, false, true, false},
		{"tx.date = DATE 2018-01-01", map[string][]string{"tx.date": {txDate}}, false, false, false},
		{
			"tx.time >= TIME 2013-05-03T14:45:00Z",
			map[string][]string{"tx.time": {time.Now().Format(query.TimeLayout)}},
			false,
			true,
			false,
		},
		{"tx.time = TIME 2013-05-03T14:45:00Z", map[string][]string{"tx.time": {txTime}}, false, false, false},
		{"abci.owner.name CONTAINS 'Igor'", map[string][]string{"abci.owner.name": {"Igor,Ivan"}}, false, true, false},
		{"abci.owner.name CONTAINS 'Igor'", map[string][]string{"abci.owner.name": {"Pavel,Ivan"}}, false, false, false},
		{"abci.owner.name = 'Igor'", map[string][]string{"abci.owner.name": {"Igor", "Ivan"}}, false, true, false},
		{
			"abci.owner.name = 'Ivan'",
			map[string][]string{"abci.owner.name": {"Igor", "Ivan"}},
			false,
			true,
			false,
		},
		{
			"abci.owner.name = 'Ivan' AND abci.owner.name = 'Igor'",
			map[string][]string{"abci.owner.name": {"Igor", "Ivan"}},
			false,
			true,
			false,
		},
		{
			"abci.owner.name = 'Ivan' AND abci.owner.name = 'John'",
			map[string][]string{"abci.owner.name": {"Igor", "Ivan"}},
			false,
			false,
			false,
		},
		{
			"tm.events.type='NewBlock'",
			map[string][]string{"tm.events.type": {"NewBlock"}, "app.name": {"fuzzed"}},
			false,
			true,
			false,
		},
		{
			"app.name = 'fuzzed'",
			map[string][]string{"tm.events.type": {"NewBlock"}, "app.name": {"fuzzed"}},
			false,
			true,
			false,
		},
		{
			"tm.events.type='NewBlock' AND app.name = 'fuzzed'",
			map[string][]string{"tm.events.type": {"NewBlock"}, "app.name": {"fuzzed"}},
			false,
			true,
			false,
		},
		{
			"tm.events.type='NewHeader' AND app.name = 'fuzzed'",
			map[string][]string{"tm.events.type": {"NewBlock"}, "app.name": {"fuzzed"}},
			false,
			false,
			false,
		},
		{"slash EXISTS",
			map[string][]string{"slash.reason": {"missing_signature"}, "slash.power": {"6000"}},
			false,
			true,
			false,
		},
		{"sl EXISTS",
			map[string][]string{"slash.reason": {"missing_signature"}, "slash.power": {"6000"}},
			false,
			true,
			false,
		},
		{"slash EXISTS",
			map[string][]string{"transfer.recipient": {"cosmos1gu6y2a0ffteesyeyeesk23082c6998xyzmt9mz"},
				"transfer.sender": {"cosmos1crje20aj4gxdtyct7z3knxqry2jqt2fuaey6u5"}},
			false,
			false,
			false,
		},
		{"slash.reason EXISTS AND slash.power > 1000",
			map[string][]string{"slash.reason": {"missing_signature"}, "slash.power": {"6000"}},
			false,
			true,
			false,
		},
		{"slash.reason EXISTS AND slash.power > 1000",
			map[string][]string{"slash.reason": {"missing_signature"}, "slash.power": {"500"}},
			false,
			false,
			false,
		},
		{"slash.reason EXISTS",
			map[string][]string{"transfer.recipient": {"cosmos1gu6y2a0ffteesyeyeesk23082c6998xyzmt9mz"},
				"transfer.sender": {"cosmos1crje20aj4gxdtyct7z3knxqry2jqt2fuaey6u5"}},
			false,
			false,
			false,
		},
	}

	for _, tc := range testCases {
		q, err := query.New(tc.s)
		if !tc.err {
			require.Nil(t, err)
		}
		require.NotNil(t, q, "Query '%s' should not be nil", tc.s)

		if tc.matches {
			match, err := q.Matches(tc.events)
			assert.Nil(t, err, "Query '%s' should not error on match %v", tc.s, tc.events)
			assert.True(t, match, "Query '%s' should match %v", tc.s, tc.events)
		} else {
			match, err := q.Matches(tc.events)
			assert.Equal(t, tc.matchErr, err != nil, "Unexpected error for query '%s' match %v", tc.s, tc.events)
			assert.False(t, match, "Query '%s' should not match %v", tc.s, tc.events)
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
		{
			s: "tm.events.type='NewBlock'",
			conditions: []query.Condition{
				{CompositeKey: "tm.events.type", Op: query.OpEqual, Operand: "NewBlock"},
			},
		},
		{
			s: "tx.gas > 7 AND tx.gas < 9",
			conditions: []query.Condition{
				{CompositeKey: "tx.gas", Op: query.OpGreater, Operand: int64(7)},
				{CompositeKey: "tx.gas", Op: query.OpLess, Operand: int64(9)},
			},
		},
		{
			s: "tx.time >= TIME 2013-05-03T14:45:00Z",
			conditions: []query.Condition{
				{CompositeKey: "tx.time", Op: query.OpGreaterEqual, Operand: txTime},
			},
		},
		{
			s: "slashing EXISTS",
			conditions: []query.Condition{
				{CompositeKey: "slashing", Op: query.OpExists},
			},
		},
	}

	for _, tc := range testCases {
		q, err := query.New(tc.s)
		require.Nil(t, err)

		c, err := q.Conditions()
		require.NoError(t, err)
		assert.Equal(t, tc.conditions, c)
	}
}
