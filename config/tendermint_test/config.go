// Import this in all *_test.go files to initialize ~/.tendermint_test.

package tendermint_test

import (
	"os"
	"path"
	"strings"

	. "github.com/tendermint/tendermint/common"
	cfg "github.com/tendermint/tendermint/config"
)

func init() {
	// Creates ~/.tendermint_test/*
	config := GetConfig("")
	cfg.ApplyConfig(config)
}

func getTMRoot(rootDir string) string {
	if rootDir == "" {
		rootDir = os.Getenv("HOME") + "/.tendermint_test"
	}
	return rootDir
}

func initTMRoot(rootDir string) {
	rootDir = getTMRoot(rootDir)
	EnsureDir(rootDir)

	configFilePath := path.Join(rootDir, "config.toml")
	genesisFilePath := path.Join(rootDir, "genesis.json")

	// Write default config file if missing.
	if !FileExists(configFilePath) {
		// Ask user for moniker
		// moniker := cfg.Prompt("Type hostname: ", "anonymous")
		MustWriteFile(configFilePath, []byte(defaultConfig("anonymous")))
	}
	if !FileExists(genesisFilePath) {
		MustWriteFile(genesisFilePath, []byte(defaultGenesis))
	}
}

func GetConfig(rootDir string) cfg.Config {
	rootDir = getTMRoot(rootDir)
	initTMRoot(rootDir)

	configFilePath := path.Join(rootDir, "config.toml")
	mapConfig, err := cfg.ReadMapConfigFromFile(configFilePath)
	if err != nil {
		Exit(Fmt("Could not read config: %v", err))
	}

	// Set defaults or panic
	if mapConfig.IsSet("chain_id") {
		Exit("Cannot set 'chain_id' via config.toml")
	}
	mapConfig.SetDefault("chain_id", "tendermint_test")
	mapConfig.SetDefault("genesis_file", rootDir+"/genesis.json")
	mapConfig.SetDefault("moniker", "anonymous")
	mapConfig.SetDefault("node_laddr", "0.0.0.0:36656")
	mapConfig.SetDefault("fast_sync", false)
	mapConfig.SetDefault("skip_upnp", true)
	mapConfig.SetDefault("addrbook_file", rootDir+"/addrbook.json")
	mapConfig.SetDefault("priv_validator_file", rootDir+"/priv_validator.json")
	mapConfig.SetDefault("db_backend", "memdb")
	mapConfig.SetDefault("db_dir", rootDir+"/data")
	mapConfig.SetDefault("log_level", "debug")
	mapConfig.SetDefault("vm_log", true)
	mapConfig.SetDefault("rpc_laddr", "0.0.0.0:36657")
	mapConfig.SetDefault("prof_laddr", "")
	mapConfig.SetDefault("revision_file", rootDir+"/revision")
	mapConfig.SetDefault("local_routing", false)
	return mapConfig
}

var defaultConfigTmpl = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

moniker = "__MONIKER__"
node_laddr = "0.0.0.0:36656"
seeds = ""
fast_sync = false
db_backend = "memdb"
log_level = "debug"
rpc_laddr = "0.0.0.0:36657"
`

func defaultConfig(moniker string) (defaultConfig string) {
	defaultConfig = strings.Replace(defaultConfigTmpl, "__MONIKER__", moniker, -1)
	return
}

// priv keys generated deterministically eg rpc/tests/helpers.go
var defaultGenesis = `{
  "chain_id" : "tendermint_test",
  "accounts": [
    {
	    "address": "E9B5D87313356465FAE33C406CE2C2979DE60BCB",
	    "amount": 200000000
    },
    {
	    "address": "DFE4AFFA4CEE17CD01CB9E061D77C3ECED29BD88",
	    "amount": 200000000
    },
    {
	    "address": "F60D30722E7B497FA532FB3207C3FB29C31B1992",
	    "amount": 200000000
    },
    {
	    "address": "336CB40A5EB92E496E19B74FDFF2BA017C877FD6",
	    "amount": 200000000
    },
    {
	    "address": "D218F0F439BF0384F6F5EF8D0F8B398D941BD1DC",
	    "amount": 200000000
    }
  ],
  "validators": [
    {
      "pub_key": [1, "583779C3BFA3F6C7E23C7D830A9C3D023A216B55079AD38BFED1207B94A19548"],
      "amount": 1000000,
      "unbond_to": [
        {
          "address": "E9B5D87313356465FAE33C406CE2C2979DE60BCB",
          "amount":  100000
        }
      ]
    }
  ]
}`

var defaultPrivValidator = `{
  "address": "1D7A91CB32F758A02EBB9BE1FB6F8DEE56F90D42",
	"pub_key": [1,"06FBAC4E285285D1D91FCBC7E91C780ADA11516F67462340B3980CE2B94940E8"],
	"priv_key": [1,"C453604BD6480D5538B4C6FD2E3E314B5BCE518D75ADE4DA3DA85AB8ADFD819606FBAC4E285285D1D91FCBC7E91C780ADA11516F67462340B3980CE2B94940E8"],
	"last_height":0,
	"last_round":0,
	"last_step":0
}`
