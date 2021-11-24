package query_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/libs/pubsub/query/syntax"
)

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
func newTestEvents(ss []string) []types.Event {
	var events []types.Event
	for _, s := range ss {
		events = append(events, newTestEvent(s))
	}
	return events
}

func splitKV(s string) (key, value string) {
	kv := strings.SplitN(s, "=", 2)
	return kv[0], kv[1]
}

func TestCompiledMatches(t *testing.T) {
	var (
		txDate = "2017-01-01"
		txTime = "2018-05-03T14:45:00Z"
	)

	testCases := []struct {
		s        string
		events   []string
		matches  bool
		matchErr bool
	}{
		{`tm.events.type='NewBlock'`,
			[]string{`tm|events.type=NewBlock`},
			true, false},
		{`tx.gas > 7`,
			[]string{`tx|gas=8`},
			true, false},
		{`transfer.amount > 7`,
			[]string{`transfer|amount=8stake`},
			true, false},
		{`transfer.amount > 7`,
			[]string{`transfer|amount=8.045`},
			true, false},
		{`transfer.amount > 7.043`,
			[]string{`transfer|amount=8.045stake`},
			true, false},
		{`transfer.amount > 8.045`,
			[]string{`transfer|amount=8.045stake`},
			false, false},
		{`tx.gas > 7 AND tx.gas < 9`,
			[]string{`tx|gas=8`},
			true, false},
		{`body.weight >= 3.5`,
			[]string{`body|weight=3.5`},
			true, false},
		{`account.balance < 1000.0`,
			[]string{`account|balance=900`},
			true, false},
		{`apples.kg <= 4`,
			[]string{`apples|kg=4.0`},
			true, false},
		{`body.weight >= 4.5`,
			[]string{`body|weight=4.5`},
			true, false},
		{`oranges.kg < 4 AND watermellons.kg > 10`,
			[]string{`oranges|kg=3`, `watermellons|kg=12`},
			true, false},
		{`peaches.kg < 4`,
			[]string{`peaches|kg=5`},
			false, false},
		{`tx.date > DATE 2017-01-01`,
			[]string{`tx|date=` + time.Now().Format(syntax.DateFormat)},
			true, false},
		{`tx.date = DATE 2017-01-01`,
			[]string{`tx|date=` + txDate},
			true, false},
		{`tx.date = DATE 2018-01-01`,
			[]string{`tx|date=` + txDate},
			false, false},
		{`tx.time >= TIME 2013-05-03T14:45:00Z`,
			[]string{`tx|time=` + time.Now().Format(syntax.TimeFormat)},
			true, false},
		{`tx.time = TIME 2013-05-03T14:45:00Z`,
			[]string{`tx|time=` + txTime},
			false, false},
		{`abci.owner.name CONTAINS 'Igor'`,
			[]string{`abci|owner.name=Igor|owner.name=Ivan`},
			true, false},
		{`abci.owner.name CONTAINS 'Igor'`,
			[]string{`abci|owner.name=Pavel|owner.name=Ivan`},
			false, false},
		{`abci.owner.name = 'Igor'`,
			[]string{`abci|owner.name=Igor|owner.name=Ivan`},
			true, false},
		{`abci.owner.name = 'Ivan'`,
			[]string{`abci|owner.name=Igor|owner.name=Ivan`},
			true, false},
		{`abci.owner.name = 'Ivan' AND abci.owner.name = 'Igor'`,
			[]string{`abci|owner.name=Igor|owner.name=Ivan`},
			true, false},
		{`abci.owner.name = 'Ivan' AND abci.owner.name = 'John'`,
			[]string{`abci|owner.name=Igor|owner.name=Ivan`},
			false, false},
		{`tm.events.type='NewBlock'`,
			[]string{`tm|events.type=NewBlock`, `app|name=fuzzed`},
			true, false},
		{`app.name = 'fuzzed'`,
			[]string{`tm|events.type=NewBlock`, `app|name=fuzzed`},
			true, false},
		{`tm.events.type='NewBlock' AND app.name = 'fuzzed'`,
			[]string{`tm|events.type=NewBlock`, `app|name=fuzzed`},
			true, false},
		{`tm.events.type='NewHeader' AND app.name = 'fuzzed'`,
			[]string{`tm|events.type=NewBlock`, `app|name=fuzzed`},
			false, false},
		{`slash EXISTS`,
			[]string{`slash|reason=missing_signature|power=6000`},
			true, false},
		{`slash EXISTS`,
			[]string{`transfer|recipient=cosmos1gu6y2a0ffteesyeyeesk23082c6998xyzmt9mz|sender=cosmos1crje20aj4gxdtyct7z3knxqry2jqt2fuaey6u5`},
			false, false},
		{`slash.reason EXISTS AND slash.power > 1000`,
			[]string{`slash|reason=missing_signature|power=6000`},
			true, false},
		{`slash.reason EXISTS AND slash.power > 1000`,
			[]string{`slash|reason=missing_signature|power=500`},
			false, false},
		{`slash.reason EXISTS`,
			[]string{`transfer|recipient=cosmos1gu6y2a0ffteesyeyeesk23082c6998xyzmt9mz|sender=cosmos1crje20aj4gxdtyct7z3knxqry2jqt2fuaey6u5`},
			false, false},
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
	// correctly handle variable type/attribute splits ("x", "y.z" == "x.y", "z")
	// since that was reqired by the original "flattened" event representation.

	for _, tc := range testCases {
		c, err := query.NewCompiled(tc.s)
		if err != nil {
			t.Errorf("NewCompiled %#q: unexpected error: %v", tc.s, err)
			continue
		}

		events := newTestEvents(tc.events)
		got, err := c.Matches(events)
		if err != nil {
			t.Errorf("Query: %#q\nInput: %+v\nMatches: got error %v",
				tc.s, events, err)
		}
		if got != tc.matches {
			t.Errorf("Query: %#q\nInput: %+v\nMatches: got %v, want %v",
				tc.s, events, got, tc.matches)
		}
	}
}
