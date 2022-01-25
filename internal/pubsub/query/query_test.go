package query_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/pubsub/query"
	"github.com/tendermint/tendermint/internal/pubsub/query/syntax"
)

// Example events from the OpenAPI documentation:
//  https://github.com/tendermint/tendermint/blob/master/rpc/openapi/openapi.yaml
//
// Redactions:
//
//   - Add an explicit "tm" event for the built-in attributes.
//   - Remove Index fields (not relevant to tests).
//   - Add explicit balance values (to use in tests).
//
var apiEvents = []types.Event{
	{
		Type: "tm",
		Attributes: []types.EventAttribute{
			{Key: "event", Value: "Tx"},
			{Key: "hash", Value: "XYZ"},
			{Key: "height", Value: "5"},
		},
	},
	{
		Type: "rewards.withdraw",
		Attributes: []types.EventAttribute{
			{Key: "address", Value: "AddrA"},
			{Key: "source", Value: "SrcX"},
			{Key: "amount", Value: "100"},
			{Key: "balance", Value: "1500"},
		},
	},
	{
		Type: "rewards.withdraw",
		Attributes: []types.EventAttribute{
			{Key: "address", Value: "AddrB"},
			{Key: "source", Value: "SrcY"},
			{Key: "amount", Value: "45"},
			{Key: "balance", Value: "999"},
		},
	},
	{
		Type: "transfer",
		Attributes: []types.EventAttribute{
			{Key: "sender", Value: "AddrC"},
			{Key: "recipient", Value: "AddrD"},
			{Key: "amount", Value: "160"},
		},
	},
}

