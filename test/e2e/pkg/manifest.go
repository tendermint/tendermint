package e2e

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// Manifest represents a TOML testnet manifest.
type Manifest struct {
	// IPv6 uses IPv6 networking instead of IPv4. Defaults to IPv4.
	IPv6 bool `toml:"ipv6"`

	// InitialHeight specifies the initial block height, set in genesis. Defaults to 1.
	InitialHeight int64 `toml:"initial_height"`

	// InitialState is an initial set of key/value pairs for the application,
	// set in genesis. Defaults to nothing.
	InitialState map[string]string `toml:"initial_state"`

	// Validators is the initial validator set in genesis, given as node names
	// and power:
	//
	// validators = { validator01 = 10; validator02 = 20; validator03 = 30 }
	//
	// Defaults to all nodes that have mode=validator at power 100. Explicitly
	// specifying an empty set will start with no validators in genesis, and
	// the application must return the validator set in InitChain via the
	// setting validator_update.0 (see below).
	Validators *map[string]int64 `toml:"validators"`

	// ValidatorUpdates is a map of heights to validator names and their power,
	// and will be returned by the ABCI application. For example, the following
	// changes the power of validator01 and validator02 at height 1000:
	//
	// [validator_update.1000]
	// validator01 = 20
	// validator02 = 10
	//
	// Specifying height 0 returns the validator update during InitChain. The
	// application returns the validator updates as-is, i.e. removing a
	// validator must be done by returning it with power 0, and any validators
	// not specified are not changed.
	ValidatorUpdates map[string]map[string]int64 `toml:"validator_update"`

	// Nodes specifies the network nodes. At least one node must be given.
	Nodes map[string]*ManifestNode `toml:"node"`

	// KeyType sets the curve that will be used by validators.
	// Options are ed25519 & secp256k1
	KeyType string `toml:"key_type"`

	// Evidence indicates the amount of evidence that will be injected into the
	// testnet via the RPC endpoint of a random node. Default is 0
	Evidence int `toml:"evidence"`

	// ABCIProtocol specifies the protocol used to communicate with the ABCI
	// application: "unix", "tcp", "grpc", or "builtin". Defaults to builtin.
	// builtin will build a complete Tendermint node into the application and
	// launch it instead of launching a separate Tendermint process.
	ABCIProtocol string `toml:"abci_protocol"`

	// Add artificial delays to each of the main ABCI calls to mimic computation time
	// of the application
	PrepareProposalDelay time.Duration `toml:"prepare_proposal_delay"`
	ProcessProposalDelay time.Duration `toml:"process_proposal_delay"`
	CheckTxDelay         time.Duration `toml:"check_tx_delay"`
	// TODO: add vote extension and finalize block delay (@cmwaters)
}

// ManifestNode represents a node in a testnet manifest.
type ManifestNode struct {
	// Mode specifies the type of node: "validator", "full", "light" or "seed".
	// Defaults to "validator". Full nodes do not get a signing key (a dummy key
	// is generated), and seed nodes run in seed mode with the PEX reactor enabled.
	Mode string `toml:"mode"`

	// SyncApp specifies whether this node should use a synchronized application
	// with an unsynchronized local client. By default this is `false`, meaning
	// that the node will run an unsynchronized application with a synchronized
	// local client.
	//
	// Only applies to validators and full nodes where their ABCI protocol is
	// "builtin".
	SyncApp bool `toml:"sync_app"`

	// Seeds is the list of node names to use as P2P seed nodes. Defaults to none.
	Seeds []string `toml:"seeds"`

	// PersistentPeers is a list of node names to maintain persistent P2P
	// connections to. If neither seeds nor persistent peers are specified,
	// this defaults to all other nodes in the network. For light clients,
	// this relates to the providers the light client is connected to.
	PersistentPeers []string `toml:"persistent_peers"`

	// Database specifies the database backend: "goleveldb", "cleveldb",
	// "rocksdb", "boltdb", or "badgerdb". Defaults to goleveldb.
	Database string `toml:"database"`

	// PrivvalProtocol specifies the protocol used to sign consensus messages:
	// "file", "unix", or "tcp". Defaults to "file". For unix and tcp, the ABCI
	// application will launch a remote signer client in a separate goroutine.
	// Only nodes with mode=validator will actually make use of this.
	PrivvalProtocol string `toml:"privval_protocol"`

	// StartAt specifies the block height at which the node will be started. The
	// runner will wait for the network to reach at least this block height.
	StartAt int64 `toml:"start_at"`

	// BlockSync specifies the block sync mode: "" (disable), "v0" or "v2".
	// Defaults to disabled.
	BlockSync string `toml:"block_sync"`

	// Mempool specifies which version of mempool to use. Either "v0" or "v1"
	// This defaults to v0.
	Mempool string `toml:"mempool_version"`

	// StateSync enables state sync. The runner automatically configures trusted
	// block hashes and RPC servers. At least one node in the network must have
	// SnapshotInterval set to non-zero, and the state syncing node must have
	// StartAt set to an appropriate height where a snapshot is available.
	StateSync bool `toml:"state_sync"`

	// PersistInterval specifies the height interval at which the application
	// will persist state to disk. Defaults to 1 (every height), setting this to
	// 0 disables state persistence.
	PersistInterval *uint64 `toml:"persist_interval"`

	// SnapshotInterval specifies the height interval at which the application
	// will take state sync snapshots. Defaults to 0 (disabled).
	SnapshotInterval uint64 `toml:"snapshot_interval"`

	// RetainBlocks specifies the number of recent blocks to retain. Defaults to
	// 0, which retains all blocks. Must be greater that PersistInterval and
	// SnapshotInterval.
	RetainBlocks uint64 `toml:"retain_blocks"`

	// Perturb lists perturbations to apply to the node after it has been
	// started and synced with the network:
	//
	// disconnect: temporarily disconnects the node from the network
	// kill:       kills the node with SIGKILL then restarts it
	// pause:      temporarily pauses (freezes) the node
	// restart:    restarts the node, shutting it down with SIGTERM
	Perturb []string `toml:"perturb"`
}

// Save saves the testnet manifest to a file.
func (m Manifest) Save(file string) error {
	f, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("failed to create manifest file %q: %w", file, err)
	}
	return toml.NewEncoder(f).Encode(m)
}

// LoadManifest loads a testnet manifest from a file.
func LoadManifest(file string) (Manifest, error) {
	manifest := Manifest{}
	_, err := toml.DecodeFile(file, &manifest)
	if err != nil {
		return manifest, fmt.Errorf("failed to load testnet manifest %q: %w", file, err)
	}
	return manifest, nil
}
