package log_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"math"
	"regexp"
	"testing"

	kitlog "github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/libs/log"
)

func TestTMFmtLogger(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	logger := log.NewTMFmtLogger(buf)

	if err := logger.Log("hello", "world"); err != nil {
		t.Fatal(err)
	}
	assert.Regexp(t, regexp.MustCompile(`N\[.+\] unknown \s+ hello=world\n$`), buf.String())

	buf.Reset()
	if err := logger.Log("a", 1, "err", errors.New("error")); err != nil {
		t.Fatal(err)
	}
	assert.Regexp(t, regexp.MustCompile(`N\[.+\] unknown \s+ a=1 err=error\n$`), buf.String())

	buf.Reset()
	if err := logger.Log("std_map", map[int]int{1: 2}, "my_map", mymap{0: 0}); err != nil {
		t.Fatal(err)
	}
	assert.Regexp(t, regexp.MustCompile(`N\[.+\] unknown \s+ std_map=map\[1:2\] my_map=special_behavior\n$`), buf.String())

	buf.Reset()
	if err := logger.Log("level", "error"); err != nil {
		t.Fatal(err)
	}
	assert.Regexp(t, regexp.MustCompile(`E\[.+\] unknown \s+\n$`), buf.String())

	buf.Reset()
	if err := logger.Log("_msg", "Hello"); err != nil {
		t.Fatal(err)
	}
	assert.Regexp(t, regexp.MustCompile(`N\[.+\] Hello \s+\n$`), buf.String())

	buf.Reset()
	if err := logger.Log("module", "main", "module", "crypto", "module", "wire"); err != nil {
		t.Fatal(err)
	}
	assert.Regexp(t, regexp.MustCompile(`N\[.+\] unknown \s+module=wire\s+\n$`), buf.String())
}

func BenchmarkTMFmtLoggerSimple(b *testing.B) {
	benchmarkRunnerKitlog(b, log.NewTMFmtLogger(ioutil.Discard), baseMessage)
}

func BenchmarkTMFmtLoggerContextual(b *testing.B) {
	benchmarkRunnerKitlog(b, log.NewTMFmtLogger(ioutil.Discard), withMessage)
}

func TestTMFmtLoggerConcurrency(t *testing.T) {
	t.Parallel()
	testConcurrency(t, log.NewTMFmtLogger(ioutil.Discard), 10000)
}

func benchmarkRunnerKitlog(b *testing.B, logger kitlog.Logger, f func(kitlog.Logger)) {
	lc := kitlog.With(logger, "common_key", "common_value")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f(lc)
	}
}

var (
	baseMessage = func(logger kitlog.Logger) { logger.Log("foo_key", "foo_value") }
	withMessage = func(logger kitlog.Logger) { kitlog.With(logger, "a", "b").Log("d", "f") }
)

// These test are designed to be run with the race detector.

func testConcurrency(t *testing.T, logger kitlog.Logger, total int) {
	n := int(math.Sqrt(float64(total)))
	share := total / n

	errC := make(chan error, n)

	for i := 0; i < n; i++ {
		go func() {
			errC <- spam(logger, share)
		}()
	}

	for i := 0; i < n; i++ {
		err := <-errC
		if err != nil {
			t.Fatalf("concurrent logging error: %v", err)
		}
	}
}

func spam(logger kitlog.Logger, count int) error {
	for i := 0; i < count; i++ {
		err := logger.Log("key", i)
		if err != nil {
			return err
		}
	}
	return nil
}

type mymap map[int]int

func (m mymap) String() string { return "special_behavior" }
