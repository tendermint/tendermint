package tendermint

type Config struct {
	Node       NodeConfig       `mapstructure:"node"`
	Chain      ChainConfig      `mapstructure:"chain"`
	ABCI       ABCIConfig       `mapstructure:"abci"`
	Network    NetworkConfig    `mapstructure:"network"`
	Blockchain BlockchainConfig `mapstructure:"blockchain"`
	Consensus  ConsensusConfig  `mapstructure:"consensus"`
	Block      BlockConfig      `mapstructure:"block"`
	Mempool    MempoolConfig    `mapstructure:"mempool"`
	RPC        RPCConfig        `mapstructure:"rpc"`
	DB         DBConfig         `mapstructure:"db"`
}

type NodeConfig struct {
	Moniker           string `mapstructure:"moniker"`             // "anonymous"
	PrivValidatorFile string `mapstructure:"priv_validator_file"` // rootDir+"/priv_validator.json")

	LogLevel       string `mapstructure:"log_level"`  // info
	ProfListenAddr string `mapstructure:"prof_laddr"` // ""
}

type ChainConfig struct {
	ChainID     string `mapstructure:"chain_id"`
	GenesisFile string `mapstructure:"genesis_file"` // rootDir/genesis.json
}

type ABCIConfig struct {
	ProxyApp string `mapstructure:"proxy_app"` //  tcp://0.0.0.0:46658
	Mode     string `mapstructure:"mode"`      // socket

	FilterPeers bool `mapstructure:"filter_peers"` // false
}

type NetworkConfig struct {
	ListenAddr     string `mapstructure:"listen_adddr"` // "tcp://0.0.0.0:46656")
	Seeds          string `mapstructure:"seeds"`        // []string ...
	SkipUPNP       bool   `mapstructure:"skip_upnp"`
	AddrBookFile   string `mapstructure:"addr_book_file"`   // rootDir+"/addrbook.json")
	AddrBookString bool   `mapstructure:"addr_book_string"` // true
	PexReactor     bool   `mapstructure:"pex_reactor"`      // false
}

type BlockchainConfig struct {
	FastSync bool `mapstructure:"fast_sync"` // true
}

type ConsensusConfig struct {
	WalFile  string `mapstructure:"wal_file"`  //rootDir+"/data/cs.wal/wal")
	WalLight bool   `mapstructure:"wal_light"` // false

	// all timeouts are in ms
	TimeoutPropose        int `mapstructure:"timeout_propose"`         // 3000
	TimeoutProposeDelta   int `mapstructure:"timeout_propose_delta"`   // 500
	TimeoutPrevote        int `mapstructure:"timeout_prevote"`         // 1000
	TimeoutPrevoteDelta   int `mapstructure:"timeout_prevote_delta"`   // 500
	TimeoutPrecommit      int `mapstructure:"timeout_precommit"`       // 1000
	TimeoutPrecommitDelta int `mapstructure:"timeout_precommit_delta"` // 500
	TimeoutCommit         int `mapstructure:"timeout_commit"`          // 1000

	// make progress asap (no `timeout_commit`) on full precommit votes
	SkipTimeoutCommit bool `mapstructure:"skip_timeout_commit"` // false
}

type BlockConfig struct {
	MaxTxs          int  `mapstructure:"max_txs"`           // 10000
	PartSize        int  `mapstructure:"part_size"`         // 65536
	DisableDataHash bool `mapstructure:"disable_data_hash"` // false
}

type MempoolConfig struct {
	Recheck      bool   `mapstructure:"recheck"`       // true
	RecheckEmpty bool   `mapstructure:"recheck_empty"` // true
	Broadcast    bool   `mapstructure:"broadcast"`     // true
	WalDir       string `mapstructure:"wal_dir"`       // rootDir+"/data/mempool.wal")
}

type RPCConfig struct {
	RPCListenAddress  string `mapstructure:"rpc_listen_addr"`  // "tcp://0.0.0.0:46657")
	GRPCListenAddress string `mapstructure:"grpc_listen_addr"` // ""
}

type DBConfig struct {
	Backend string `mapstructure:"backend"` // leveldb
	Dir     string `mapstructure:"dir"`     // rootDir/data

	TxIndex string `mapstructure:"tx_index"` // "kv"
}
