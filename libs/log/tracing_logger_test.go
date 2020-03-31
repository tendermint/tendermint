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
	err1 := errors.New("courage is grace under pressure")
	err2 := errors.New("it does not matter how slowly you go, so long as you do not stop")
	logger1.With("err1", err1).Info("foo", "err2", err2)

	want := strings.Replace(
		strings.Replace(
			`{"_msg":"foo","err1":"`+
				fmt.Sprintf("%+v", err1)+
				`","err2":"`+
				fmt.Sprintf("%+v", err2)+
				`","level":"info"}`,
			"\t", "", -1,
		), "\n", "", -1)
	have := strings.Replace(strings.Replace(strings.TrimSpace(buf.String()), "\\n", "", -1), "\\t", "", -1)
	if want != have {
		t.Errorf("\nwant '%s'\nhave '%s'", want, have)
	}

	buf.Reset()

	logger.With(
		"err1", stderr.New("opportunities don't happen. You create them"),
	).Info(
		"foo", "err2", stderr.New("once you choose hope, anything's possible"),
	)

	want = `{"_msg":"foo",` +
		`"err1":"opportunities don't happen. You create them",` +
		`"err2":"once you choose hope, anything's possible",` +
		`"level":"info"}`
	have = strings.TrimSpace(buf.String())
	if want != have {
		t.Errorf("\nwant '%s'\nhave '%s'", want, have)
	}

	buf.Reset()

	logger.With("user", "Sam").With("context", "value").Info("foo", "bar", "baz")

	want = `{"_msg":"foo","bar":"baz","context":"value","level":"info","user":"Sam"}`
	have = strings.TrimSpace(buf.String())
	if want != have {
		t.Errorf("\nwant '%s'\nhave '%s'", want, have)
	}
}
