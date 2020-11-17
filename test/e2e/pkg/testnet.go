//nolint: gosec
package e2e

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	mcs "github.com/tendermint/tendermint/test/maverick/consensus"
)

const (
	randomSeed     int64  = 2308084734268
	proxyPortFirst uint32 = 5701
	networkIPv4           = "10.186.73.0/24"
	networkIPv6           = "fd80:b10c::/48"
)

type Mode string
type Protocol string
type Perturbation string

const (
	ModeValidator Mode = "validator"
	ModeFull      Mode = "full"
	ModeSeed      Mode = "seed"

	ProtocolBuiltin Protocol = "builtin"
	ProtocolFile    Protocol = "file"
	ProtocolGRPC    Protocol = "grpc"
	ProtocolTCP     Protocol = "tcp"
	ProtocolUNIX    Protocol = "unix"

	PerturbationDisconnect Perturbation = "disconnect"
	PerturbationKill       Perturbation = "kill"
	PerturbationPause      Perturbation = "pause"
	PerturbationRestart    Perturbation = "restart"
)

// Testnet represents a single testnet.
type Testnet struct {
	Name             string
	File             string
	Dir              string
	IP               *net.IPNet
	InitialHeight    int64
	InitialState     map[string]string
	Validators       map[*Node]int64
	ValidatorUpdates map[int64]map[*Node]int64
	Nodes            []*Node
	KeyType          string
}

// Node represents a Tendermint node in a testnet.
type Node struct {
	Name             string
	Testnet          *Testnet
	Mode             Mode
	PrivvalKey       crypto.PrivKey
	NodeKey          crypto.PrivKey
	IP               net.IP
	ProxyPort        uint32
	StartAt          int64
	FastSync         string
	StateSync        bool
	Database         string
	ABCIProtocol     Protocol
	PrivvalProtocol  Protocol
	PersistInterval  uint64
	SnapshotInterval uint64
	RetainBlocks     uint64
	Seeds            []*Node
	PersistentPeers  []*Node
	Perturbations    []Perturbation
	Misbehaviors     map[int64]string
}

