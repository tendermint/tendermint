// Import this in all *_test.go files to initialize ~/.tendermint_test.

package tendermint_test

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/spf13/viper"

	cfg "github.com/tendermint/go-config"
	. "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/logger"
)

func init() {
	// Creates ~/.tendermint_test
	EnsureDir(os.Getenv("HOME")+"/.tendermint_test", 0700)
}

func initTMRoot(rootDir string) {
	// Remove ~/.tendermint_test_bak
	if FileExists(rootDir + "_bak") {
		err := os.RemoveAll(rootDir + "_bak")
		if err != nil {
			PanicSanity(err.Error())
		}
	}
	// Move ~/.tendermint_test to ~/.tendermint_test_bak
	if FileExists(rootDir) {
		err := os.Rename(rootDir, rootDir+"_bak")
		if err != nil {
			PanicSanity(err.Error())
		}
	}
	// Create new dir
	EnsureDir(rootDir, 0700)
	EnsureDir(rootDir+"/data", 0700)

	configFilePath := path.Join(rootDir, "config.toml")
	genesisFilePath := path.Join(rootDir, "genesis.json")
	privFilePath := path.Join(rootDir, "priv_validator.json")

	// Write default config file if missing.
	if !FileExists(configFilePath) {
		// Ask user for moniker
		// moniker := cfg.Prompt("Type hostname: ", "anonymous")
		MustWriteFile(configFilePath, []byte(defaultConfig("anonymous")), 0644)
	}
	if !FileExists(genesisFilePath) {
		MustWriteFile(genesisFilePath, []byte(defaultGenesis), 0644)
	}
	// we always overwrite the priv val
	MustWriteFile(privFilePath, []byte(defaultPrivValidator), 0644)
}

func ResetConfig(localPath string) *viper.Viper {
	rootDir := os.Getenv("HOME") + "/.tendermint_test/" + localPath
	initTMRoot(rootDir)

	config := viper.New()
	config.SetConfigName("config")
	config.SetConfigType("toml")
	config.AddConfigPath(rootDir)
	err := config.ReadInConfig()
	if err != nil {
		Exit(Fmt("Could not read config: %v", err))
	}
	config.WatchConfig()

	// Set defaults or panic
	if config.IsSet("chain_id") {
		Exit(fmt.Sprintf("Cannot set 'chain_id' via config.toml:\n %v\n %v\n ", config.Get("chain_id"), rootDir))
	}

	config.SetDefault("chain_id", "tendermint_test")
	config.SetDefault("genesis_file", rootDir+"/genesis.json")
	config.SetDefault("proxy_app", "dummy")
	config.SetDefault("abci", "socket")
	config.SetDefault("moniker", "anonymous")
	config.SetDefault("node_laddr", "tcp://0.0.0.0:36656")
	config.SetDefault("fast_sync", false)
	config.SetDefault("skip_upnp", true)
	config.SetDefault("addrbook_file", rootDir+"/addrbook.json")
	config.SetDefault("addrbook_strict", true) // disable to allow connections locally
	config.SetDefault("pex_reactor", false)    // enable for peer exchange
	config.SetDefault("priv_validator_file", rootDir+"/priv_validator.json")
	config.SetDefault("db_backend", "memdb")
	config.SetDefault("db_dir", rootDir+"/data")
	config.SetDefault("log_level", "info")
	config.SetDefault("rpc_laddr", "tcp://0.0.0.0:36657")
	config.SetDefault("grpc_laddr", "tcp://0.0.0.0:36658")
	config.SetDefault("prof_laddr", "")
	config.SetDefault("revision_file", rootDir+"/revision")
	config.SetDefault("cs_wal_file", rootDir+"/data/cs.wal/wal")
	config.SetDefault("cs_wal_light", false)
	config.SetDefault("filter_peers", false)

	config.SetDefault("block_size", 10000)
	config.SetDefault("block_part_size", 65536) // part size 64K
	config.SetDefault("disable_data_hash", false)
	config.SetDefault("timeout_handshake", 10000)
	config.SetDefault("timeout_propose", 2000)
	config.SetDefault("timeout_propose_delta", 1)
	config.SetDefault("timeout_prevote", 10)
	config.SetDefault("timeout_prevote_delta", 1)
	config.SetDefault("timeout_precommit", 10)
	config.SetDefault("timeout_precommit_delta", 1)
	config.SetDefault("timeout_commit", 10)
	config.SetDefault("skip_timeout_commit", true)
	config.SetDefault("mempool_recheck", true)
	config.SetDefault("mempool_recheck_empty", true)
	config.SetDefault("mempool_broadcast", true)
	config.SetDefault("mempool_wal_dir", "")

	config.SetDefault("tx_index", "kv")

	logger.SetLogLevel(config.GetString("log_level"))

	return config
}

var defaultConfigTmpl = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

proxy_app = "dummy"
moniker = "__MONIKER__"
node_laddr = "tcp://0.0.0.0:36656"
seeds = ""
fast_sync = false
db_backend = "memdb"
log_level = "info"
rpc_laddr = "tcp://0.0.0.0:36657"
`

func defaultConfig(moniker string) (defaultConfig string) {
	defaultConfig = strings.Replace(defaultConfigTmpl, "__MONIKER__", moniker, -1)
	return
}

var defaultGenesis = `{
  "genesis_time": "0001-01-01T00:00:00.000Z",
  "chain_id": "tendermint_test",
  "validators": [
    {
      "pub_key": {
        "type": "ed25519",
        "data":"3B3069C422E19688B45CBFAE7BB009FC0FA1B1EA86593519318B7214853803C8"
      },
      "amount": 10,
      "name": ""
    }
  ],
  "app_hash": ""
}`

var defaultPrivValidator = `{
  "address": "D028C9981F7A87F3093672BF0D5B0E2A1B3ED456",
  "pub_key": {
    "type": "ed25519",
    "data": "3B3069C422E19688B45CBFAE7BB009FC0FA1B1EA86593519318B7214853803C8"
  },
  "priv_key": {
    "type": "ed25519",
    "data": "27F82582AEFAE7AB151CFB01C48BB6C1A0DA78F9BDDA979A9F70A84D074EB07D3B3069C422E19688B45CBFAE7BB009FC0FA1B1EA86593519318B7214853803C8"
  },
  "last_height": 0,
  "last_round": 0,
  "last_step": 0
}`
