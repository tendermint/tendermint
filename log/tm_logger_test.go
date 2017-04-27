package log_test

import (
	"io/ioutil"
	"testing"

	"github.com/tendermint/tmlibs/log"
)

func TestTmLogger(t *testing.T) {
	t.Parallel()
	logger := log.NewTmLogger(ioutil.Discard)
	if err := logger.Info("Hello", "abc", 123); err != nil {
		t.Error(err)
	}
	if err := log.With(logger, "def", "ghi").Debug(""); err != nil {
		t.Error(err)
	}
}

func BenchmarkTmLoggerSimple(b *testing.B) {
	benchmarkRunner(b, log.NewTmLogger(ioutil.Discard), baseInfoMessage)
}

func BenchmarkTmLoggerContextual(b *testing.B) {
	benchmarkRunner(b, log.NewTmLogger(ioutil.Discard), withInfoMessage)
}

func benchmarkRunner(b *testing.B, logger log.Logger, f func(log.Logger)) {
	lc := log.With(logger, "common_key", "common_value")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f(lc)
	}
}

var (
	baseInfoMessage = func(logger log.Logger) { logger.Info("foo_message", "foo_key", "foo_value") }
	withInfoMessage = func(logger log.Logger) { log.With(logger, "a", "b").Info("c", "d", "f") }
)
