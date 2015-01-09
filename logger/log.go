package logger

import (
	"fmt"
	"os"

	"github.com/tendermint/log15"
	"github.com/tendermint/tendermint/config"
)

var rootHandler log15.Handler

func init() {
	handlers := []log15.Handler{}

	// By default, there's a stdout terminal format handler.
	handlers = append(handlers, log15.LvlFilterHandler(
		log15.LvlDebug,
		log15.StreamHandler(os.Stdout, log15.TerminalFormat()),
	))

	// Maybe also write to a file.
	if config.App.GetString("Log.Dir") != "" {
		// Create log dir if it doesn't exist
		err := os.MkdirAll(config.App.GetString("Log.Dir"), 0700)
		if err != nil {
			fmt.Printf("Could not create directory: %v", err)
			os.Exit(1)
		}
		// File handler
		handlers = append(handlers, log15.LvlFilterHandler(
			log15.LvlDebug,
			log15.Must.FileHandler(config.App.GetString("Log.Dir")+"/tendermint.log", log15.LogfmtFormat()),
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
