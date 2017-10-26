package config

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"text/template"

	cmn "github.com/tendermint/tmlibs/common"
)

var configTemplate *template.Template

func init() {
	var err error
	if configTemplate, err = template.New("configFileTemplate").Parse(defaultConfigTemplate); err != nil {
		panic(err)
	}
}

/****** these are for production settings ***********/

func EnsureRoot(rootDir string) {
	cmn.EnsureDir(rootDir, 0700)
	cmn.EnsureDir(rootDir+"/data", 0700)

	configFilePath := path.Join(rootDir, "config.toml")

	if cmn.FileExists(configFilePath) {
		// TODO: Prompt user to replace if config file already exists.
	} else {
		// Ask user for moniker [moniker := cfg.Prompt("Type hostname: ", "anonymous")] (TODO)
		// Then write the default config from template
		writeConfigFile(configFilePath)
	}
}

// XXX: this func should probably be called by cmd/tendermint/commands/init.go
// alongside the writing of the genesis.json and priv_validator.json
func writeConfigFile(configFilePath string) {
	var buffer bytes.Buffer

	if err := configTemplate.Execute(&buffer, DefaultConfig()); err != nil {
		panic(err)
	}

	cmn.MustWriteFile(configFilePath, buffer.Bytes(), 0644)
}

// Note: any changes to the comments/variables/mapstructure
// must be reflected in the appropriate struct in config/config.go
const defaultConfigTemplate = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

##### main base config options #####

# TCP or UNIX socket address of the ABCI application,
# or the name of an ABCI application compiled in with the Tendermint binary
proxy_app = "{{ .BaseConfig.ProxyApp }}"

# A custom human readable name for this node
moniker = "{{ .BaseConfig.Moniker }}"

# If this node is many blocks behind the tip of the chain, FastSync
# allows them to catchup quickly by downloading blocks in parallel
# and verifying their commits
fast_sync = {{ .BaseConfig.FastSync }}

# Database backend: leveldb | memdb
db_backend = "{{ .BaseConfig.DBBackend }}"

# Database directory
db_path = "{{ .BaseConfig.DBPath }}"

# Output level for logging
log_level = "{{ .BaseConfig.LogLevel }}"

##### additional base config options #####

# The ID of the chain to join (should be signed with every transaction and vote)
chain_id = "{{ .BaseConfig.ChainID }}"

# Path to the JSON file containing the initial validator set and other meta data
genesis_file = "{{ .BaseConfig.Genesis }}"

# Path to the JSON file containing the private key to use as a validator in the consensus protocol
priv_validator_file = "{{ .BaseConfig.PrivValidator }}"

# Mechanism to connect to the ABCI application: socket | grpc
abci = "{{ .BaseConfig.ABCI }}"

# TCP or UNIX socket address for the profiling server to listen on
prof_laddr = "{{ .BaseConfig.ProfListenAddress }}"

# If true, query the ABCI app on connecting to a new peer
# so the app can decide if we should keep the connection or not
filter_peers = {{ .BaseConfig.FilterPeers }}

# What indexer to use for transactions
tx_index = "{{ .BaseConfig.TxIndex }}"

##### advanced configuration options #####

##### rpc server configuration options #####
[rpc]

# TCP or UNIX socket address for the RPC server to listen on
laddr = "{{ .RPC.ListenAddress }}"

# TCP or UNIX socket address for the gRPC server to listen on
# NOTE: This server only supports /broadcast_tx_commit
grpc_laddr = "{{ .RPC.GRPCListenAddress }}"

# Activate unsafe RPC commands like /dial_seeds and /unsafe_flush_mempool
unsafe = {{ .RPC.Unsafe }}

##### peer to peer configuration options #####
[p2p]

# Address to listen for incoming connections
laddr = "{{ .P2P.ListenAddress }}"

# Comma separated list of seed nodes to connect to
seeds = ""

# Path to address book
addr_book_file = "{{ .P2P.AddrBook }}"

# Set true for strict address routability rules
addr_book_strict = {{ .P2P.AddrBookStrict }}

# Time to wait before flushing messages out on the connection, in ms
flush_throttle_timeout = {{ .P2P.FlushThrottleTimeout }}

# Maximum number of peers to connect to
max_num_peers = {{ .P2P.MaxNumPeers }}

# Maximum size of a message packet payload, in bytes
max_msg_packet_payload_size = {{ .P2P.MaxMsgPacketPayloadSize }}

# Rate at which packets can be sent, in bytes/second
send_rate = {{ .P2P.SendRate }}

# Rate at which packets can be received, in bytes/second
recv_rate = {{ .P2P.RecvRate }}

##### mempool configuration options #####
[mempool]

recheck = {{ .Mempool.Recheck }}
recheck_empty = {{ .Mempool.RecheckEmpty }}
broadcast = {{ .Mempool.Broadcast }}
wal_dir = "{{ .Mempool.WalPath }}"

##### consensus configuration options #####
[consensus]

wal_file = "{{ .Consensus.WalPath }}"
wal_light = {{ .Consensus.WalLight }}

# All timeouts are in milliseconds
timeout_propose = {{ .Consensus.TimeoutPropose }}
timeout_propose_delta = {{ .Consensus.TimeoutProposeDelta }}
timeout_prevote = {{ .Consensus.TimeoutPrevote }}
timeout_prevote_delta = {{ .Consensus.TimeoutPrevoteDelta }}
timeout_precommit = {{ .Consensus.TimeoutPrecommit }}
timeout_precommit_delta = {{ .Consensus.TimeoutPrecommitDelta }}
timeout_commit = {{ .Consensus.TimeoutCommit }}

# Make progress as soon as we have all the precommits (as if TimeoutCommit = 0)
skip_timeout_commit = {{ .Consensus.SkipTimeoutCommit }}

# BlockSize
max_block_size_txs = {{ .Consensus.MaxBlockSizeTxs }}
max_block_size_bytes = {{ .Consensus.MaxBlockSizeBytes }}

# EmptyBlocks mode and possible interval between empty blocks in seconds
create_empty_blocks = {{ .Consensus.CreateEmptyBlocks }}
create_empty_blocks_interval = {{ .Consensus.CreateEmptyBlocksInterval }}

# Reactor sleep duration parameters are in milliseconds
peer_gossip_sleep_duration = {{ .Consensus.PeerGossipSleepDuration }}
peer_query_maj23_sleep_duration = {{ .Consensus.PeerQueryMaj23SleepDuration }}
`

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
		writeConfigFile(configFilePath)
	}
	if !cmn.FileExists(genesisFilePath) {
		cmn.MustWriteFile(genesisFilePath, []byte(testGenesis), 0644)
	}
	// we always overwrite the priv val
	cmn.MustWriteFile(privFilePath, []byte(testPrivValidator), 0644)

	config := TestConfig().SetRoot(rootDir)
	return config
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
      "power": 10,
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
