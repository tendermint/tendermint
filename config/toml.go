package config

import (
	"bytes"
	"os"
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

// EnsureRoot creates the root, config, and data directories if they don't exist,
// and panics if it fails.
func EnsureRoot(rootDir string) {
	if err := cmn.EnsureDir(rootDir, 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}
	if err := cmn.EnsureDir(filepath.Join(rootDir, defaultConfigDir), 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}
	if err := cmn.EnsureDir(filepath.Join(rootDir, defaultDataDir), 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}

	configFilePath := filepath.Join(rootDir, defaultConfigFilePath)

	// Write default config file if missing.
	if !cmn.FileExists(configFilePath) {
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

# Output level for logging, including package level options
log_level = "{{ .BaseConfig.LogLevel }}"

##### additional base config options #####

# Path to the JSON file containing the initial validator set and other meta data
genesis_file = "{{ .BaseConfig.Genesis }}"

# Path to the JSON file containing the private key to use as a validator in the consensus protocol
priv_validator_file = "{{ .BaseConfig.PrivValidator }}"

# Path to the JSON file containing the private key to use for node authentication in the p2p protocol
node_key_file = "{{ .BaseConfig.NodeKey}}"

# Mechanism to connect to the ABCI application: socket | grpc
abci = "{{ .BaseConfig.ABCI }}"

# TCP or UNIX socket address for the profiling server to listen on
prof_laddr = "{{ .BaseConfig.ProfListenAddress }}"

# If true, query the ABCI app on connecting to a new peer
# so the app can decide if we should keep the connection or not
filter_peers = {{ .BaseConfig.FilterPeers }}

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

# Comma separated list of nodes to keep persistent connections to
# Do not add private peers to this list if you don't want them advertised
persistent_peers = ""

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

# Set true to enable the peer-exchange reactor
pex = {{ .P2P.PexReactor }}

# Seed mode, in which node constantly crawls the network and looks for
# peers. If another node asks it for addresses, it responds and disconnects.
#
# Does not work if the peer-exchange reactor is disabled.
seed_mode = {{ .P2P.SeedMode }}

# Authenticated encryption
auth_enc = {{ .P2P.AuthEnc }}

# Comma separated list of peer IDs to keep private (will not be gossiped to other peers)
private_peer_ids = "{{ .P2P.PrivatePeerIDs }}"

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

##### transactions indexer configuration options #####
[tx_index]

# What indexer to use for transactions
#
# Options:
#   1) "null" (default)
#   2) "kv" - the simplest possible indexer, backed by key-value storage (defaults to levelDB; see DBBackend).
indexer = "{{ .TxIndex.Indexer }}"

# Comma-separated list of tags to index (by default the only tag is tx hash)
#
# It's recommended to index only a subset of tags due to possible memory
# bloat. This is, of course, depends on the indexer's DB and the volume of
# transactions.
index_tags = "{{ .TxIndex.IndexTags }}"

# When set to true, tells indexer to index all tags. Note this may be not
# desirable (see the comment above). IndexTags has a precedence over
# IndexAllTags (i.e. when given both, IndexTags will be indexed).
index_all_tags = {{ .TxIndex.IndexAllTags }}
`

/****** these are for test settings ***********/

func ResetTestRoot(testName string) *Config {
	rootDir := os.ExpandEnv("$HOME/.tendermint_test")
	rootDir = filepath.Join(rootDir, testName)
	// Remove ~/.tendermint_test_bak
	if cmn.FileExists(rootDir + "_bak") {
		if err := os.RemoveAll(rootDir + "_bak"); err != nil {
			cmn.PanicSanity(err.Error())
		}
	}
	// Move ~/.tendermint_test to ~/.tendermint_test_bak
	if cmn.FileExists(rootDir) {
		if err := os.Rename(rootDir, rootDir+"_bak"); err != nil {
			cmn.PanicSanity(err.Error())
		}
	}
	// Create new dir
	if err := cmn.EnsureDir(rootDir, 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}
	if err := cmn.EnsureDir(filepath.Join(rootDir, defaultConfigDir), 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}
	if err := cmn.EnsureDir(filepath.Join(rootDir, defaultDataDir), 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}

	baseConfig := DefaultBaseConfig()
	configFilePath := filepath.Join(rootDir, defaultConfigFilePath)
	genesisFilePath := filepath.Join(rootDir, baseConfig.Genesis)
	privFilePath := filepath.Join(rootDir, baseConfig.PrivValidator)

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
