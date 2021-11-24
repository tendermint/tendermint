package query_test

import (
	"strings"
	"testing"

	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/libs/pubsub/query/syntax"
)

func BenchmarkParsePEG(b *testing.B) {
	const input = `tm.events.type='NewBlock' AND abci.account.name='Igor'`

	for i := 0; i < b.N; i++ {
		_, err := query.New(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseCustom(b *testing.B) {
	const input = `tm.events.type='NewBlock' AND abci.account.name='Igor'`

	for i := 0; i < b.N; i++ {
		_, err := syntax.NewParser(strings.NewReader(input)).Parse()
		if err != nil {
			b.Fatal(err)
		}
	}
}
