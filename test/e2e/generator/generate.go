package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

var (
	// testnetCombinations defines global testnet options, where we generate a
	// separate testnet for each combination (Cartesian product) of options.
	testnetCombinations = map[string][]interface{}{
		"topology":      {"single", "quad", "large"},
		"initialHeight": {0, 1000},
		"initialState": {
			map[string]string{},
			map[string]string{"initial01": "a", "initial02": "b", "initial03": "c"},
		},
		"validators": {"genesis", "initchain"},
		"abci":       {"builtin", "outofprocess"},
	}

	// The following specify randomly chosen values for testnet nodes.
	nodeDatabases = weightedChoice{
		"goleveldb": 35,
		"badgerdb":  35,
		"boltdb":    15,
		"rocksdb":   10,
		"cleveldb":  5,
	}
	ABCIProtocols = weightedChoice{
		"tcp":  20,
		"grpc": 20,
		"unix": 10,
	}
	nodePrivvalProtocols = weightedChoice{
		"file": 50,
		"grpc": 20,
		"tcp":  20,
		"unix": 10,
	}
	nodeStateSyncs = weightedChoice{
		e2e.StateSyncDisabled: 10,
		e2e.StateSyncP2P:      45,
		e2e.StateSyncRPC:      45,
	}
	nodePersistIntervals  = uniformChoice{0, 1, 5}
	nodeSnapshotIntervals = uniformChoice{0, 5}
	nodeRetainBlocks      = uniformChoice{
		0,
		2 * int(e2e.EvidenceAgeHeight),
		4 * int(e2e.EvidenceAgeHeight),
	}
	nodePerturbations = probSetChoice{
		"disconnect": 0.1,
		"pause":      0.1,
		"kill":       0.1,
		"restart":    0.1,
	}

	// the following specify random chosen values for the entire testnet
	evidence   = uniformChoice{0, 1, 10}
	txSize     = uniformChoice{1024, 4096} // either 1kb or 4kb
	ipv6       = uniformChoice{false, true}
	keyType    = uniformChoice{types.ABCIPubKeyTypeEd25519, types.ABCIPubKeyTypeSecp256k1}
	abciDelays = uniformChoice{"none", "small", "large"}

	voteExtensionEnableHeightOffset = uniformChoice{int64(0), int64(10), int64(100)}
	voteExtensionEnabled            = uniformChoice{true, false}
)

// Generate generates random testnets using the given RNG.
func Generate(r *rand.Rand, opts Options) ([]e2e.Manifest, error) {
	manifests := []e2e.Manifest{}

	for _, opt := range combinations(testnetCombinations) {
		manifest, err := generateTestnet(r, opt)
		if err != nil {
			return nil, err
		}

		if len(manifest.Nodes) < opts.MinNetworkSize {
			continue
		}

		if opts.MaxNetworkSize > 0 && len(manifest.Nodes) >= opts.MaxNetworkSize {
			continue
		}

		manifests = append(manifests, manifest)
	}

	return manifests, nil
}

type Options struct {
	MinNetworkSize int
	MaxNetworkSize int
	NumGroups      int
	Directory      string
	Reverse        bool
}

