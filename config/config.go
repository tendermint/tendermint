package config

// Config struct for a Tendermint node
type Config struct {
	// The ID of the chain to join (should be signed with every transaction and vote)
	ChainID string `mapstructure:"chain_id"`

	// A JSON file containing the initial validator set and other meta data
	GenesisFile string `mapstructure:"genesis_file"`

	// A JSON file containing the private key to use as a validator in the consensus protocol
	PrivValidatorFile string `mapstructure:"priv_validator_file"`

	// A custom human readable name for this node
	Moniker string `mapstructure:"moniker"`

	// TCP or UNIX socket address of the ABCI application,
	// or the name of an ABCI application compiled in with the Tendermint binary
	ProxyApp string `mapstructure:"proxy_app"`

	// Mechanism to connect to the ABCI application: socket | grpc
	ABCI string `mapstructure:"abci"`

	// Output level for logging
	LogLevel string `mapstructure:"log_level"`

	// TCP or UNIX socket address for the profiling server to listen on
	ProfListenAddress string `mapstructure:"prof_laddr"`

	// If this node is many blocks behind the tip of the chain, FastSync
	// allows them to catchup quickly by downloading blocks in parallel
	// and verifying their commits
	FastSync bool `mapstructure:"fast_sync"`

	// If true, query the ABCI app on connecting to a new peer
	// so the app can decide if we should keep the connection or not
	FilterPeers bool `mapstructure:"filter_peers"` // false

	// What indexer to use for transactions
	TxIndex string `mapstructure:"tx_index"`

	// Database backend: leveldb | memdb
	DBBackend string `mapstructure:"db_backend"`

	// Database directory
	DBDir string `mapstructure:"db_dir"`

	// TCP or UNIX socket address for the RPC server to listen on
	RPCListenAddress string `mapstructure:"rpc_laddr"`

	// TCP or UNIX socket address for the gRPC server to listen on
	// NOTE: This server only supports /broadcast_tx_commit
	GRPCListenAddress string `mapstructure:"grpc_laddr"`
}

func NewDefaultConfig(rootDir string) *Config {
	return &Config{
		GenesisFile:       rootDir + "/genesis.json",
		PrivValidatorFile: rootDir + "/priv_validator.json",
		Moniker:           "anonymous",
		ProxyApp:          "tcp://127.0.0.1:46658",
		ABCI:              "socket",
		LogLevel:          "info",
		ProfListenAddress: "",
		FastSync:          true,
		FilterPeers:       false,
		TxIndex:           "kv",
		DBBackend:         "leveldb",
		DBDir:             rootDir + "/data",
		RPCListenAddress:  "tcp://0.0.0.0:46657",
		GRPCListenAddress: "",
	}
}
