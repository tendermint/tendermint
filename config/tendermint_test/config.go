// Import this in all *_test.go files to initialize ~/.tendermint_test.

package tendermint_test

import (
	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/naoina/toml"
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
	privValFilePath := path.Join(rootDir, "priv_validator.json")

	// Write default config file if missing.
	if !FileExists(configFilePath) {
		// Ask user for moniker
		moniker := cfg.Prompt("Type hostname: ", "anonymous")
		MustWriteFile(configFilePath, []byte(defaultConfig(moniker)))
	}
	if !FileExists(genesisFilePath) {
		MustWriteFile(genesisFilePath, []byte(defaultGenesis))
	}
	if !FileExists(privValFilePath) {
		MustWriteFile(privValFilePath, []byte(privValFilePath))
	}
}

func GetConfig(rootDir string) cfg.Config {
	rootDir = getTMRoot(rootDir)
	initTMRoot(rootDir)

	var mapConfig = cfg.MapConfig(make(map[string]interface{}))
	configFilePath := path.Join(rootDir, "config.toml")
	configFileBytes := MustReadFile(configFilePath)
	err := toml.Unmarshal(configFileBytes, mapConfig)
	if err != nil {
		Exit(Fmt("Could not read config: %v", err))
	}

	// Set defaults or panic
	if mapConfig.IsSet("chain_id") {
		Exit("Cannot set 'chain_id' via config.toml")
	}
	if mapConfig.IsSet("version") {
		Exit("Cannot set 'version' via config.toml")
	}
	mapConfig.SetDefault("chain_id", "tendermint_test")
	mapConfig.SetDefault("version", "0.5.0")
	mapConfig.SetDefault("genesis_file", rootDir+"/genesis.json")
	mapConfig.SetDefault("moniker", "anonymous")
	mapConfig.SetDefault("node_laddr", "0.0.0.0:36656")
	mapConfig.SetDefault("fast_sync", false)
	mapConfig.SetDefault("addrbook_file", rootDir+"/addrbook.json")
	mapConfig.SetDefault("priv_validator_file", rootDir+"/priv_validator.json")
	mapConfig.SetDefault("db_backend", "memdb")
	mapConfig.SetDefault("db_dir", rootDir+"/data")
	mapConfig.SetDefault("log_level", "debug")
	mapConfig.SetDefault("rpc_laddr", "0.0.0.0:36657")
	mapConfig.SetDefault("revisions_file", rootDir+"/revisions")
	return mapConfig
}

func ensureDefault(mapConfig cfg.MapConfig, key string, value interface{}) {
	if !mapConfig.IsSet(key) {
		mapConfig[key] = value
	}
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
	    "address": "C3C1AF26C0CB2C1DB233D8936AD2C6335AAB6844",
	    "amount": 200000000
    },
    {
	    "address": "C76F0E490A003FDB4A94B310C354F1650A6F97B7",
	    "amount": 200000000
    },
    {
	    "address": "576C84059355CD3B8CBDD81C3FCBC5CE5B6632E0",
	    "amount": 200000000
    },
    {
	    "address": "CD9AB051EDEA88E61ABDF2A1ACF10C3803F0972F",
	    "amount": 200000000
    },
    {
	    "address": "4EE2D93B0A1FBA4E9EBE20E088AA122002A2EB0C",
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