// generateTestnet generates a single testnet with the given options.
func generateTestnet(r *rand.Rand, opt map[string]interface{}) (e2e.Manifest, error) {
	manifest := e2e.Manifest{
		IPv6:             ipv6.Choose(r).(bool),
		InitialHeight:    int64(opt["initialHeight"].(int)),
		InitialState:     opt["initialState"].(map[string]string),
		Validators:       &map[string]int64{},
		ValidatorUpdates: map[string]map[string]int64{},
		Nodes:            map[string]*e2e.ManifestNode{},
		KeyType:          keyType.Choose(r).(string),
		Evidence:         evidence.Choose(r).(int),
		QueueType:        "priority",
		TxSize:           txSize.Choose(r).(int),
	}

	if voteExtensionEnabled.Choose(r).(bool) {
		manifest.VoteExtensionsEnableHeight = manifest.InitialHeight + voteExtensionEnableHeightOffset.Choose(r).(int64)
	}

	if opt["abci"] == "builtin" {
		manifest.ABCIProtocol = string(e2e.ProtocolBuiltin)
	} else {
		manifest.ABCIProtocol = ABCIProtocols.Choose(r)
	}

	switch abciDelays.Choose(r).(string) {
	case "none":
	case "small":
		manifest.PrepareProposalDelayMS = 100
		manifest.ProcessProposalDelayMS = 100
		manifest.VoteExtensionDelayMS = 20
		manifest.FinalizeBlockDelayMS = 200
	case "large":
		manifest.PrepareProposalDelayMS = 200
		manifest.ProcessProposalDelayMS = 200
		manifest.CheckTxDelayMS = 20
		manifest.VoteExtensionDelayMS = 100
		manifest.FinalizeBlockDelayMS = 500
	}

	var numSeeds, numValidators, numFulls, numLightClients int
	switch opt["topology"].(string) {
	case "single":
		numValidators = 1
	case "quad":
		numValidators = 4
	case "large":
		// FIXME Networks are kept small since large ones use too much CPU.
		numSeeds = r.Intn(1)
		numLightClients = r.Intn(2)
		numValidators = 4 + r.Intn(4)
		numFulls = r.Intn(4)
	default:
		return manifest, fmt.Errorf("unknown topology %q", opt["topology"])
	}

	// First we generate seed nodes, starting at the initial height.
	for i := 1; i <= numSeeds; i++ {
		node := generateNode(r, manifest, e2e.ModeSeed, 0, false)
		manifest.Nodes[fmt.Sprintf("seed%02d", i)] = node
	}

	var numSyncingNodes = 0

	// Next, we generate validators. We make sure a BFT quorum of validators start
	// at the initial height, and that we have two archive nodes. We also set up
	// the initial validator set, and validator set updates for delayed nodes.
	nextStartAt := manifest.InitialHeight + 5
	quorum := numValidators*2/3 + 1
	for i := 1; i <= numValidators; i++ {
		startAt := int64(0)
		if i > quorum && numSyncingNodes < 2 && r.Float64() >= 0.25 {
			numSyncingNodes++
			startAt = nextStartAt
			nextStartAt += 5
		}
		name := fmt.Sprintf("validator%02d", i)
		node := generateNode(r, manifest, e2e.ModeValidator, startAt, i <= 2)

		manifest.Nodes[name] = node

		if startAt == 0 {
			(*manifest.Validators)[name] = int64(30 + r.Intn(71))
		} else {
			manifest.ValidatorUpdates[fmt.Sprint(startAt+5)] = map[string]int64{
				name: int64(30 + r.Intn(71)),
			}
		}
	}

	// Move validators to InitChain if specified.
	switch opt["validators"].(string) {
	case "genesis":
	case "initchain":
		manifest.ValidatorUpdates["0"] = *manifest.Validators
		manifest.Validators = &map[string]int64{}
	default:
		return manifest, fmt.Errorf("invalid validators option %q", opt["validators"])
	}

	// Finally, we generate random full nodes.
	for i := 1; i <= numFulls; i++ {
		startAt := int64(0)
		if numSyncingNodes < 2 && r.Float64() >= 0.5 {
			numSyncingNodes++
			startAt = nextStartAt
			nextStartAt += 5
		}
		node := generateNode(r, manifest, e2e.ModeFull, startAt, false)

		manifest.Nodes[fmt.Sprintf("full%02d", i)] = node
	}

	// We now set up peer discovery for nodes. Seed nodes are fully meshed with
	// each other, while non-seed nodes either use a set of random seeds or a
	// set of random peers that start before themselves.
	var seedNames, peerNames, lightProviders []string
	for name, node := range manifest.Nodes {
		if node.Mode == string(e2e.ModeSeed) {
			seedNames = append(seedNames, name)
		} else {
			// if the full node or validator is an ideal candidate, it is added as a light provider.
			// There are at least two archive nodes so there should be at least two ideal candidates
			if (node.StartAt == 0 || node.StartAt == manifest.InitialHeight) && node.RetainBlocks == 0 {
				lightProviders = append(lightProviders, name)
			}
			peerNames = append(peerNames, name)
		}
	}

	for _, name := range seedNames {
		for _, otherName := range seedNames {
			if name != otherName {
				manifest.Nodes[name].Seeds = append(manifest.Nodes[name].Seeds, otherName)
			}
		}
	}

	sort.Slice(peerNames, func(i, j int) bool {
		iName, jName := peerNames[i], peerNames[j]
		switch {
		case manifest.Nodes[iName].StartAt < manifest.Nodes[jName].StartAt:
			return true
		case manifest.Nodes[iName].StartAt > manifest.Nodes[jName].StartAt:
			return false
		default:
			return strings.Compare(iName, jName) == -1
		}
	})
	for i, name := range peerNames {
		// there are seeds, statesync is disabled, and it's
		// either the first peer by the sort order, and
		// (randomly half of the remaining peers use a seed
		// node; otherwise, choose some remaining set of the
		// peers.

		if len(seedNames) > 0 &&
			manifest.Nodes[name].StateSync == e2e.StateSyncDisabled &&
			(i == 0 || r.Float64() >= 0.5) {

			// choose one of the seeds
			manifest.Nodes[name].Seeds = uniformSetChoice(seedNames).Choose(r)
		} else if i > 1 && r.Float64() >= 0.5 {
			peers := uniformSetChoice(peerNames[:i])
			manifest.Nodes[name].PersistentPeers = peers.ChooseAtLeast(r, 2)
		}
	}

	// lastly, set up the light clients
	for i := 1; i <= numLightClients; i++ {
		startAt := manifest.InitialHeight + 5

		node := generateLightNode(r, startAt+(5*int64(i)), lightProviders)

		manifest.Nodes[fmt.Sprintf("light%02d", i)] = node

	}

	return manifest, nil
}