// LoadTestnet loads a testnet from a manifest file, using the filename to
// determine the testnet name and directory (from the basename of the file).
// The testnet generation must be deterministic, since it is generated
// separately by the runner and the test cases. For this reason, testnets use a
// random seed to generate e.g. keys.
func LoadTestnet(file string) (*Testnet, error) {
	manifest, err := LoadManifest(file)
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
	proxyPortGen := newPortGenerator(proxyPortFirst)

	testnet := &Testnet{
		Name:             filepath.Base(dir),
		File:             file,
		Dir:              dir,
		IP:               ipGen.Network(),
		InitialHeight:    1,
		InitialState:     manifest.InitialState,
		Validators:       map[*Node]int64{},
		ValidatorUpdates: map[int64]map[*Node]int64{},
		Nodes:            []*Node{},
	}
	if manifest.InitialHeight > 0 {
		testnet.InitialHeight = manifest.InitialHeight
	}

	// Set up nodes, in alphabetical order (IPs and ports get same order).
	nodeNames := []string{}
	for name := range manifest.Nodes {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	for _, name := range nodeNames {
		nodeManifest := manifest.Nodes[name]
		node := &Node{
			Name:             name,
			Testnet:          testnet,
			PrivvalKey:       keyGen.Generate(manifest.KeyType),
			NodeKey:          keyGen.Generate("ed25519"),
			IP:               ipGen.Next(),
			ProxyPort:        proxyPortGen.Next(),
			Mode:             ModeValidator,
			Database:         "goleveldb",
			ABCIProtocol:     ProtocolUNIX,
			PrivvalProtocol:  ProtocolFile,
			StartAt:          nodeManifest.StartAt,
			FastSync:         nodeManifest.FastSync,
			StateSync:        nodeManifest.StateSync,
			PersistInterval:  1,
			SnapshotInterval: nodeManifest.SnapshotInterval,
			RetainBlocks:     nodeManifest.RetainBlocks,
			Perturbations:    []Perturbation{},
			Misbehaviors:     make(map[int64]string),
		}
		if node.StartAt == testnet.InitialHeight {
			node.StartAt = 0 // normalize to 0 for initial nodes, since code expects this
		}
		if nodeManifest.Mode != "" {
			node.Mode = Mode(nodeManifest.Mode)
		}
		if nodeManifest.Database != "" {
			node.Database = nodeManifest.Database
		}
		if nodeManifest.ABCIProtocol != "" {
			node.ABCIProtocol = Protocol(nodeManifest.ABCIProtocol)
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
		for heightString, misbehavior := range nodeManifest.Misbehaviors {
			height, err := strconv.ParseInt(heightString, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("unable to parse height %s to int64: %w", heightString, err)
			}
			node.Misbehaviors[height] = misbehavior
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
			node.PersistentPeers = append(node.PersistentPeers, peer)
		}

		// If there are no seeds or persistent peers specified, default to persistent
		// connections to all other nodes.
		if len(node.PersistentPeers) == 0 && len(node.Seeds) == 0 {
			for _, peer := range testnet.Nodes {
				if peer.Name == node.Name {
					continue
				}
				node.PersistentPeers = append(node.PersistentPeers, peer)
			}
		}
	}

	// Set up genesis validators. If not specified explicitly, use all validator nodes.
	if manifest.Validators != nil {
		for validatorName, power := range *manifest.Validators {
			validator := testnet.LookupNode(validatorName)
			if validator == nil {
				return nil, fmt.Errorf("unknown validator %q", validatorName)
			}
			testnet.Validators[validator] = power
		}
	} else {
		for _, node := range testnet.Nodes {
			if node.Mode == ModeValidator {
				testnet.Validators[node] = 100
			}
		}
	}

	// Set up validator updates.
	for heightStr, validators := range manifest.ValidatorUpdates {
		height, err := strconv.Atoi(heightStr)
		if err != nil {
			return nil, fmt.Errorf("invalid validator update height %q: %w", height, err)
		}
		valUpdate := map[*Node]int64{}
		for name, power := range validators {
			node := testnet.LookupNode(name)
			if node == nil {
				return nil, fmt.Errorf("unknown validator %q for update at height %v", name, height)
			}
			valUpdate[node] = power
		}
		testnet.ValidatorUpdates[int64(height)] = valUpdate
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
	switch n.FastSync {
	case "", "v0", "v1", "v2":
	default:
		return fmt.Errorf("invalid fast sync setting %q", n.FastSync)
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
	switch n.PrivvalProtocol {
	case ProtocolFile, ProtocolUNIX, ProtocolTCP:
	default:
		return fmt.Errorf("invalid privval protocol setting %q", n.PrivvalProtocol)
	}

	if n.StartAt > 0 && n.StartAt < n.Testnet.InitialHeight {
		return fmt.Errorf("cannot start at height %v lower than initial height %v",
			n.StartAt, n.Testnet.InitialHeight)
	}
	if n.StateSync && n.StartAt == 0 {
		return errors.New("state synced nodes cannot start at the initial height")
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

	if (n.PrivvalProtocol != "file" || n.Mode != "validator") && len(n.Misbehaviors) != 0 {
		return errors.New("must be using \"file\" privval protocol to implement misbehaviors")
	}

	for height, misbehavior := range n.Misbehaviors {
		if height < n.StartAt {
			return fmt.Errorf("misbehavior height %d is below node start height %d",
				height, n.StartAt)
		}
		if height < testnet.InitialHeight {
			return fmt.Errorf("misbehavior height %d is below network initial height %d",
				height, testnet.InitialHeight)
		}
		exists := false
		for possibleBehaviors := range mcs.MisbehaviorList {
			if possibleBehaviors == misbehavior {
				exists = true
			}
		}
		if !exists {
			return fmt.Errorf("misbehavior %s does not exist", misbehavior)
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

// ArchiveNodes returns a list of archive nodes that start at the initial height
// and contain the entire blockchain history. They are used e.g. as light client
// RPC servers.
func (t Testnet) ArchiveNodes() []*Node {
	nodes := []*Node{}
	for _, node := range t.Nodes {
		if node.Mode != ModeSeed && node.StartAt == 0 && node.RetainBlocks == 0 {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// RandomNode returns a random non-seed node.
func (t Testnet) RandomNode() *Node {
	for {
		node := t.Nodes[rand.Intn(len(t.Nodes))]
		if node.Mode != ModeSeed {
			return node
		}
	}
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

// LastMisbehaviorHeight returns the height of the last misbehavior.
func (t Testnet) LastMisbehaviorHeight() int64 {
	lastHeight := int64(0)
	for _, node := range t.Nodes {
		for height := range node.Misbehaviors {
			if height > lastHeight {
				lastHeight = height
			}
		}
	}
	return lastHeight
}

// Address returns a P2P endpoint address for the node.
func (n Node) AddressP2P(withID bool) string {
	ip := n.IP.String()
	if n.IP.To4() == nil {
		// IPv6 addresses must be wrapped in [] to avoid conflict with : port separator
		ip = fmt.Sprintf("[%v]", ip)
	}
	addr := fmt.Sprintf("%v:26656", ip)
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
	return rpchttp.New(fmt.Sprintf("http://127.0.0.1:%v", n.ProxyPort), "/websocket")
}

// keyGenerator generates pseudorandom Ed25519 keys based on a seed.
type keyGenerator struct {
	random *rand.Rand
}

func newKeyGenerator(seed int64) *keyGenerator {
	return &keyGenerator{
		random: rand.New(rand.NewSource(seed)),
	}
}

func (g *keyGenerator) Generate(keyType string) crypto.PrivKey {
	seed := make([]byte, ed25519.SeedSize)

	_, err := io.ReadFull(g.random, seed)
	if err != nil {
		panic(err) // this shouldn't happen
	}
	switch keyType {
	case "secp256k1":
		return secp256k1.GenPrivKeySecp256k1(seed)
	case "", "ed25519":
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
