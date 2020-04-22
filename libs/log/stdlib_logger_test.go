package log_test

import (
	"bytes"
	stdlog "log"
	"strings"
	"testing"

	"github.com/tendermint/tendermint/libs/log"
)

func TestStdLibLogger(t *testing.T) {
	var buf bytes.Buffer

	logger := log.NewStdLibLogger(&buf, "prefix ", stdlog.LstdFlags)
	logger.Info("foo", "baz", "bar")
	msg := strings.TrimSpace(buf.String())
	if !strings.Contains(msg, "info: foo baz=bar") {
		t.Errorf("expected logger to contain log level, msg, and keyvals, got %s", msg)
	}
	if !strings.Contains(msg, "prefix ") {
		t.Errorf("expected logger to contain prefix, got %s", msg)
	}

	buf.Reset()

	logger.Debug("foo", "baz", "bar")
	msg = strings.TrimSpace(buf.String())
	if !strings.Contains(msg, "debug: foo baz=bar") {
		t.Errorf("expected logger to contain log level, msg, and keyvals, got %s", msg)
	}

	buf.Reset()

	logger.Error("foo", "baz", "bar")
	msg = strings.TrimSpace(buf.String())
	if !strings.Contains(msg, "error: foo baz=bar") {
		t.Errorf("expected logger to contain log level, msg, and keyvals, got %s", msg)
	}

	buf.Reset()

	logger2 := logger.With("module", "devs")
	logger2.Error("foo", "baz", "bar")
	msg = strings.TrimSpace(buf.String())
	if !strings.Contains(msg, "error: foo module=devs baz=bar") {
		t.Errorf("expected logger to contain log level, msg, and keyvals, got %s", msg)
	}
}
