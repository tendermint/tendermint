package logger

import (
	"os"

	"github.com/tendermint/log15"
	. "github.com/tendermint/tendermint/common"
)

var rootHandler log15.Handler

func init() {
	Reset()
}

// You might want to call this after resetting tendermint/config.
func Reset() {

	var logLevel string = "debug"
	if config != nil {
		logLevel = config.GetString("log_level")
	}

	// stdout handler
	//handlers := []log15.Handler{}
	stdoutHandler := log15.LvlFilterHandler(
		getLevel(logLevel),
		log15.StreamHandler(os.Stdout, log15.TerminalFormat()),
	)
	//handlers = append(handlers, stdoutHandler)

	// Set rootHandler.
	//rootHandler = log15.MultiHandler(handlers...)
	rootHandler = stdoutHandler

	// By setting handlers on the root, we handle events from all loggers.
	log15.Root().SetHandler(rootHandler)
}

// See binary/log for an example of usage.
func RootHandler() log15.Handler {
	return rootHandler
}

func New(ctx ...interface{}) log15.Logger {
	return log15.Root().New(ctx...)
}

func getLevel(lvlString string) log15.Lvl {
	lvl, err := log15.LvlFromString(lvlString)
	if err != nil {
		Exit(Fmt("Invalid log level %v: %v", lvlString, err))
	}
	return lvl
}
