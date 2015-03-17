package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	flag "github.com/spf13/pflag"
	"github.com/tendermint/confer"
)

var app *confer.Config
var appMtx sync.Mutex

func App() *confer.Config {
	appMtx.Lock()
	defer appMtx.Unlock()
	if app == nil {
		Init("")
	}
	return app
}

// NOTE: If you change this, maybe also change initDefaults()
var defaultConfig = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

Network =         "tendermint_testnet0"
ListenAddr =      "0.0.0.0:8080"
# First node to connect to.  Command-line overridable.
SeedNode =        "23.239.22.253:8080"

[DB]
# The only other available backend is "memdb"
Backend =         "leveldb"
# Dir =             "~/.tendermint/data"

[Log.Stdout]
Level =           "info"

[Log.File]
Level =           "debug"
# Dir =             "~/.tendermint/log"

[RPC.HTTP]
# For the RPC API HTTP server.  Port required.
ListenAddr = "127.0.0.1:8081"

[Alert]
# TODO: Document options

[SMTP]
# TODO: Document options
`

var defaultGenesis = `
{
  "Accounts": [
    {
      "Address": "553722287BF1230C081C270908C1F453E7D1C397",
      "Amount":  200000000
    },
    {
      "Address": "AC89A6DDF4C309A89A2C4078CE409A5A7B282270",
      "Amount":  200000000
    }
  ],
  "Validators": [
    {
      "PubKey": [1, "932A857D334BA5A38DD8E0D9CDE9C84687C21D0E5BEE64A1EDAB9C6C32344F1A"],
      "Amount": 100000000,
      "UnbondTo": [
        {
          "Address": "553722287BF1230C081C270908C1F453E7D1C397",
          "Amount":  100000000
        }
      ]
    }
  ]
}
`

// NOTE: If you change this, maybe also change defaultConfig
func initDefaults(rootDir string) {
	app.SetDefault("Network", "tendermint_testnet0")
	app.SetDefault("ListenAddr", "0.0.0.0:8080")
	app.SetDefault("DB.Backend", "leveldb")
	app.SetDefault("DB.Dir", rootDir+"/data")
	app.SetDefault("Log.Stdout.Level", "info")
	app.SetDefault("Log.File.Dir", rootDir+"/log")
	app.SetDefault("Log.File.Level", "debug")
	app.SetDefault("RPC.HTTP.ListenAddr", "0.0.0.0:8081")

	app.SetDefault("GenesisFile", rootDir+"/genesis.json")
	app.SetDefault("AddrBookFile", rootDir+"/addrbook.json")
	app.SetDefault("PrivValidatorfile", rootDir+"/priv_validator.json")
}

func Init(rootDir string) {

	// Get RootDir
	if rootDir == "" {
		rootDir = os.Getenv("TMROOT")
	}
	if rootDir == "" {
		rootDir = os.Getenv("HOME") + "/.tendermint"
	}
	configFile := path.Join(rootDir, "config.toml")
	genesisFile := path.Join(rootDir, "genesis.json")

	// Write default config file if missing.
	checkWriteFile(configFile, defaultConfig)
	checkWriteFile(genesisFile, defaultGenesis)

	// Initialize Config
	app = confer.NewConfig()
	initDefaults(rootDir)
	paths := []string{configFile}
	if err := app.ReadPaths(paths...); err != nil {
		log.Warn("Error reading configuration", "paths", paths, "error", err)
	}

	// Confused?
	// app.Debug()
}

// Check if a file exists; if not, ensure the directory is made and write the file
func checkWriteFile(configFile, contents string) {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if strings.Index(configFile, "/") != -1 {
			err := os.MkdirAll(filepath.Dir(configFile), 0700)
			if err != nil {
				fmt.Printf("Could not create directory: %v", err)
				os.Exit(1)
			}
		}
		err := ioutil.WriteFile(configFile, []byte(contents), 0600)
		if err != nil {
			fmt.Printf("Could not write config file: %v", err)
			os.Exit(1)
		}
		fmt.Printf("Config file written to %v.\n", configFile)
	}
}

func ParseFlags(args []string) {
	var flags = flag.NewFlagSet("main", flag.ExitOnError)
	var printHelp = false

	// Declare flags
	flags.BoolVar(&printHelp, "help", false, "Print this help message.")
	flags.String("listen_addr", app.GetString("ListenAddr"), "Listen address. (0.0.0.0:0 means any interface, any port)")
	flags.String("seed_node", app.GetString("SeedNode"), "Address of seed node")
	flags.String("rpc_http_listen_addr", app.GetString("RPC.HTTP.ListenAddr"), "RPC listen address. Port required")
	flags.Parse(args)
	if printHelp {
		flags.PrintDefaults()
		os.Exit(0)
	}

	// Merge parsed flag values onto app.
	app.BindPFlag("ListenAddr", flags.Lookup("listen_addr"))
	app.BindPFlag("SeedNode", flags.Lookup("seed_node"))
	app.BindPFlag("RPC.HTTP.ListenAddr", flags.Lookup("rpc_http_listen_addr"))

	// Confused?
	//app.Debug()
}
