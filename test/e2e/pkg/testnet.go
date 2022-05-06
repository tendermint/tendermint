//nolint: gosec
package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dashevo/dashd-go/btcjson"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/dash/llmq"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/types"
)

const (
	randomSeed     int64  = 2308084734268
	proxyPortFirst uint32 = 5701
	networkIPv4           = "10.186.73.0/24"
	networkIPv6           = "fd80:b10c::/48"
	networkPortP2P uint16 = 26656
)

type Mode string
type Protocol string
type Perturbation string

const (
	ModeValidator Mode = "validator"
	ModeFull      Mode = "full"
	ModeLight     Mode = "light"
	ModeSeed      Mode = "seed"

	ProtocolBuiltin  Protocol = "builtin"
	ProtocolFile     Protocol = "file"
	ProtocolGRPC     Protocol = "grpc"
	ProtocolTCP      Protocol = "tcp"
	ProtocolUNIX     Protocol = "unix"
	ProtocolDashCore Protocol = "dashcore"

	PerturbationDisconnect Perturbation = "disconnect"
	PerturbationKill       Perturbation = "kill"
	PerturbationPause      Perturbation = "pause"
	PerturbationRestart    Perturbation = "restart"

	EvidenceAgeHeight int64         = 7
	EvidenceAgeTime   time.Duration = 500 * time.Millisecond

	StateSyncP2P      = "p2p"
	StateSyncRPC      = "rpc"
	StateSyncDisabled = ""
)

// Testnet represents a single testnet.

type ValidatorConfig struct {
	abci.ValidatorUpdate
}

type ValidatorsMap map[*Node]ValidatorConfig
type Testnet struct {
	Name             string
	File             string
	Dir              string
	IP               *net.IPNet
	InitialHeight    int64
	InitialState     map[string]string
	Validators       ValidatorsMap
	ValidatorUpdates map[int64]ValidatorsMap
	Nodes            []*Node
	KeyType          string
	Evidence         int
	LogLevel         string
	TxSize           int
	ABCIProtocol     string

	// Tenderdash-specific fields
	GenesisCoreHeight         uint32 // InitialCoreHeight is a core height put into genesis file
	InitAppCoreHeight         uint32 // InitAppCoreHeight returned in InitApp response
	ChainLockUpdates          map[int64]int64
	ThresholdPublicKey        crypto.PubKey
	ThresholdPublicKeyUpdates map[int64]crypto.PubKey
	QuorumType                btcjson.LLMQType
	QuorumHash                crypto.QuorumHash
	QuorumHashUpdates         map[int64]crypto.QuorumHash
}

// Node represents a Tenderdash node in a testnet.
type Node struct {
	Name                 string
	Testnet              *Testnet
	Mode                 Mode
	PrivvalKeys          map[string]crypto.QuorumKeys
	PrivvalUpdateHeights map[string]crypto.QuorumHash
	NodeKey              crypto.PrivKey
	ProTxHash            crypto.ProTxHash
	IP                   net.IP
	ProxyPort            uint32
	StartAt              int64
	Mempool              string
	StateSync            string
	Database             string
	ABCIProtocol         Protocol
	PrivvalProtocol      Protocol
	PersistInterval      uint64
	SnapshotInterval     uint64
	RetainBlocks         uint64
	Seeds                []*Node
	PersistentPeers      []*Node
	Perturbations        []Perturbation
	LogLevel             string
	QueueType            string
	HasStarted           bool
}

