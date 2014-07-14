package common

import (
	stdlog "log"
	"os"

	"github.com/op/go-logging"
)

var Log = logging.MustGetLogger("main")

func init() {
	// Customize the output format
	logging.SetFormatter(logging.MustStringFormatter("▶ %{level:.1s} 0x%{id:x} %{message}"))

	// Setup one stdout and one syslog backend.
	logBackend := logging.NewLogBackend(os.Stderr, "", stdlog.LstdFlags|stdlog.Lshortfile)
	logBackend.Color = true

	syslogBackend, err := logging.NewSyslogBackend("")
	if err != nil {
		panic(err)
	}

	// Combine them both into one logging backend.
	logging.SetBackend(logBackend, syslogBackend)

	// Test
	/*
	   Log.Debug("debug")
	   Log.Info("info")
	   Log.Notice("notice")
	   Log.Warning("warning")
	   Log.Error("error")
	*/
}

var Debug = Log.Debug
var Info = Log.Info
var Notice = Log.Notice
var Warning = Log.Warning
var Warn = Log.Warning
var Error = Log.Error
var Critical = Log.Critical
