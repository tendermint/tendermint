package config

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	cmn "github.com/tendermint/tmlibs/common"
)

/****** these are for production settings ***********/

func EnsureRoot(rootDir string) {
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

var defaultConfigTmpl = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

proxy_app = "tcp://127.0.0.1:46658"
moniker = "__MONIKER__"
fast_sync = true
db_backend = "leveldb"
log_level = "state:info,*:error"

[rpc]
laddr = "tcp://0.0.0.0:46657"

[p2p]
laddr = "tcp://0.0.0.0:46656"
seeds = ""
`

func defaultConfig(moniker string) string {
	return strings.Replace(defaultConfigTmpl, "__MONIKER__", moniker, -1)
}

/****** these are for test settings ***********/

func ResetTestRoot(testName string) *Config {
	rootDir := os.ExpandEnv("$HOME/.tendermint_test")
	rootDir = filepath.Join(rootDir, testName)
	// Remove ~/.tendermint_test_bak
	if cmn.FileExists(rootDir + "_bak") {
		err := os.RemoveAll(rootDir + "_bak")
		if err != nil {
			cmn.PanicSanity(err.Error())
		}
	}
	// Move ~/.tendermint_test to ~/.tendermint_test_bak
	if cmn.FileExists(rootDir) {
		err := os.Rename(rootDir, rootDir+"_bak")
		if err != nil {
			cmn.PanicSanity(err.Error())
		}
	}
	// Create new dir
	cmn.EnsureDir(rootDir, 0700)
	cmn.EnsureDir(rootDir+"/data", 0700)

	configFilePath := path.Join(rootDir, "config.toml")
	genesisFilePath := path.Join(rootDir, "genesis.json")
	privFilePath := path.Join(rootDir, "priv_validator.json")

	// Write default config file if missing.
	if !cmn.FileExists(configFilePath) {
		// Ask user for moniker
		cmn.MustWriteFile(configFilePath, []byte(testConfig("anonymous")), 0644)
	}
	if !cmn.FileExists(genesisFilePath) {
		cmn.MustWriteFile(genesisFilePath, []byte(testGenesis), 0644)
	}
	// we always overwrite the priv val
	cmn.MustWriteFile(privFilePath, []byte(testPrivValidator), 0644)

	config := TestConfig().SetRoot(rootDir)
	return config
}

var testConfigTmpl = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

proxy_app = "dummy"
moniker = "__MONIKER__"
fast_sync = false
db_backend = "memdb"
log_level = "info"

[rpc]
laddr = "tcp://0.0.0.0:36657"

[p2p]
laddr = "tcp://0.0.0.0:36656"
seeds = ""
`

func testConfig(moniker string) (testConfig string) {
	testConfig = strings.Replace(testConfigTmpl, "__MONIKER__", moniker, -1)
	return
}

var testGenesis = `{
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

var testPrivValidator = `{
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