func TestCompiledMatches(t *testing.T) {
	var (
		txDate = "2017-01-01"
		txTime = "2018-05-03T14:45:00Z"
	)

	testCases := []struct {
		s       string
		events  []types.Event
		matches bool
	}{
		{`tm.events.type='NewBlock'`,
			newTestEvents(`tm|events.type=NewBlock`),
			true},
		{`tx.gas > 7`,
			newTestEvents(`tx|gas=8`),
			true},
		{`transfer.amount > 7`,
			newTestEvents(`transfer|amount=8stake`),
			true},
		{`transfer.amount > 7`,
			newTestEvents(`transfer|amount=8.045`),
			true},
		{`transfer.amount > 7.043`,
			newTestEvents(`transfer|amount=8.045stake`),
			true},
		{`transfer.amount > 8.045`,
			newTestEvents(`transfer|amount=8.045stake`),
			false},
		{`tx.gas > 7 AND tx.gas < 9`,
			newTestEvents(`tx|gas=8`),
			true},
		{`body.weight >= 3.5`,
			newTestEvents(`body|weight=3.5`),
			true},
		{`account.balance < 1000.0`,
			newTestEvents(`account|balance=900`),
			true},
		{`apples.kg <= 4`,
			newTestEvents(`apples|kg=4.0`),
			true},
		{`body.weight >= 4.5`,
			newTestEvents(`body|weight=4.5`),
			true},
		{`oranges.kg < 4 AND watermellons.kg > 10`,
			newTestEvents(`oranges|kg=3`, `watermellons|kg=12`),
			true},
		{`peaches.kg < 4`,
			newTestEvents(`peaches|kg=5`),
			false},
		{`tx.date > DATE 2017-01-01`,
			newTestEvents(`tx|date=` + time.Now().Format(syntax.DateFormat)),
			true},
		{`tx.date = DATE 2017-01-01`,
			newTestEvents(`tx|date=` + txDate),
			true},
		{`tx.date = DATE 2018-01-01`,
			newTestEvents(`tx|date=` + txDate),
			false},
		{`tx.time >= TIME 2013-05-03T14:45:00Z`,
			newTestEvents(`tx|time=` + time.Now().Format(syntax.TimeFormat)),
			true},
		{`tx.time = TIME 2013-05-03T14:45:00Z`,
			newTestEvents(`tx|time=` + txTime),
			false},
		{`abci.owner.name CONTAINS 'Igor'`,
			newTestEvents(`abci|owner.name=Igor|owner.name=Ivan`),
			true},
		{`abci.owner.name CONTAINS 'Igor'`,
			newTestEvents(`abci|owner.name=Pavel|owner.name=Ivan`),
			false},
		{`abci.owner.name = 'Igor'`,
			newTestEvents(`abci|owner.name=Igor|owner.name=Ivan`),
			true},
		{`abci.owner.name = 'Ivan'`,
			newTestEvents(`abci|owner.name=Igor|owner.name=Ivan`),
			true},
		{`abci.owner.name = 'Ivan' AND abci.owner.name = 'Igor'`,
			newTestEvents(`abci|owner.name=Igor|owner.name=Ivan`),
			true},
		{`abci.owner.name = 'Ivan' AND abci.owner.name = 'John'`,
			newTestEvents(`abci|owner.name=Igor|owner.name=Ivan`),
			false},
		{`tm.events.type='NewBlock'`,
			newTestEvents(`tm|events.type=NewBlock`, `app|name=fuzzed`),
			true},
		{`app.name = 'fuzzed'`,
			newTestEvents(`tm|events.type=NewBlock`, `app|name=fuzzed`),
			true},
		{`tm.events.type='NewBlock' AND app.name = 'fuzzed'`,
			newTestEvents(`tm|events.type=NewBlock`, `app|name=fuzzed`),
			true},
		{`tm.events.type='NewHeader' AND app.name = 'fuzzed'`,
			newTestEvents(`tm|events.type=NewBlock`, `app|name=fuzzed`),
			false},
		{`slash EXISTS`,
			newTestEvents(`slash|reason=missing_signature|power=6000`),
			true},
		{`slash EXISTS`,
			newTestEvents(`transfer|recipient=cosmos1gu6y2a0ffteesyeyeesk23082c6998xyzmt9mz|sender=cosmos1crje20aj4gxdtyct7z3knxqry2jqt2fuaey6u5`),
			false},
		{`slash.reason EXISTS AND slash.power > 1000`,
			newTestEvents(`slash|reason=missing_signature|power=6000`),
			true},
		{`slash.reason EXISTS AND slash.power > 1000`,
			newTestEvents(`slash|reason=missing_signature|power=500`),
			false},
		{`slash.reason EXISTS`,
			newTestEvents(`transfer|recipient=cosmos1gu6y2a0ffteesyeyeesk23082c6998xyzmt9mz|sender=cosmos1crje20aj4gxdtyct7z3knxqry2jqt2fuaey6u5`),
			false},

		// Test cases based on the OpenAPI examples.
		{`tm.event = 'Tx' AND rewards.withdraw.address = 'AddrA'`,
			apiEvents, true},
		{`tm.event = 'Tx' AND rewards.withdraw.address = 'AddrA' AND rewards.withdraw.source = 'SrcY'`,
			apiEvents, true},
		{`tm.event = 'Tx' AND transfer.sender = 'AddrA'`,
			apiEvents, false},
		{`tm.event = 'Tx' AND transfer.sender = 'AddrC'`,
			apiEvents, true},
		{`tm.event = 'Tx' AND transfer.sender = 'AddrZ'`,
			apiEvents, false},
		{`tm.event = 'Tx' AND rewards.withdraw.address = 'AddrZ'`,
			apiEvents, false},
		{`tm.event = 'Tx' AND rewards.withdraw.source = 'W'`,
			apiEvents, false},
	}

	// NOTE: The original implementation allowed arbitrary prefix matches on
	// attribute tags, e.g., "sl" would match "slash".
	//
	// That is weird and probably wrong: "foo.ba" should not match "foo.bar",
	// or there is no way to distinguish the case where there were two values
	// for "foo.bar" or one value each for "foo.ba" and "foo.bar".
	//
	// Apart from a single test case, I could not find any attested usage of
	// this implementation detail. It isn't documented in the OpenAPI docs and
	// is not shown in any of the example inputs.
	//
	// On that basis, I removed that test case. This implementation still does
	// correctly handle variable type/attribute splits ("x", "y.z" / "x.y", "z")
	// since that was required by the original "flattened" event representation.

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%02d", i+1), func(t *testing.T) {
			c, err := query.New(tc.s)
			if err != nil {
				t.Fatalf("NewCompiled %#q: unexpected error: %v", tc.s, err)
			}

			got := c.Matches(tc.events)
			if got != tc.matches {
				t.Errorf("Query: %#q\nInput: %+v\nMatches: got %v, want %v",
					tc.s, tc.events, got, tc.matches)
			}
		})
	}
}

func TestAllMatchesAll(t *testing.T) {
	events := newTestEvents(
		``,
		`Asher|Roth=`,
		`Route|66=`,
		`Rilly|Blue=`,
	)
	for i := 0; i < len(events); i++ {
		if !query.All.Matches(events[:i]) {
			t.Errorf("Did not match on %+v ", events[:i])
		}
	}
}

// newTestEvent constructs an Event message from a template string.
// The format is "type|attr1=val1|attr2=val2|...".
func newTestEvent(s string) types.Event {
	var event types.Event
	parts := strings.Split(s, "|")
	event.Type = parts[0]
	if len(parts) == 1 {
		return event // type only, no attributes
	}
	for _, kv := range parts[1:] {
		key, val := splitKV(kv)
		event.Attributes = append(event.Attributes, types.EventAttribute{
			Key:   key,
			Value: val,
		})
	}
	return event
}

// newTestEvents constructs a slice of Event messages by applying newTestEvent
// to each element of ss.
func newTestEvents(ss ...string) []types.Event {
	events := make([]types.Event, len(ss))
	for i, s := range ss {
		events[i] = newTestEvent(s)
	}
	return events
}

func splitKV(s string) (key, value string) {
	kv := strings.SplitN(s, "=", 2)
	return kv[0], kv[1]
}
