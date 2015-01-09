package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/tendermint/confer"
	flag "github.com/spf13/pflag"
)

var rootDir string
var App *confer.Config
var defaultConfig = `
# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

Network =         "tendermint_testnet0"
ListenAddr =      "0.0.0.0:0"
# First node to connect to.  Command-line overridable.
# SeedNode =          "a.b.c.d:pppp"

[DB]
# The only other available backend is "memdb"
Backend =         "leveldb"
# The leveldb data directory.
# Dir =           "<YOUR_HOME_DIRECTORY>/.tendermint/data"

[RPC]
# For the RPC API HTTP server.  Port required.
HTTP.ListenAddr = "0.0.0.0:8080"

[Alert]
# TODO: Document options

[SMTP]
# TODO: Document options
`

func init() {

	// Get RootDir
	rootDir = os.Getenv("TMROOT")
	if rootDir == "" {
		rootDir = os.Getenv("HOME") + "/.tendermint"
	}
	configFile := rootDir + "/config.toml"

	// Write default config file if missing.
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if strings.Index(configFile, "/") != -1 {
			err := os.MkdirAll(filepath.Dir(configFile), 0700)
			if err != nil {
				fmt.Printf("Could not create directory: %v", err)
				os.Exit(1)
			}
		}
		err := ioutil.WriteFile(configFile, []byte(defaultConfig), 0600)
		if err != nil {
			fmt.Printf("Could not write config file: %v", err)
			os.Exit(1)
		}
		fmt.Printf("Config file written to %v. Please edit & run again\n", configFile)
		os.Exit(1)
	}

	// Initialize Config
	App = confer.NewConfig()
	initDefaults()
	paths := []string{configFile}
	if err := App.ReadPaths(paths...); err != nil {
		log.Warn("Error reading configuration", "paths", paths, "error", err)
	}
}

func initDefaults() {
	App.SetDefault("Network", "tendermint_testnet0")
	App.SetDefault("ListenAddr", "0.0.0.0:0")
	App.SetDefault("DB.Backend", "leveldb")
	App.SetDefault("DB.Dir", rootDir+"/data")
	App.SetDefault("Log.Level", "debug")
	App.SetDefault("Log.Dir", rootDir+"/log")
	App.SetDefault("RPC.HTTP.ListenAddr", "0.0.0.0:8080")

	App.SetDefault("GenesisFile", rootDir+"/genesis.json")
	App.SetDefault("AddrbookFile", rootDir+"/addrbook.json")
	App.SetDefault("PrivValidatorfile", rootDir+"/priv_valdiator.json")
}

func ParseFlags(args []string) {
	var flags = flag.NewFlagSet("main", flag.ExitOnError)
	var printHelp = false

	// Declare flags
	flags.BoolVar(&printHelp, "help", false, "Print this help message.")
	flags.String("listen_addr", App.GetString("ListenAddr"), "Listen address. (0.0.0.0:0 means any interface, any port)")
	flags.String("seed_node", App.GetString("SeedNode"), "Address of seed node")
	flags.String("rpc_http_listen_addr", App.GetString("RPC.HTTP.ListenAddr"), "RPC listen address. Port required")
	flags.Parse(args)
	if printHelp {
		flags.PrintDefaults()
		os.Exit(0)
	}

	// Merge parsed flag values onto App.
	App.BindPFlag("ListenAddr", flags.Lookup("listen_addr"))
	App.BindPFlag("SeedNode", flags.Lookup("seed_node"))
	App.BindPFlag("RPC.HTTP.ListenAddr", flags.Lookup("rpc_http_listen_addr"))

	// Confused?
	//App.Debug()
}
