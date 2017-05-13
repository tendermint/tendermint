package log_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tendermint/tmlibs/log"
)

func TestVariousLevels(t *testing.T) {
	testCases := []struct {
		name    string
		allowed log.Option
		want    string
	}{
		{
			"AllowAll",
			log.AllowAll(),
			strings.Join([]string{
				`{"_msg":"here","level":"debug","this is":"debug log"}`,
				`{"_msg":"here","level":"info","this is":"info log"}`,
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
		},
		{
			"AllowDebug",
			log.AllowDebug(),
			strings.Join([]string{
				`{"_msg":"here","level":"debug","this is":"debug log"}`,
				`{"_msg":"here","level":"info","this is":"info log"}`,
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
		},
		{
			"AllowInfo",
			log.AllowInfo(),
			strings.Join([]string{
				`{"_msg":"here","level":"info","this is":"info log"}`,
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
		},
		{
			"AllowError",
			log.AllowError(),
			strings.Join([]string{
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
		},
		{
			"AllowNone",
			log.AllowNone(),
			``,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := log.NewFilter(log.NewTMJSONLogger(&buf), tc.allowed)

			logger.Debug("here", "this is", "debug log")
			logger.Info("here", "this is", "info log")
			logger.Error("here", "this is", "error log")

			if want, have := tc.want, strings.TrimSpace(buf.String()); want != have {
				t.Errorf("\nwant:\n%s\nhave:\n%s", want, have)
			}
		})
	}
}

func TestErrNotAllowed(t *testing.T) {
	myError := errors.New("squelched!")
	opts := []log.Option{
		log.AllowError(),
		log.ErrNotAllowed(myError),
	}
	logger := log.NewFilter(log.NewNopLogger(), opts...)

	if want, have := myError, logger.Info("foo", "bar", "baz"); want != have {
		t.Errorf("want %#+v, have %#+v", want, have)
	}

	if want, have := error(nil), logger.Error("foo", "bar", "baz"); want != have {
		t.Errorf("want %#+v, have %#+v", want, have)
	}
}

func TestLevelContext(t *testing.T) {
	var buf bytes.Buffer

	var logger log.Logger
	logger = log.NewTMJSONLogger(&buf)
	logger = log.NewFilter(logger, log.AllowError())
	logger = logger.With("context", "value")

	logger.Error("foo", "bar", "baz")
	if want, have := `{"_msg":"foo","bar":"baz","context":"value","level":"error"}`, strings.TrimSpace(buf.String()); want != have {
		t.Errorf("\nwant '%s'\nhave '%s'", want, have)
	}

	buf.Reset()
	logger.Info("foo", "bar", "baz")
	if want, have := ``, strings.TrimSpace(buf.String()); want != have {
		t.Errorf("\nwant '%s'\nhave '%s'", want, have)
	}
}

func TestNewFilterByLevel(t *testing.T) {
	var logger log.Logger
	logger = log.NewNopLogger()
	if _, err := log.NewFilterByLevel(logger, "info"); err != nil {
		t.Fatal(err)
	}
	if _, err := log.NewFilterByLevel(logger, "other"); err == nil {
		t.Fatal(err)
	}
}
