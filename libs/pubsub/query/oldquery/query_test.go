package query_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	query "github.com/tendermint/tendermint/libs/pubsub/query/oldquery"
)

func expandEvents(flattenedEvents map[string][]string) []abci.Event {
	events := make([]abci.Event, len(flattenedEvents))

	for composite, values := range flattenedEvents {
		tokens := strings.Split(composite, ".")

		attrs := make([]abci.EventAttribute, len(values))
		for i, v := range values {
			attrs[i] = abci.EventAttribute{
				Key:   tokens[len(tokens)-1],
				Value: v,
			}
		}

		events = append(events, abci.Event{
			Type:       strings.Join(tokens[:len(tokens)-1], "."),
			Attributes: attrs,
		})
	}

	return events
}

func TestMatches(t *testing.T) {
	var (
		txDate = "2017-01-01"
		txTime = "2018-05-03T14:45:00Z"
	)

	testCases := []struct {
		s       string
		events  map[string][]string
		matches bool
	}{
		{"tm.events.type='NewBlock'", map[string][]string{"tm.events.type": {"NewBlock"}}, true},
		{"tx.gas > 7", map[string][]string{"tx.gas": {"8"}}, true},
		{"transfer.amount > 7", map[string][]string{"transfer.amount": {"8stake"}}, true},
		{"transfer.amount > 7", map[string][]string{"transfer.amount": {"8.045stake"}}, true},
		{"transfer.amount > 7.043", map[string][]string{"transfer.amount": {"8.045stake"}}, true},
		{"transfer.amount > 8.045", map[string][]string{"transfer.amount": {"8.045stake"}}, false},
		{"tx.gas > 7 AND tx.gas < 9", map[string][]string{"tx.gas": {"8"}}, true},
		{"body.weight >= 3.5", map[string][]string{"body.weight": {"3.5"}}, true},
		{"account.balance < 1000.0", map[string][]string{"account.balance": {"900"}}, true},
		{"apples.kg <= 4", map[string][]string{"apples.kg": {"4.0"}}, true},
		{"body.weight >= 4.5", map[string][]string{"body.weight": {fmt.Sprintf("%v", float32(4.5))}}, true},
		{
			"oranges.kg < 4 AND watermellons.kg > 10",
			map[string][]string{"oranges.kg": {"3"}, "watermellons.kg": {"12"}},
			true,
		},
		{"peaches.kg < 4", map[string][]string{"peaches.kg": {"5"}}, false},
		{
			"tx.date > DATE 2017-01-01",
			map[string][]string{"tx.date": {time.Now().Format(query.DateLayout)}},
			true,
		},
		{"tx.date = DATE 2017-01-01", map[string][]string{"tx.date": {txDate}}, true},
		{"tx.date = DATE 2018-01-01", map[string][]string{"tx.date": {txDate}}, false},
		{
			"tx.time >= TIME 2013-05-03T14:45:00Z",
			map[string][]string{"tx.time": {time.Now().Format(query.TimeLayout)}},
			true,
		},
		{"tx.time = TIME 2013-05-03T14:45:00Z", map[string][]string{"tx.time": {txTime}}, false},
		{"abci.owner.name CONTAINS 'Igor'", map[string][]string{"abci.owner.name": {"Igor,Ivan"}}, true},
		{"abci.owner.name CONTAINS 'Igor'", map[string][]string{"abci.owner.name": {"Pavel,Ivan"}}, false},
		{"abci.owner.name = 'Igor'", map[string][]string{"abci.owner.name": {"Igor", "Ivan"}}, true},
		{
			"abci.owner.name = 'Ivan'",
			map[string][]string{"abci.owner.name": {"Igor", "Ivan"}},
			true,
		},
		{
			"abci.owner.name = 'Ivan' AND abci.owner.name = 'Igor'",
			map[string][]string{"abci.owner.name": {"Igor", "Ivan"}},
			true,
		},
		{
			"abci.owner.name = 'Ivan' AND abci.owner.name = 'John'",
			map[string][]string{"abci.owner.name": {"Igor", "Ivan"}},
			false,
		},
		{
			"tm.events.type='NewBlock'",
			map[string][]string{"tm.events.type": {"NewBlock"}, "app.name": {"fuzzed"}},
			true,
		},
		{
			"app.name = 'fuzzed'",
			map[string][]string{"tm.events.type": {"NewBlock"}, "app.name": {"fuzzed"}},
			true,
		},
		{
			"tm.events.type='NewBlock' AND app.name = 'fuzzed'",
			map[string][]string{"tm.events.type": {"NewBlock"}, "app.name": {"fuzzed"}},
			true,
		},
		{
			"tm.events.type='NewHeader' AND app.name = 'fuzzed'",
			map[string][]string{"tm.events.type": {"NewBlock"}, "app.name": {"fuzzed"}},
			false,
		},
		{"slash EXISTS",
			map[string][]string{"slash.reason": {"missing_signature"}, "slash.power": {"6000"}},
			true,
		},
		{"sl EXISTS",
			map[string][]string{"slash.reason": {"missing_signature"}, "slash.power": {"6000"}},
			true,
		},
		{"slash EXISTS",
			map[string][]string{"transfer.recipient": {"cosmos1gu6y2a0ffteesyeyeesk23082c6998xyzmt9mz"},
				"transfer.sender": {"cosmos1crje20aj4gxdtyct7z3knxqry2jqt2fuaey6u5"}},
			false,
		},
		{"slash.reason EXISTS AND slash.power > 1000",
			map[string][]string{"slash.reason": {"missing_signature"}, "slash.power": {"6000"}},
			true,
		},
		{"slash.reason EXISTS AND slash.power > 1000",
			map[string][]string{"slash.reason": {"missing_signature"}, "slash.power": {"500"}},
			false,
		},
		{"slash.reason EXISTS",
			map[string][]string{"transfer.recipient": {"cosmos1gu6y2a0ffteesyeyeesk23082c6998xyzmt9mz"},
				"transfer.sender": {"cosmos1crje20aj4gxdtyct7z3knxqry2jqt2fuaey6u5"}},
			false,
		},
	}

	for _, tc := range testCases {
		q, err := query.New(tc.s)
		require.Nil(t, err)
		require.NotNil(t, q, "Query '%s' should not be nil", tc.s)

		rawEvents := expandEvents(tc.events)
		match, err := q.Matches(rawEvents)
		require.Nil(t, err, "Query '%s' should not error on input %v", tc.s, tc.events)
		require.Equal(t, tc.matches, match, "Query '%s' on input %v: got %v, want %v",
			tc.s, tc.events, match, tc.matches)
	}
}

func TestMustParse(t *testing.T) {
	require.Panics(t, func() { query.MustParse("=") })
	require.NotPanics(t, func() { query.MustParse("tm.events.type='NewBlock'") })
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
		require.Equal(t, tc.conditions, c)
	}
}