// generateNode randomly generates a node, with some constraints to avoid
// generating invalid configurations. We do not set Seeds or PersistentPeers
// here, since we need to know the overall network topology and startup
// sequencing.
func generateNode(
	r *rand.Rand,
	manifest e2e.Manifest,
	mode e2e.Mode,
	startAt int64,
	forceArchive bool,
) *e2e.ManifestNode {
	node := e2e.ManifestNode{
		Mode:             string(mode),
		StartAt:          startAt,
		Database:         nodeDatabases.Choose(r),
		PrivvalProtocol:  nodePrivvalProtocols.Choose(r),
		StateSync:        e2e.StateSyncDisabled,
		PersistInterval:  ptrUint64(uint64(nodePersistIntervals.Choose(r).(int))),
		SnapshotInterval: uint64(nodeSnapshotIntervals.Choose(r).(int)),
		RetainBlocks:     uint64(nodeRetainBlocks.Choose(r).(int)),
		Perturb:          nodePerturbations.Choose(r),
	}

	if node.PrivvalProtocol == "" {
		node.PrivvalProtocol = "file"
	}

	if startAt > 0 {
		node.StateSync = nodeStateSyncs.Choose(r)
		if manifest.InitialHeight-startAt <= 5 && node.StateSync == e2e.StateSyncDisabled {
			// avoid needing to blocsync more than five total blocks.
			node.StateSync = uniformSetChoice([]string{
				e2e.StateSyncP2P,
				e2e.StateSyncRPC,
			}).Choose(r)[0]
		}
	}

	// If this node is forced to be an archive node, retain all blocks and
	// enable state sync snapshotting.
	if forceArchive {
		node.RetainBlocks = 0
		node.SnapshotInterval = 3
	}

	// If a node which does not persist state also does not retain blocks, randomly
	// choose to either persist state or retain all blocks.
	if node.PersistInterval != nil && *node.PersistInterval == 0 && node.RetainBlocks > 0 {
		if r.Float64() > 0.5 {
			node.RetainBlocks = 0
		} else {
			node.PersistInterval = ptrUint64(node.RetainBlocks)
		}
	}

	// If either PersistInterval or SnapshotInterval are greater than RetainBlocks,
	// expand the block retention time.
	if node.RetainBlocks > 0 {
		if node.PersistInterval != nil && node.RetainBlocks < *node.PersistInterval {
			node.RetainBlocks = *node.PersistInterval
		}
		if node.RetainBlocks < node.SnapshotInterval {
			node.RetainBlocks = node.SnapshotInterval
		}
	}

	return &node
}

func generateLightNode(r *rand.Rand, startAt int64, providers []string) *e2e.ManifestNode {
	return &e2e.ManifestNode{
		Mode:            string(e2e.ModeLight),
		StartAt:         startAt,
		Database:        nodeDatabases.Choose(r),
		PersistInterval: ptrUint64(0),
		PersistentPeers: providers,
	}
}

func ptrUint64(i uint64) *uint64 {
	return &i
}
