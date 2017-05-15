package flags_test

import (
	"testing"

	tmflags "github.com/tendermint/tendermint/cmd/tendermint/commands/flags"
	"github.com/tendermint/tmlibs/log"
)

func TestIsLogLevelSimple(t *testing.T) {
	simpleFlags := []string{"info", "debug", "some"}
	for _, f := range simpleFlags {
		if !tmflags.IsLogLevelSimple(f) {
			t.Fatalf("%s is a simple flag", f)
		}
	}

	complexFlags := []string{"mempool:error", "mempool:error,*:debug"}
	for _, f := range complexFlags {
		if tmflags.IsLogLevelSimple(f) {
			t.Fatalf("%s is a complex flag", f)
		}
	}
}

func TestParseComplexLogLevel(t *testing.T) {
	logger := log.TestingLogger()

	correctLogLevels := []string{"mempool:error", "mempool:error,*:debug", "*:debug,wire:none"}
	for _, lvl := range correctLogLevels {
		if _, err := tmflags.ParseComplexLogLevel(lvl, logger); err != nil {
			t.Fatal(err)
		}
	}

	incorrectLogLevel := []string{"some", "mempool:some", "*:some,mempool:error"}
	for _, lvl := range incorrectLogLevel {
		if _, err := tmflags.ParseComplexLogLevel(lvl, logger); err == nil {
			t.Fatalf("Expected %s to produce error", lvl)
		}
	}
}