// LoadTestnet loads a testnet from a manifest file, using the filename to
// determine the testnet name and directory (from the basename of the file).
// The testnet generation must be deterministic, since it is generated
// separately by the runner and the test cases. For this reason, testnets use a
// random seed to generate e.g. keys.
func LoadTestnet(file string) (*Testnet, error) {
	fmt.Printf("loading manifest\n")
	manifest, err := LoadManifest(file)
	fmt.Printf("loaded manifest\n")
	if err != nil {
		return nil, err
	}
	dir := strings.TrimSuffix(file, filepath.Ext(file))

	// Set up resource generators. These must be deterministic.
	netAddress := networkIPv4
	if manifest.IPv6 {
		netAddress = networkIPv6
	}
	_, ipNet, err := net.ParseCIDR(netAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid IP network address %q: %w", netAddress, err)
	}

	ipGen := newIPGenerator(ipNet)
	keyGen := newKeyGenerator(randomSeed)
	proTxHashGen := newProTxHashGenerator(randomSeed + 1)
	quorumHashGen := newQuorumHashGenerator(randomSeed + 2)
	proxyPortGen := newPortGenerator(proxyPortFirst)

	// Set up nodes, in alphabetical order (IPs and ports get same order).
	nodeNames := []string{}
	for name := range manifest.Nodes {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	validatorCount := countValidators(manifest.Nodes)
	fmt.Printf("validator count is %v\n", validatorCount)

	quorumType := manifest.QuorumType
	if quorumType == 0 {
		quorumType = 100
	}

	ld := llmq.MustGenerate(
		genProTxHashes(proTxHashGen, validatorCount),
		llmq.WithSeed(randomSeed),
	)
	quorumHash := quorumHashGen.generate()

	testnet := &Testnet{
		Name:                      filepath.Base(dir),
		File:                      file,
		Dir:                       dir,
		IP:                        ipGen.Network(),
		InitialHeight:             1,
		GenesisCoreHeight:         1,
		InitAppCoreHeight:         0,
		InitialState:              manifest.InitialState,
		Validators:                ValidatorsMap{},
		ValidatorUpdates:          map[int64]ValidatorsMap{},
		ChainLockUpdates:          map[int64]int64{},
		Nodes:                     []*Node{},
		Evidence:                  manifest.Evidence,
		KeyType:                   bls12381.KeyType,
		LogLevel:                  manifest.LogLevel,
		TxSize:                    manifest.TxSize,
		ABCIProtocol:              manifest.ABCIProtocol,
		ThresholdPublicKey:        ld.ThresholdPubKey,
		ThresholdPublicKeyUpdates: map[int64]crypto.PubKey{},
		QuorumType:                btcjson.LLMQType(quorumType),
		QuorumHash:                quorumHash,
		QuorumHashUpdates:         map[int64]crypto.QuorumHash{},
	}
	if len(manifest.KeyType) != 0 {
		testnet.KeyType = manifest.KeyType
	}
	if testnet.TxSize <= 0 {
		testnet.TxSize = 1024
	}
	if manifest.InitialHeight > 0 {
		testnet.InitialHeight = manifest.InitialHeight
	}
	if testnet.ABCIProtocol == "" {
		testnet.ABCIProtocol = string(ProtocolBuiltin)
	}
	if manifest.GenesisCoreChainLockedHeight > 0 {
		testnet.GenesisCoreHeight = manifest.GenesisCoreChainLockedHeight
	}
	if manifest.InitAppCoreChainLockedHeight > 0 {
		testnet.InitAppCoreHeight = manifest.InitAppCoreChainLockedHeight
	}

	for _, name := range nodeNames {
		fmt.Printf("Creating node: %s\n", name)
		privKey := keyGen.Generate(manifest.KeyType)
		quorumKeys := crypto.QuorumKeys{
			PrivKey:            privKey,
			ThresholdPublicKey: ld.ThresholdPubKey,
		}
		privateKeysMap := make(map[string]crypto.QuorumKeys)
		privateKeysMap[quorumHash.String()] = quorumKeys
		nodeManifest := manifest.Nodes[name]
		node := &Node{
			Name:             name,
			Testnet:          testnet,
			PrivvalKeys:      privateKeysMap,
			NodeKey:          keyGen.Generate("ed25519"),
			IP:               ipGen.Next(),
			ProxyPort:        proxyPortGen.Next(),
			Mode:             ModeValidator,
			Database:         "goleveldb",
			ABCIProtocol:     Protocol(testnet.ABCIProtocol),
			PrivvalProtocol:  ProtocolFile,
			StartAt:          nodeManifest.StartAt,
			Mempool:          nodeManifest.Mempool,
			StateSync:        nodeManifest.StateSync,
			PersistInterval:  1,
			SnapshotInterval: nodeManifest.SnapshotInterval,
			RetainBlocks:     nodeManifest.RetainBlocks,
			Perturbations:    []Perturbation{},
			LogLevel:         manifest.LogLevel,
			QueueType:        manifest.QueueType,
		}
		if node.StartAt == testnet.InitialHeight {
			node.StartAt = 0 // normalize to 0 for initial nodes, since code expects this
		}
		if nodeManifest.Mode != "" {
			node.Mode = Mode(nodeManifest.Mode)
		}
		if node.Mode == ModeLight {
			node.ABCIProtocol = ProtocolBuiltin
		}
		if nodeManifest.Database != "" {
			node.Database = nodeManifest.Database
		}
		if nodeManifest.PrivvalProtocol != "" {
			node.PrivvalProtocol = Protocol(nodeManifest.PrivvalProtocol)
		}
		if nodeManifest.PersistInterval != nil {
			node.PersistInterval = *nodeManifest.PersistInterval
		}
		for _, p := range nodeManifest.Perturb {
			node.Perturbations = append(node.Perturbations, Perturbation(p))
		}
		if nodeManifest.LogLevel != "" {
			node.LogLevel = nodeManifest.LogLevel
		}
		testnet.Nodes = append(testnet.Nodes, node)
	}

	// We do a second pass to set up seeds and persistent peers, which allows graph cycles.
	for _, node := range testnet.Nodes {
		nodeManifest, ok := manifest.Nodes[node.Name]
		if !ok {
			return nil, fmt.Errorf("failed to look up manifest for node %q", node.Name)
		}
		for _, seedName := range nodeManifest.Seeds {
			seed := testnet.LookupNode(seedName)
			if seed == nil {
				return nil, fmt.Errorf("unknown seed %q for node %q", seedName, node.Name)
			}
			node.Seeds = append(node.Seeds, seed)
		}
		for _, peerName := range nodeManifest.PersistentPeers {
			peer := testnet.LookupNode(peerName)
			if peer == nil {
				return nil, fmt.Errorf("unknown persistent peer %q for node %q", peerName, node.Name)
			}
			if peer.Mode == ModeLight {
				return nil, fmt.Errorf("can not have a light client as a persistent peer (for %q)", node.Name)
			}
			node.PersistentPeers = append(node.PersistentPeers, peer)
		}

		// If there are no seeds or persistent peers specified, default to persistent
		// connections to all other full nodes.
		if len(node.PersistentPeers) == 0 && len(node.Seeds) == 0 {
			for _, peer := range testnet.Nodes {
				if peer.Name == node.Name {
					continue
				}
				if peer.Mode == ModeLight {
					continue
				}
				node.PersistentPeers = append(node.PersistentPeers, peer)
			}
		}
	}
	err = updateNodeParams(
		nodesFilter(testnet.Nodes, shouldHaveName(manifest.Validators)),
		initValidator(
			ld.Iter(),
			quorumHash,
			updateProTxHash(),
			updateGenesisValidators(testnet),
		),
	)
	if err != nil {
		return nil, err
	}
	err = updateNodeParams(
		nodesFilter(testnet.Nodes, shouldNotBeValidator()),
		initAnyNode(ld.ThresholdPubKey, quorumHash),
	)
	if err != nil {
		return nil, err
	}

	heights := make([]int, 0, len(manifest.ValidatorUpdates))
	// We need to do validator updates in order, as we use the previous validator set as the basis of current proTxHashes
	for heightStr := range manifest.ValidatorUpdates {
		height, err := strconv.Atoi(heightStr)
		if err != nil {
			return nil, fmt.Errorf("invalid validator update height %q: %w", height, err)
		}
		heights = append(heights, height)
	}

	sort.Ints(heights)

	// Set up validator updates.
	for _, height := range heights {
		heightStr := strconv.FormatInt(int64(height), 10)
		validators := manifest.ValidatorUpdates[heightStr]
		valUpdate := ValidatorsMap{}
		proTxHashes := make([]crypto.ProTxHash, 0, len(validators))
		for name := range validators {
			node := testnet.LookupNode(name)
			if node == nil {
				return nil, fmt.Errorf("unknown validator %q for update at height %v", name, height)
			}
			if node.ProTxHash == nil {
				node.ProTxHash = proTxHashGen.generate()
				fmt.Printf("Set validator (at update) %s proTxHash to %X\n", node.Name, node.ProTxHash)
			}
			proTxHashes = append(proTxHashes, node.ProTxHash)
		}

		ld = llmq.MustGenerate(proTxHashes, llmq.WithSeed(randomSeed+int64(height)))
		quorumHash := quorumHashGen.generate()

		err = updateNodeParams(
			lookupNodesByProTxHash(testnet, ld.ProTxHashes...),
			initValidator(
				ld.Iter(),
				quorumHash,
				updateValidatorUpdate(valUpdate),
				printInitValidatorInfo(height),
			),
			updatePrivvalUpdateHeights(modifyHeight(height), quorumHash),
		)
		if err != nil {
			return nil, err
		}

		err = updateNodeParams(
			nodesFilter(testnet.Nodes, proTxHashShouldNotBeIn(ld.ProTxHashes)),
			initAnyNode(ld.ThresholdPubKey, quorumHash),
			updatePrivvalUpdateHeights(modifyHeight(height), quorumHash),
		)
		if err != nil {
			return nil, err
		}
		if height == 0 {
			testnet.QuorumHash = quorumHash
			testnet.ThresholdPublicKey = ld.ThresholdPubKey
			testnet.Validators = valUpdate
		}
		testnet.ValidatorUpdates[int64(height)] = valUpdate
		testnet.ThresholdPublicKeyUpdates[int64(height)] = ld.ThresholdPubKey
		testnet.QuorumHashUpdates[int64(height)] = quorumHash
	}

	chainLockSetHeights := make([]int, 0, len(manifest.ChainLockUpdates))
	// We need to do validator updates in order, as we use the previous validator set as the basis of current proTxHashes
	for heightStr := range manifest.ChainLockUpdates {
		height, err := strconv.Atoi(heightStr)
		if err != nil {
			return nil, fmt.Errorf("invalid validator update height %q: %w", height, err)
		}
		chainLockSetHeights = append(chainLockSetHeights, height)
	}

	sort.Ints(chainLockSetHeights)

	// Set up validator updates.
	for _, height := range chainLockSetHeights {
		heightStr := strconv.FormatInt(int64(height), 10)
		chainLockHeight := manifest.ChainLockUpdates[heightStr]
		testnet.ChainLockUpdates[int64(height)] = chainLockHeight
		fmt.Printf("Set chainlock at height %d / core height is %d\n", height, chainLockHeight)
	}

	return testnet, testnet.Validate()
}

// Validate validates a testnet.
func (t Testnet) Validate() error {
	if t.Name == "" {
		return errors.New("network has no name")
	}
	if t.IP == nil {
		return errors.New("network has no IP")
	}
	if len(t.Nodes) == 0 {
		return errors.New("network has no nodes")
	}
	switch t.KeyType {
	case "", types.ABCIPubKeyTypeBLS12381:
	default:
		return errors.New("unsupported KeyType")
	}
	for _, node := range t.Nodes {
		if err := node.Validate(t); err != nil {
			return fmt.Errorf("invalid node %q: %w", node.Name, err)
		}
	}
	return nil
}

// Validate validates a node.
func (n Node) Validate(testnet Testnet) error {
	if n.Name == "" {
		return errors.New("node has no name")
	}
	if n.IP == nil {
		return errors.New("node has no IP address")
	}
	if !testnet.IP.Contains(n.IP) {
		return fmt.Errorf("node IP %v is not in testnet network %v", n.IP, testnet.IP)
	}
	if n.ProxyPort > 0 {
		if n.ProxyPort <= 1024 {
			return fmt.Errorf("local port %v must be >1024", n.ProxyPort)
		}
		for _, peer := range testnet.Nodes {
			if peer.Name != n.Name && peer.ProxyPort == n.ProxyPort {
				return fmt.Errorf("peer %q also has local port %v", peer.Name, n.ProxyPort)
			}
		}
	}
	if n.Mode == "validator" {
		if n.ProTxHash == nil {
			return fmt.Errorf("validator %s must have a proTxHash set", n.Name)
		}
		if len(n.ProTxHash) != crypto.ProTxHashSize {
			return fmt.Errorf("validator %s must have a proTxHash of size 32 (%d)", n.Name, len(n.ProTxHash))
		}
	}
	switch n.StateSync {
	case StateSyncDisabled, StateSyncP2P, StateSyncRPC:
	default:
		return fmt.Errorf("invalid state sync setting %q", n.StateSync)
	}
	switch n.Mempool {
	case "", "v0", "v1":
	default:
		return fmt.Errorf("invalid mempool version %q", n.Mempool)
	}
	switch n.QueueType {
	case "", "priority", "fifo":
	default:
		return fmt.Errorf("unsupported p2p queue type: %s", n.QueueType)
	}
	switch n.Database {
	case "goleveldb", "cleveldb", "boltdb", "rocksdb", "badgerdb":
	default:
		return fmt.Errorf("invalid database setting %q", n.Database)
	}
	switch n.ABCIProtocol {
	case ProtocolBuiltin, ProtocolUNIX, ProtocolTCP, ProtocolGRPC:
	default:
		return fmt.Errorf("invalid ABCI protocol setting %q", n.ABCIProtocol)
	}
	if n.Mode == ModeLight && n.ABCIProtocol != ProtocolBuiltin {
		return errors.New("light client must use builtin protocol")
	}
	switch n.PrivvalProtocol {
	case ProtocolFile, ProtocolUNIX, ProtocolTCP, ProtocolGRPC, ProtocolDashCore:
	default:
		return fmt.Errorf("invalid privval protocol setting %q", n.PrivvalProtocol)
	}

	if n.StartAt > 0 && n.StartAt < n.Testnet.InitialHeight {
		return fmt.Errorf("cannot start at height %v lower than initial height %v",
			n.StartAt, n.Testnet.InitialHeight)
	}
	if n.StateSync != StateSyncDisabled && n.StartAt == 0 {
		return errors.New("state synced nodes cannot start at the initial height")
	}
	if n.RetainBlocks != 0 && n.RetainBlocks < uint64(EvidenceAgeHeight) {
		return fmt.Errorf("retain_blocks must be greater or equal to max evidence age (%d)",
			EvidenceAgeHeight)
	}
	if n.PersistInterval == 0 && n.RetainBlocks > 0 {
		return errors.New("persist_interval=0 requires retain_blocks=0")
	}
	if n.PersistInterval > 1 && n.RetainBlocks > 0 && n.RetainBlocks < n.PersistInterval {
		return errors.New("persist_interval must be less than or equal to retain_blocks")
	}
	if n.SnapshotInterval > 0 && n.RetainBlocks > 0 && n.RetainBlocks < n.SnapshotInterval {
		return errors.New("snapshot_interval must be less than er equal to retain_blocks")
	}

	for _, perturbation := range n.Perturbations {
		switch perturbation {
		case PerturbationDisconnect, PerturbationKill, PerturbationPause, PerturbationRestart:
		default:
			return fmt.Errorf("invalid perturbation %q", perturbation)
		}
	}

	return nil
}

// LookupNode looks up a node by name. For now, simply do a linear search.
func (t Testnet) LookupNode(name string) *Node {
	for _, node := range t.Nodes {
		if node.Name == name {
			return node
		}
	}
	return nil
}

// LookupNode looks up a node by name. For now, simply do a linear search.
func (t Testnet) LookupNodeByProTxHash(proTxHash crypto.ProTxHash) *Node {
	for _, node := range t.Nodes {
		if bytes.Equal(node.ProTxHash, proTxHash) {
			return node
		}
	}
	return nil
}

// ArchiveNodes returns a list of archive nodes that start at the initial height
// and contain the entire blockchain history. They are used e.g. as light client
// RPC servers.
func (t Testnet) ArchiveNodes() []*Node {
	nodes := []*Node{}
	for _, node := range t.Nodes {
		if !node.Stateless() && node.StartAt == 0 && node.RetainBlocks == 0 {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// IPv6 returns true if the testnet is an IPv6 network.
func (t Testnet) IPv6() bool {
	return t.IP.IP.To4() == nil
}

// HasPerturbations returns whether the network has any perturbations.
func (t Testnet) HasPerturbations() bool {
	for _, node := range t.Nodes {
		if len(node.Perturbations) > 0 {
			return true
		}
	}
	return false
}

// validatorUpdate creates an abci.ValidatorUpdate struct from the current node
func (n *Node) validatorUpdate(pubKey crypto.PubKey) (abci.ValidatorUpdate, error) {
	proTxHash := n.ProTxHash.Bytes()

	power := types.DefaultDashVotingPower

	address := n.AddressP2P(false)
	validatorUpdate := abci.UpdateValidatorProto(proTxHash, pubKey, power, address)
	return validatorUpdate, nil
}

// Address returns a P2P endpoint address for the node.
func (n Node) AddressP2P(withID bool) string {
	ip := n.IP.String()
	if n.IP.To4() == nil {
		// IPv6 addresses must be wrapped in [] to avoid conflict with : port separator
		ip = fmt.Sprintf("[%v]", ip)
	}
	addr := fmt.Sprintf("%v:%d", ip, networkPortP2P)
	if withID {
		addr = fmt.Sprintf("%x@%v", n.NodeKey.PubKey().Address().Bytes(), addr)
	}
	return addr
}

// Address returns an RPC endpoint address for the node.
func (n Node) AddressRPC() string {
	ip := n.IP.String()
	if n.IP.To4() == nil {
		// IPv6 addresses must be wrapped in [] to avoid conflict with : port separator
		ip = fmt.Sprintf("[%v]", ip)
	}
	return fmt.Sprintf("%v:26657", ip)
}

// Client returns an RPC client for a node.
func (n Node) Client() (*rpchttp.HTTP, error) {
	return rpchttp.New(fmt.Sprintf("http://127.0.0.1:%v", n.ProxyPort))
}

// Stateless returns true if the node is either a seed node or a light node
func (n Node) Stateless() bool {
	return n.Mode == ModeLight || n.Mode == ModeSeed
}

// keyGenerator generates pseudorandom keys based on a seed.
type keyGenerator struct {
	random *rand.Rand
}

func newKeyGenerator(seed int64) *keyGenerator {
	return &keyGenerator{
		random: rand.New(rand.NewSource(seed)),
	}
}

func (g *keyGenerator) Generate(keyType string) crypto.PrivKey {
	seed := make([]byte, bls12381.SeedSize)

	_, err := io.ReadFull(g.random, seed)
	if err != nil {
		panic(err) // this shouldn't happen
	}
	switch keyType {
	case "secp256k1":
		return secp256k1.GenPrivKeySecp256k1(seed)
	case "", "bls12381":
		return bls12381.GenPrivKeyFromSecret(seed)
	case "ed25519":
		return ed25519.GenPrivKeyFromSecret(seed)
	default:
		panic("KeyType not supported") // should not make it this far
	}
}

// portGenerator generates local Docker proxy ports for each node.
type portGenerator struct {
	nextPort uint32
}

func newPortGenerator(firstPort uint32) *portGenerator {
	return &portGenerator{nextPort: firstPort}
}

func (g *portGenerator) Next() uint32 {
	port := g.nextPort
	g.nextPort++
	if g.nextPort == 0 {
		panic("port overflow")
	}
	return port
}

// ipGenerator generates sequential IP addresses for each node, using a random
// network address.
type ipGenerator struct {
	network *net.IPNet
	nextIP  net.IP
}

func newIPGenerator(network *net.IPNet) *ipGenerator {
	nextIP := make([]byte, len(network.IP))
	copy(nextIP, network.IP)
	gen := &ipGenerator{network: network, nextIP: nextIP}
	// Skip network and gateway addresses
	gen.Next()
	gen.Next()
	return gen
}

func (g *ipGenerator) Network() *net.IPNet {
	n := &net.IPNet{
		IP:   make([]byte, len(g.network.IP)),
		Mask: make([]byte, len(g.network.Mask)),
	}
	copy(n.IP, g.network.IP)
	copy(n.Mask, g.network.Mask)
	return n
}

func (g *ipGenerator) Next() net.IP {
	ip := make([]byte, len(g.nextIP))
	copy(ip, g.nextIP)
	for i := len(g.nextIP) - 1; i >= 0; i-- {
		g.nextIP[i]++
		if g.nextIP[i] != 0 {
			break
		}
	}
	return ip
}
