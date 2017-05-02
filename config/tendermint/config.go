package tendermint

import (
	"os"
	"path"
	"strings"

	"github.com/spf13/viper"
	cmn "github.com/tendermint/tmlibs/common"
)

func getTMRoot(rootDir string) string {
	if rootDir == "" {
		rootDir = os.Getenv("TMHOME")
	}
	if rootDir == "" {
		// deprecated, use TMHOME (TODO: remove in TM 0.11.0)
		rootDir = os.Getenv("TMROOT")
	}
	if rootDir == "" {
		rootDir = os.Getenv("HOME") + "/.tendermint"
	}
	return rootDir
}

func initTMRoot(rootDir string) {
	rootDir = getTMRoot(rootDir)
	cmn.EnsureDir(rootDir, 0700)
	cmn.EnsureDir(rootDir+"/data", 0700)

	configFilePath := path.Join(rootDir, "config.toml")

	// Write default config file if missing.
	if !cmn.FileExists(configFilePath) {
		// Ask user for moniker
		// moniker := cfg.Prompt("Type hostname: ", "anonymous")
		cmn.MustWriteFile(configFilePath, []byte(defaultConfig("anonymous")), 0644)
	}
}

func GetConfig(rootDir string) *viper.Viper {
	rootDir = getTMRoot(rootDir)
	initTMRoot(rootDir)

	config := viper.New()
	config.SetConfigName("config")
	config.SetConfigType("toml")
	config.AddConfigPath(rootDir)
	err := config.ReadInConfig()
	if err != nil {
		cmn.Exit(cmn.Fmt("Could not read config from directory %v: %v", rootDir, err))
	}
	//config.WatchConfig()

	// Set defaults or panic
	if config.IsSet("chain.chain_id") {
		cmn.Exit("Cannot set 'chain_id' via config.toml")
	}
	//mapConfig.SetRequired("chain_id") // blows up if you try to use it before setting.
	config.SetDefault("moniker", "anonymous")
	config.SetDefault("log_level", "info")
	config.SetDefault("prof_laddr", "")
	config.SetDefault("genesis_file", rootDir+"/genesis.json")
	config.SetDefault("proxy_app", "tcp://127.0.0.1:46658")
	config.SetDefault("abci", "socket")
	config.SetDefault("filter_peers", false)
	config.SetDefault("fast_sync", true)
	config.SetDefault("priv_validator_file", rootDir+"/priv_validator.json")
	config.SetDefault("db_backend", "leveldb")
	config.SetDefault("db_dir", rootDir+"/data")
	config.SetDefault("rpc_laddr", "tcp://0.0.0.0:46657")
	config.SetDefault("grpc_laddr", "")
	config.SetDefault("tx_index", "kv")

	config.SetDefault("network.listen_addr", "tcp://0.0.0.0:46656")
	config.SetDefault("network.seeds", "")
	config.SetDefault("network.skip_upnp", false)
	config.SetDefault("network.addrbook_file", rootDir+"/addrbook.json")
	config.SetDefault("network.addrbook_strict", true) // disable to allow connections locally
	config.SetDefault("network.pex_reactor", false)    // enable for peer exchange

	config.SetDefault("consensus.wal_file", rootDir+"/data/cs.wal/wal")
	config.SetDefault("consensus.wal_light", false)
	config.SetDefault("consensus.max_block_size_txs", 10000) // max number of txs
	config.SetDefault("consensus.block_part_size", 65536)    // part size 64K
	// all timeouts are in ms
	config.SetDefault("consensus.timeout_handshake", 10000)
	config.SetDefault("consensus.timeout_propose", 3000)
	config.SetDefault("consensus.timeout_propose_delta", 500)
	config.SetDefault("consensus.timeout_prevote", 1000)
	config.SetDefault("consensus.timeout_prevote_delta", 500)
	config.SetDefault("consensus.timeout_precommit", 1000)
	config.SetDefault("consensus.timeout_precommit_delta", 500)
	config.SetDefault("consensus.timeout_commit", 1000)
	// make progress asap (no `timeout_commit`) on full precommit votes
	config.SetDefault("consensus.skip_timeout_commit", false)

	config.SetDefault("mempool.recheck", true)
	config.SetDefault("mempool.recheck_empty", true)
	config.SetDefault("mempool.broadcast", true)
	config.SetDefault("mempool.wal_dir", rootDir+"/data/mempool.wal")

	return config
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
