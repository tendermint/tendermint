package log_test

import (
	"io/ioutil"
	"testing"

	"github.com/tendermint/tmlibs/log"
)

func BenchmarkTMLoggerSimple(b *testing.B) {
	benchmarkRunner(b, log.NewTMLogger(ioutil.Discard), baseInfoMessage)
}

func BenchmarkTMLoggerContextual(b *testing.B) {
	benchmarkRunner(b, log.NewTMLogger(ioutil.Discard), withInfoMessage)
}

func benchmarkRunner(b *testing.B, logger log.Logger, f func(log.Logger)) {
	lc := logger.With("common_key", "common_value")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f(lc)
	}
}

var (
	baseInfoMessage = func(logger log.Logger) { logger.Info("foo_message", "foo_key", "foo_value") }
	withInfoMessage = func(logger log.Logger) { logger.With("a", "b").Info("c", "d", "f") }
)
