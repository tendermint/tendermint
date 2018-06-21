package benchmarks

import (
	"testing"

	cmn "github.com/tendermint/tendermint/libs/common"
)

func BenchmarkSomething(b *testing.B) {
	b.StopTimer()
	numItems := 100000
	numChecks := 100000
	keys := make([]string, numItems)
	for i := 0; i < numItems; i++ {
		keys[i] = cmn.RandStr(100)
	}
	txs := make([]string, numChecks)
	for i := 0; i < numChecks; i++ {
		txs[i] = cmn.RandStr(100)
	}
	b.StartTimer()

	counter := 0
	for j := 0; j < b.N; j++ {
		foo := make(map[string]string)
		for _, key := range keys {
			foo[key] = key
		}
		for _, tx := range txs {
			if _, ok := foo[tx]; ok {
				counter++
			}
		}
	}
}
