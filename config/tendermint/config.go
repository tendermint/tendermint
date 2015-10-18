package tendermint

import (
	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/naoina/toml"
	"os"
	"path"
	"strings"

	. "github.com/tendermint/tendermint/common"
	cfg "github.com/tendermint/tendermint/config"
)

func getTMRoot(rootDir string) string {
	if rootDir == "" {
		rootDir = os.Getenv("TMROOT")
	}
	if rootDir == "" {
		rootDir = os.Getenv("HOME") + "/.tendermint"
	}
	return rootDir
}

func initTMRoot(rootDir string) {
	rootDir = getTMRoot(rootDir)
	EnsureDir(rootDir)

	configFilePath := path.Join(rootDir, "config.toml")

	// Write default config file if missing.
	if !FileExists(configFilePath) {
		// Ask user for moniker
		// moniker := cfg.Prompt("Type hostname: ", "anonymous")
		MustWriteFile(configFilePath, []byte(defaultConfig("anonymous")))
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
	if mapConfig.IsSet("revision_file") {
		Exit("Cannot set 'revision_file' via config.toml. It must match what's in the Makefile")
	}
	mapConfig.SetDefault("chain_id", "tendermint_testnet_11.c") // TODO ALSO UPDATE GENESIS BELOW!!!
	mapConfig.SetDefault("genesis_file", rootDir+"/genesis.json")
	mapConfig.SetDefault("moniker", "anonymous")
	mapConfig.SetDefault("node_laddr", "0.0.0.0:46656")
	// mapConfig.SetDefault("seeds", "goldenalchemist.chaintest.net:46656")
	mapConfig.SetDefault("fast_sync", true)
	mapConfig.SetDefault("skip_upnp", false)
	mapConfig.SetDefault("addrbook_file", rootDir+"/addrbook.json")
	mapConfig.SetDefault("priv_validator_file", rootDir+"/priv_validator.json")
	mapConfig.SetDefault("db_backend", "leveldb")
	mapConfig.SetDefault("db_dir", rootDir+"/data")
	mapConfig.SetDefault("vm_log", true)
	mapConfig.SetDefault("log_level", "info")
	mapConfig.SetDefault("rpc_laddr", "0.0.0.0:46657")
	mapConfig.SetDefault("prof_laddr", "")
	mapConfig.SetDefault("revision_file", rootDir+"/revision")
	mapConfig.SetDefault("local_routing", false)
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
node_laddr = "0.0.0.0:46656"
seeds = ""
fast_sync = true
db_backend = "leveldb"
log_level = "notice"
rpc_laddr = "0.0.0.0:46657"
`

func defaultConfig(moniker string) (defaultConfig string) {
	defaultConfig = strings.Replace(defaultConfigTmpl, "__MONIKER__", moniker, -1)
	return
}
