package log_test

import (
	"bytes"
	stderr "errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/libs/log"
)

func TestTracingLogger(t *testing.T) {
	var buf bytes.Buffer

	logger := log.NewTMJSONLogger(&buf)

	logger1 := log.NewTracingLogger(logger)
	err1 := errors.New("Courage is grace under pressure.")
	err2 := errors.New("It does not matter how slowly you go, so long as you do not stop.")
	logger1.With("err1", err1).Info("foo", "err2", err2)
	have := strings.Replace(strings.Replace(strings.TrimSpace(buf.String()), "\\n", "", -1), "\\t", "", -1)
	if want := strings.Replace(strings.Replace(`{"_msg":"foo","err1":"`+fmt.Sprintf("%+v", err1)+`","err2":"`+fmt.Sprintf("%+v", err2)+`","level":"info"}`, "\t", "", -1), "\n", "", -1); want != have {
		t.Errorf("\nwant '%s'\nhave '%s'", want, have)
	}

	buf.Reset()

	logger.With("err1", stderr.New("Opportunities don't happen. You create them.")).Info("foo", "err2", stderr.New("Once you choose hope, anything's possible."))
	if want, have := `{"_msg":"foo","err1":"Opportunities don't happen. You create them.","err2":"Once you choose hope, anything's possible.","level":"info"}`, strings.TrimSpace(buf.String()); want != have {
		t.Errorf("\nwant '%s'\nhave '%s'", want, have)
	}

	buf.Reset()

	logger.With("user", "Sam").With("context", "value").Info("foo", "bar", "baz")
	if want, have := `{"_msg":"foo","bar":"baz","context":"value","level":"info","user":"Sam"}`, strings.TrimSpace(buf.String()); want != have {
		t.Errorf("\nwant '%s'\nhave '%s'", want, have)
	}
}
