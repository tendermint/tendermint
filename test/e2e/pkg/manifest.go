package e2e

import (
	"fmt"
	"os"
	"sort"

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

	// LogLevel sets the log level of the entire testnet. This can be overridden
	// by individual nodes.
	LogLevel string `toml:"log_level"`

	// QueueType describes the type of queue that the system uses internally
	QueueType string `toml:"queue_type"`

	// Number of bytes per tx. Default is 1kb (1024)
	TxSize int `toml:"tx_size"`

	// VoteExtensionsEnableHeight configures the first height during which
	// the chain will use and require vote extension data to be present
	// in precommit messages.
	VoteExtensionsEnableHeight int64 `toml:"vote_extensions_enable_height"`

	// ABCIProtocol specifies the protocol used to communicate with the ABCI
	// application: "unix", "tcp", "grpc", or "builtin". Defaults to builtin.
	// builtin will build a complete Tendermint node into the application and
	// launch it instead of launching a separate Tendermint process.
	ABCIProtocol string `toml:"abci_protocol"`

	// Add artificial delays to each of the main ABCI calls to mimic computation time
	// of the application
	PrepareProposalDelayMS uint64 `toml:"prepare_proposal_delay_ms"`
	ProcessProposalDelayMS uint64 `toml:"process_proposal_delay_ms"`
	CheckTxDelayMS         uint64 `toml:"check_tx_delay_ms"`
	VoteExtensionDelayMS   uint64 `toml:"vote_extension_delay_ms"`
	FinalizeBlockDelayMS   uint64 `toml:"finalize_block_delay_ms"`
}

// ManifestNode represents a node in a testnet manifest.
type ManifestNode struct {
	// Mode specifies the type of node: "validator", "full", "light" or "seed".
	// Defaults to "validator". Full nodes do not get a signing key (a dummy key
	// is generated), and seed nodes run in seed mode with the PEX reactor enabled.
	Mode string `toml:"mode"`

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
	// "file", "unix", "tcp", or "grpc". Defaults to "file". For tcp and unix, the ABCI
	// application will launch a remote signer client in a separate goroutine.
	// For grpc the ABCI application will launch a remote signer server.
	// Only nodes with mode=validator will actually make use of this.
	PrivvalProtocol string `toml:"privval_protocol"`

	// StartAt specifies the block height at which the node will be started. The
	// runner will wait for the network to reach at least this block height.
	StartAt int64 `toml:"start_at"`

	// Mempool specifies which version of mempool to use. Either "v0" or "v1"
	Mempool string `toml:"mempool_version"`

	// StateSync enables state sync. The runner automatically configures trusted
	// block hashes and RPC servers. At least one node in the network must have
	// SnapshotInterval set to non-zero, and the state syncing node must have
	// StartAt set to an appropriate height where a snapshot is available.
	// StateSync can either be "p2p" or "rpc" or an empty string to disable
	StateSync string `toml:"state_sync"`

	// PersistInterval specifies the height interval at which the application
	// will persist state to disk. Defaults to 1 (every height), setting this to
	// 0 disables state persistence.
	PersistInterval *uint64 `toml:"persist_interval"`

	// SnapshotInterval specifies the height interval at which the application
	// will take state sync snapshots. Defaults to 0 (disabled).
	SnapshotInterval uint64 `toml:"snapshot_interval"`

	// RetainBlocks specifies the number of recent blocks to retain. Defaults to
	// 0, which retains all blocks. Must be greater that PersistInterval,
	// SnapshotInterval and EvidenceAgeHeight.
	RetainBlocks uint64 `toml:"retain_blocks"`

	// Perturb lists perturbations to apply to the node after it has been
	// started and synced with the network:
	//
	// disconnect: temporarily disconnects the node from the network
	// kill:       kills the node with SIGKILL then restarts it
	// pause:      temporarily pauses (freezes) the node
	// restart:    restarts the node, shutting it down with SIGTERM
	Perturb []string `toml:"perturb"`

	// Log level sets the log level of the specific node i.e. "info".
	// This is helpful when debugging a specific problem. This overrides the network
	// level.
	LogLevel string `toml:"log_level"`
}

// Stateless reports whether m is a node that does not own state, including light and seed nodes.
func (m ManifestNode) Stateless() bool {
	return m.Mode == string(ModeLight) || m.Mode == string(ModeSeed)
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

// SortManifests orders (in-place) a list of manifests such that the
// manifests will be ordered in terms of complexity (or expected
// runtime). Complexity is determined first by the number of nodes,
// and then by the total number of perturbations in the network.
//
// If reverse is true, then the manifests are ordered with the most
// complex networks before the less complex networks.
func SortManifests(manifests []Manifest, reverse bool) {
	sort.SliceStable(manifests, func(i, j int) bool {
		// sort based on a point-based comparison between two
		// manifests.
		var (
			left  = manifests[i]
			right = manifests[j]
		)

		// scores start with 100 points for each node. The
		// number of nodes in a network is the most important
		// factor in the complexity of the test.
		leftScore := len(left.Nodes) * 100
		rightScore := len(right.Nodes) * 100

		// add two points for every node perturbation, and one
		// point for every node that starts after genesis.
		for _, n := range left.Nodes {
			leftScore += (len(n.Perturb) * 2)

			if n.StartAt > 0 {
				leftScore += 3
			}
		}
		for _, n := range right.Nodes {
			rightScore += (len(n.Perturb) * 2)
			if n.StartAt > 0 {
				rightScore += 3
			}
		}

		// add one point if the network has evidence.
		if left.Evidence > 0 {
			leftScore += 2
		}

		if right.Evidence > 0 {
			rightScore += 2
		}

		if left.TxSize > right.TxSize {
			leftScore++
		}

		if right.TxSize > left.TxSize {
			rightScore++
		}

		if reverse {
			return leftScore >= rightScore
		}

		return leftScore < rightScore
	})
}

// SplitGroups divides a list of manifests into n groups of
// manifests.
func SplitGroups(groups int, manifests []Manifest) [][]Manifest {
	groupSize := (len(manifests) + groups - 1) / groups
	splitManifests := make([][]Manifest, 0, groups)

	for i := 0; i < len(manifests); i += groupSize {
		grp := make([]Manifest, groupSize)
		n := copy(grp, manifests[i:])
		splitManifests = append(splitManifests, grp[:n])
	}

	return splitManifests
}

// WriteManifests writes a collection of manifests into files with the
// specified path prefix.
func WriteManifests(prefix string, manifests []Manifest) error {
	for i, manifest := range manifests {
		if err := manifest.Save(fmt.Sprintf("%s-%04d.toml", prefix, i)); err != nil {
			return err
		}
	}

	return nil
}
