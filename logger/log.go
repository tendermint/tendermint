package logger

import (
	"fmt"
	"os"

	"github.com/tendermint/log15"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
)

var rootHandler log15.Handler

func getLevel(lvlString string) log15.Lvl {
	lvl, err := log15.LvlFromString(lvlString)
	if err != nil {
		Exit(Fmt("Invalid log level %v: %v", lvlString, err))
	}
	return lvl
}

func init() {
	handlers := []log15.Handler{}

	// By default, there's a stdout terminal format handler.
	handlers = append(handlers, log15.LvlFilterHandler(
		getLevel(config.App().GetString("Log.Stdout.Level")),
		log15.StreamHandler(os.Stdout, log15.TerminalFormat()),
	))

	// Maybe also write to a file.
	if _logFileDir := config.App().GetString("Log.File.Dir"); _logFileDir != "" {
		// Create log dir if it doesn't exist
		err := os.MkdirAll(_logFileDir, 0700)
		if err != nil {
			fmt.Printf("Could not create directory: %v", err)
			os.Exit(1)
		}
		// File handler
		handlers = append(handlers, log15.LvlFilterHandler(
			getLevel(config.App().GetString("Log.File.Level")),
			log15.Must.FileHandler(_logFileDir+"/tendermint.log", log15.LogfmtFormat()),
		))
	}

	// Set rootHandler.
	rootHandler = log15.MultiHandler(handlers...)

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
