package tendermint

import (
	"os"
	"path"
	"strings"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
)

// GetTMRoot returns Tendermint root directory.
func GetTMRoot(rootDir string) string {
	if rootDir != "" {
		return rootDir
	}

	dir := os.Getenv("TMHOME")
	if dir == "" {
		dir = os.Getenv("TMROOT")
	}
	if dir == "" {
		dir = os.Getenv("HOME") + "/.tendermint"
	}
	return dir
}

func initTMRoot(rootDir string) {
	rootDir = GetTMRoot(rootDir)
	EnsureDir(rootDir, 0700)
	EnsureDir(rootDir+"/data", 0700)

	configFilePath := path.Join(rootDir, "config.toml")

	// Write default config file if missing.
	if !FileExists(configFilePath) {
		// Ask user for moniker
		// moniker := cfg.Prompt("Type hostname: ", "anonymous")
		MustWriteFile(configFilePath, []byte(defaultConfig("anonymous")), 0644)
	}
}

func GetConfig(rootDir string) cfg.Config {
	rootDir = GetTMRoot(rootDir)
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
	mapConfig.SetDefault("abci", "socket")
	mapConfig.SetDefault("moniker", "anonymous")
	mapConfig.SetDefault("node_laddr", "tcp://0.0.0.0:46656")
	mapConfig.SetDefault("seeds", "")
	// mapConfig.SetDefault("seeds", "goldenalchemist.chaintest.net:46656")
	mapConfig.SetDefault("fast_sync", true)
	mapConfig.SetDefault("skip_upnp", false)
	mapConfig.SetDefault("addrbook_file", rootDir+"/addrbook.json")
	mapConfig.SetDefault("addrbook_strict", true) // disable to allow connections locally
	mapConfig.SetDefault("pex_reactor", false)    // enable for peer exchange
	mapConfig.SetDefault("priv_validator_file", rootDir+"/priv_validator.json")
	mapConfig.SetDefault("db_backend", "leveldb")
	mapConfig.SetDefault("db_dir", rootDir+"/data")
	mapConfig.SetDefault("log_level", "info")
	mapConfig.SetDefault("rpc_laddr", "tcp://0.0.0.0:46657")
	mapConfig.SetDefault("grpc_laddr", "")
	mapConfig.SetDefault("prof_laddr", "")
	mapConfig.SetDefault("revision_file", rootDir+"/revision")
	mapConfig.SetDefault("cs_wal_file", rootDir+"/data/cs.wal/wal")
	mapConfig.SetDefault("cs_wal_light", false)
	mapConfig.SetDefault("filter_peers", false)

	mapConfig.SetDefault("block_size", 10000)      // max number of txs
	mapConfig.SetDefault("block_part_size", 65536) // part size 64K
	mapConfig.SetDefault("disable_data_hash", false)

	// all timeouts are in ms
	mapConfig.SetDefault("timeout_handshake", 10000)
	mapConfig.SetDefault("timeout_propose", 3000)
	mapConfig.SetDefault("timeout_propose_delta", 500)
	mapConfig.SetDefault("timeout_prevote", 1000)
	mapConfig.SetDefault("timeout_prevote_delta", 500)
	mapConfig.SetDefault("timeout_precommit", 1000)
	mapConfig.SetDefault("timeout_precommit_delta", 500)
	mapConfig.SetDefault("timeout_commit", 1000)

	// make progress asap (no `timeout_commit`) on full precommit votes
	mapConfig.SetDefault("skip_timeout_commit", false)
	mapConfig.SetDefault("mempool_recheck", true)
	mapConfig.SetDefault("mempool_recheck_empty", true)
	mapConfig.SetDefault("mempool_broadcast", true)
	mapConfig.SetDefault("mempool_wal_dir", rootDir+"/data/mempool.wal")

	mapConfig.SetDefault("tx_index", "kv")

	return mapConfig
}

var defaultConfigTmpl = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

proxy_app = "tcp://127.0.0.1:46658"
moniker = "__MONIKER__"
node_laddr = "tcp://0.0.0.0:46656"
seeds = ""
fast_sync = true
db_backend = "leveldb"
log_level = "notice"
rpc_laddr = "tcp://0.0.0.0:46657"
`

func defaultConfig(moniker string) (defaultConfig string) {
	defaultConfig = strings.Replace(defaultConfigTmpl, "__MONIKER__", moniker, -1)
	return
}
