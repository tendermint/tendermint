package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Generate generates random testnets using the given RNG.
func Generate(r *rand.Rand) ([]e2e.Manifest, error) {
	var manifests []e2e.Manifest
	for _, opt := range combinations(cfg.testnetCombinations) {
		manifest, err := generateTestnet(r, opt)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}

// generateTestnet generates a single testnet with the given options.
func generateTestnet(r *rand.Rand, opt map[string]interface{}) (e2e.Manifest, error) {
	manifest := e2e.Manifest{
		IPv6:                         opt["ipv6"].(bool),
		InitialHeight:                int64(opt["initialHeight"].(int)),
		InitialCoreChainLockedHeight: uint32(opt["initialCoreChainLockedHeight"].(int)),
		InitialState:                 opt["initialState"].(map[string]string),
		Validators:                   &map[string]int64{},
		ValidatorUpdates:             map[string]map[string]int64{},
		ChainLockUpdates:             map[string]int64{},
		Nodes:                        map[string]*e2e.ManifestNode{},
	}
	topology, ok := topologies[opt["topology"].(string)]
	if !ok {
		return manifest, fmt.Errorf("unknown topology %q", opt["topology"])
	}

	numSeeds := topology.seeds.compute(r)
	numValidators := topology.validators.compute(r)
	numFulls := topology.fulls.compute(r)
	numChainLocks := topology.chainLocks.compute(r)
	numLightClients := topology.lightClients.compute(r)

	// First we generate seed nodes, starting at the initial height.
	for i := 1; i <= numSeeds; i++ {
		manifest.Nodes[fmt.Sprintf("seed%02d", i)] = generateNode(
			r, e2e.ModeSeed, 0, manifest.InitialHeight, false)
	}

	heightStep := int64(topology.quorumRotate)

	// Next, we generate validators. We make sure a BFT quorum of validators start
	// at the initial height, and that we have two archive nodes. We also set up
	// the initial validator set, and validator set updates for delayed nodes.
	nextStartAt := manifest.InitialHeight + heightStep

	// prepare the list of the validator names
	validatorNames := generateValidatorNames(numValidators)

	valPlr := validatorUpdatesPopulator{
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
		initialHeight:  manifest.InitialHeight,
		validatorNames: validatorNames,
		quorumMembers:  topology.quorumMembersCount,
		quorumRotate:   heightStep,
	}
	// generate the validators updates list since initial height and updates every N blocks specified in quorumRotate
	valHeights := valPlr.populate(manifest.ValidatorUpdates)
	for i, name := range validatorNames {
		manifest.Nodes[name] = generateNode(r, e2e.ModeValidator, valHeights[name][0], manifest.InitialHeight, i < 2)
		validatorNames = append(validatorNames, name)
	}

	// Move validators to InitChain if specified.
	switch opt["validators"].(string) {
	case "genesis":
		*manifest.Validators = manifest.ValidatorUpdates["0"]
		delete(manifest.ValidatorUpdates, "0")
	case "initchain":
	default:
		return manifest, fmt.Errorf("invalid validators option %q", opt["validators"])
	}

	// Finally, we generate random full nodes.
	for i := 1; i <= numFulls; i++ {
		startAt := int64(0)
		if r.Float64() >= 0.5 {
			startAt = nextStartAt
			nextStartAt += heightStep
		}
		manifest.Nodes[fmt.Sprintf("full%02d", i)] = generateNode(
			r, e2e.ModeFull, startAt, manifest.InitialHeight, false)
	}

	// Finally, we generate random chain locks.
	startAtChainLocks := r.Intn(5)
	startAtChainLocksHeight := 3000 + r.Intn(5)
	for i := 1; i <= numChainLocks; i++ {
		startAtChainLocks += r.Intn(5)
		startAtChainLocksHeight += r.Intn(5)
		manifest.ChainLockUpdates[fmt.Sprintf("%d", startAtChainLocks)] = int64(startAtChainLocksHeight)
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
		if len(seedNames) > 0 && (i == 0 || r.Float64() >= 0.5) {
			manifest.Nodes[name].Seeds = uniformSetChoice(seedNames).Choose(r)
		} else if i > 0 {
			manifest.Nodes[name].PersistentPeers = uniformSetChoice(peerNames[:i]).Choose(r)
		}
	}

	// lastly, set up the light clients
	for i := 1; i <= numLightClients; i++ {
		startAt := manifest.InitialHeight + heightStep
		manifest.Nodes[fmt.Sprintf("light%02d", i)] = generateLightNode(
			r, startAt+(5*int64(i)), lightProviders,
		)
	}

	return manifest, nil
}

// generateNode randomly generates a node, with some constraints to avoid
// generating invalid configurations. We do not set Seeds or PersistentPeers
// here, since we need to know the overall network topology and startup
// sequencing.
func generateNode(
	r *rand.Rand, mode e2e.Mode, startAt int64, initialHeight int64, forceArchive bool,
) *e2e.ManifestNode {
	n := cfg.node
	node := e2e.ManifestNode{
		Mode:             string(mode),
		StartAt:          startAt,
		Database:         n.databases.Choose(r).(string),
		ABCIProtocol:     n.abciProtocols.Choose(r).(string),
		PrivvalProtocol:  n.privvalProtocols.Choose(r).(string),
		FastSync:         n.fastSyncs.Choose(r).(string),
		StateSync:        n.stateSyncs.Choose(r).(bool) && startAt > 0,
		PersistInterval:  ptrUint64(uint64(n.persistIntervals.Choose(r).(int))),
		SnapshotInterval: uint64(n.snapshotIntervals.Choose(r).(int)),
		RetainBlocks:     uint64(n.retainBlocks.Choose(r).(int)),
		Perturb:          n.perturbations.Choose(r),
	}

	// If this node is forced to be an archive node, retain all blocks and
	// enable state sync snapshotting.
	if forceArchive {
		node.RetainBlocks = 0
		node.SnapshotInterval = 3
	}

	if node.Mode == string(e2e.ModeValidator) {
		misbehaveAt := startAt + 5 + int64(r.Intn(10))
		if startAt == 0 {
			misbehaveAt += initialHeight - 1
		}
		node.Misbehaviors = n.misbehaviors.Choose(r).(misbehaviorOption).atHeight(misbehaveAt)
		if len(node.Misbehaviors) != 0 {
			node.PrivvalProtocol = "file"
		}
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
		Database:        cfg.node.databases.Choose(r).(string),
		ABCIProtocol:    "builtin",
		PersistInterval: ptrUint64(0),
		PersistentPeers: providers,
	}
}

func ptrUint64(i uint64) *uint64 {
	return &i
}

type misbehaviorOption struct {
	misbehavior string
}

func (m misbehaviorOption) atHeight(height int64) map[string]string {
	misbehaviorMap := make(map[string]string)
	if m.misbehavior == "" {
		return misbehaviorMap
	}
	misbehaviorMap[strconv.Itoa(int(height))] = m.misbehavior
	return misbehaviorMap
}
