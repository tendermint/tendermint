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
	mapConfig.SetDefault("chain_id", "tendermint_testnet_10")
	// Major: alpha
	// Minor: encrypted p2p!
	// Revision: ripemd for NewContractAddress
	mapConfig.SetDefault("version", "0.5.1")
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
	mapConfig.SetDefault("log_level", "info")
	mapConfig.SetDefault("rpc_laddr", "0.0.0.0:46657")
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
node_laddr = "0.0.0.0:46656"
seeds = "goldenalchemist.chaintest.net:46656"
fast_sync = true
db_backend = "leveldb"
log_level = "debug"
rpc_laddr = "0.0.0.0:46657"
`

func defaultConfig(moniker string) (defaultConfig string) {
	defaultConfig = strings.Replace(defaultConfigTmpl, "__MONIKER__", moniker, -1)
	return
}

var defaultGenesis = `{
    "chain_id": "tendermint_testnet_11",
    "accounts": [
        {
            "address": "9FCBA7F840A0BFEBBE755E853C9947270A912D04",
            "amount": 2098999998000000
        },
				{
					  "address": "B9FA4AB462B9C6BF6A62DB4AE77C9E7087209A04",
						"amount": 1000000000000
				},
        {
            "address": "A88A61069B6660F30F65E8786AFDD4F1D8F625E9",
						"amount": 1000000
				},
        {
            "address": "EE2EE9247973B4AFC3867CFE5F415410AC251B61",
						"amount": 1000000
				}
    ],
    "validators": [
        {
            "pub_key": [1, "178EC6008A4364508979C70CBF100BD4BCBAA12DDE6251F5F486B4FD09014F06"],
            "amount": 100000000000
        },
        {
            "pub_key": [1, "2A77777CC51467DE42350D4A8F34720D527734189BE64C7A930DD169E1FED3C6"],
            "amount": 100000000000
        },
        {
            "pub_key": [1, "3718E69D09B11B3AD3FA31AEF07EC416D2AEED241CACE7B0F30AE9803FFB0F08"],
            "amount": 100000000000
        },
        {
            "pub_key": [1, "C6B0440DEACD1E4CF1C736CEB8E38E788B700BA2B2045A55CB657A455CF5F889"],
            "amount": 100000000000
        },
        {
            "pub_key": [1, "3BA1190D54F91EFBF8B0125F7EC116AD4BA2894B6EE38564A5D5FD3230D91F7B"],
            "amount": 100000000000
        }
    ]
}`
