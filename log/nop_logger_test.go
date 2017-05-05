package log_test

import (
	"testing"

	"github.com/tendermint/tmlibs/log"
)

func TestNopLogger(t *testing.T) {
	t.Parallel()
	logger := log.NewNopLogger()
	if err := logger.Info("Hello", "abc", 123); err != nil {
		t.Error(err)
	}
	if err := logger.With("def", "ghi").Debug(""); err != nil {
		t.Error(err)
	}
}
