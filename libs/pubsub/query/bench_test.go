package query_test

import (
	"testing"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	oldquery "github.com/tendermint/tendermint/libs/pubsub/query/oldquery"
)

const testQuery = `tm.events.type='NewBlock' AND abci.account.name='Igor'`

var testEvents = []types.Event{
	{
		Type: "tm.events",
		Attributes: []types.EventAttribute{{
			Key:   "index",
			Value: "25",
		}, {
			Key:   "type",
			Value: "NewBlock",
		}},
	},
	{
		Type: "abci.account",
		Attributes: []types.EventAttribute{{
			Key:   "name",
			Value: "Anya",
		}, {
			Key:   "name",
			Value: "Igor",
		}},
	},
}

func BenchmarkParsePEG(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := oldquery.New(testQuery)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseCustom(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := query.New(testQuery)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMatchPEG(b *testing.B) {
	q, err := oldquery.New(testQuery)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ok, err := q.Matches(testEvents)
		if err != nil {
			b.Fatal(err)
		} else if !ok {
			b.Error("no match")
		}
	}
}

func BenchmarkMatchCustom(b *testing.B) {
	q, err := query.New(testQuery)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ok, err := q.Matches(testEvents)
		if err != nil {
			b.Fatal(err)
		} else if !ok {
			b.Error("no match")
		}
	}
}
