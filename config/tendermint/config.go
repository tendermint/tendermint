package tendermint

import (
	"os"
	"path"
	"strings"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
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
	EnsureDir(rootDir, 0700)

	configFilePath := path.Join(rootDir, "config.toml")

	// Write default config file if missing.
	if !FileExists(configFilePath) {
		// Ask user for moniker
		// moniker := cfg.Prompt("Type hostname: ", "anonymous")
		MustWriteFile(configFilePath, []byte(defaultConfig("anonymous")), 0644)
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
	if mapConfig.IsSet("revision_file") {
		Exit("Cannot set 'revision_file' via config.toml. It must match what's in the Makefile")
	}
	mapConfig.SetRequired("chain_id") // blows up if you try to use it before setting.
	mapConfig.SetDefault("genesis_file", rootDir+"/genesis.json")
	mapConfig.SetDefault("proxy_app", "tcp://127.0.0.1:46658")
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
	mapConfig.SetDefault("cswal", rootDir+"/cswal")
	return mapConfig
}

var defaultConfigTmpl = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

proxy_app = "tcp://127.0.0.1:46658"
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
