// Import this in all *_test.go files to initialize ~/.tendermint_test.

package tendermint_test

import (
	"os"
	"path"
	"strings"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-logger"
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

func ResetConfig(localPath string) cfg.Config {
	rootDir := os.Getenv("HOME") + "/.tendermint_test/" + localPath
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
	mapConfig.SetDefault("proxy_app", "dummy")
	mapConfig.SetDefault("abci", "socket")
	mapConfig.SetDefault("moniker", "anonymous")
	mapConfig.SetDefault("node_laddr", "tcp://0.0.0.0:36656")
	mapConfig.SetDefault("fast_sync", false)
	mapConfig.SetDefault("skip_upnp", true)
	mapConfig.SetDefault("addrbook_file", rootDir+"/addrbook.json")
	mapConfig.SetDefault("addrbook_strict", true) // disable to allow connections locally
	mapConfig.SetDefault("pex_reactor", false)    // enable for peer exchange
	mapConfig.SetDefault("priv_validator_file", rootDir+"/priv_validator.json")
	mapConfig.SetDefault("db_backend", "memdb")
	mapConfig.SetDefault("db_dir", rootDir+"/data")
	mapConfig.SetDefault("log_level", "info")
	mapConfig.SetDefault("rpc_laddr", "tcp://0.0.0.0:36657")
	mapConfig.SetDefault("grpc_laddr", "tcp://0.0.0.0:36658")
	mapConfig.SetDefault("prof_laddr", "")
	mapConfig.SetDefault("revision_file", rootDir+"/revision")
	mapConfig.SetDefault("cs_wal_file", rootDir+"/data/cs.wal/wal")
	mapConfig.SetDefault("cs_wal_light", false)
	mapConfig.SetDefault("filter_peers", false)

	mapConfig.SetDefault("block_size", 10000)
	mapConfig.SetDefault("block_part_size", 65536) // part size 64K
	mapConfig.SetDefault("disable_data_hash", false)
	mapConfig.SetDefault("timeout_propose", 2000)
	mapConfig.SetDefault("timeout_propose_delta", 1)
	mapConfig.SetDefault("timeout_prevote", 10)
	mapConfig.SetDefault("timeout_prevote_delta", 1)
	mapConfig.SetDefault("timeout_precommit", 10)
	mapConfig.SetDefault("timeout_precommit_delta", 1)
	mapConfig.SetDefault("timeout_commit", 10)
	mapConfig.SetDefault("skip_timeout_commit", true)
	mapConfig.SetDefault("mempool_recheck", true)
	mapConfig.SetDefault("mempool_recheck_empty", true)
	mapConfig.SetDefault("mempool_broadcast", true)
	mapConfig.SetDefault("mempool_wal_dir", "")

	logger.SetLogLevel(mapConfig.GetString("log_level"))

	return mapConfig
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
      "pub_key": [
        1,
        "3B3069C422E19688B45CBFAE7BB009FC0FA1B1EA86593519318B7214853803C8"
      ],
      "amount": 10,
      "name": ""
    }
  ],
  "app_hash": ""
}`

var defaultPrivValidator = `{
  "address": "D028C9981F7A87F3093672BF0D5B0E2A1B3ED456",
  "pub_key": [
    1,
    "3B3069C422E19688B45CBFAE7BB009FC0FA1B1EA86593519318B7214853803C8"
  ],
  "priv_key": [
    1,
    "27F82582AEFAE7AB151CFB01C48BB6C1A0DA78F9BDDA979A9F70A84D074EB07D3B3069C422E19688B45CBFAE7BB009FC0FA1B1EA86593519318B7214853803C8"
  ],
  "last_height": 0,
  "last_round": 0,
  "last_step": 0
}`
