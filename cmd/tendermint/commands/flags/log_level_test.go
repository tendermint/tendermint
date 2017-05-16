package flags_test

import (
	"bytes"
	"strings"
	"testing"

	tmflags "github.com/tendermint/tendermint/cmd/tendermint/commands/flags"
	"github.com/tendermint/tmlibs/log"
)

func TestParseLogLevel(t *testing.T) {
	var buf bytes.Buffer
	jsonLogger := log.NewTMJSONLogger(&buf)

	correctLogLevels := []struct {
		lvl              string
		expectedLogLines []string
	}{
		{"mempool:error", []string{``, ``, `{"_msg":"Mesmero","level":"error","module":"mempool"}`}},
		{"mempool:error,*:debug", []string{``, ``, `{"_msg":"Mesmero","level":"error","module":"mempool"}`}},
		{"*:debug,wire:none", []string{
			`{"_msg":"Kingpin","level":"debug","module":"mempool"}`,
			`{"_msg":"Kitty Pryde","level":"info","module":"mempool"}`,
			`{"_msg":"Mesmero","level":"error","module":"mempool"}`}},
	}

	for _, c := range correctLogLevels {
		logger, err := tmflags.ParseLogLevel(c.lvl, jsonLogger)
		if err != nil {
			t.Fatal(err)
		}

		logger = logger.With("module", "mempool")

		buf.Reset()

		logger.Debug("Kingpin")
		if have := strings.TrimSpace(buf.String()); c.expectedLogLines[0] != have {
			t.Errorf("\nwant '%s'\nhave '%s'\nlevel '%s'", c.expectedLogLines[0], have, c.lvl)
		}

		buf.Reset()

		logger.Info("Kitty Pryde")
		if have := strings.TrimSpace(buf.String()); c.expectedLogLines[1] != have {
			t.Errorf("\nwant '%s'\nhave '%s'\nlevel '%s'", c.expectedLogLines[1], have, c.lvl)
		}

		buf.Reset()

		logger.Error("Mesmero")
		if have := strings.TrimSpace(buf.String()); c.expectedLogLines[2] != have {
			t.Errorf("\nwant '%s'\nhave '%s'\nlevel '%s'", c.expectedLogLines[2], have, c.lvl)
		}
	}

	incorrectLogLevel := []string{"some", "mempool:some", "*:some,mempool:error"}
	for _, lvl := range incorrectLogLevel {
		if _, err := tmflags.ParseLogLevel(lvl, jsonLogger); err == nil {
			t.Fatalf("Expected %s to produce error", lvl)
		}
	}
}
