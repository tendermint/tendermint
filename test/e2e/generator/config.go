package main

import (
	"fmt"
	"math/rand"
)

var topologies = map[string]topology{
	"single": {
		validators:         topologyItem{val: 1},
		chainLocks:         topologyItem{val: 5},
		quorumMembersCount: 1,
		// TODO: probably need to change quorum-rotate value
		quorumRotate: 15,
	},
	"quad": {
		validators:         topologyItem{val: 4},
		chainLocks:         topologyItem{rand: 10},
		quorumMembersCount: 1,
		quorumRotate:       15,
	},
	"large": {
		// FIXME Networks are kept small since large ones use too much CPU.
		seeds:              topologyItem{rand: 3},
		validators:         topologyItem{val: 4, rand: 7},
		fulls:              topologyItem{rand: 5},
		chainLocks:         topologyItem{rand: 10},
		lightClients:       topologyItem{rand: 3},
		quorumMembersCount: 2,
		quorumRotate:       15,
	},
	"dash": {
		seeds:              topologyItem{rand: 3},
		validators:         topologyItem{val: 15},
		fulls:              topologyItem{rand: 5},
		chainLocks:         topologyItem{rand: 10},
		lightClients:       topologyItem{rand: 3},
		quorumMembersCount: 3,
		quorumRotate:       15,
	},
}

var cfg = defaultCfg()

var configPresets = map[string]config{
	"default": defaultCfg(),
	"dash":    dashCfg(),
}

type topologyItem struct {
	val  int
	rand int
}

func (t topologyItem) compute(r *rand.Rand) int {
	v := t.val
	if t.rand > 0 {
		v += r.Intn(t.rand)
	}
	return v
}

type topology struct {
	seeds              topologyItem
	validators         topologyItem
	fulls              topologyItem
	chainLocks         topologyItem
	lightClients       topologyItem
	quorumMembersCount int
	quorumRotate       int
}

type node struct {
	databases         uniformChoice
	abciProtocols     uniformChoice
	privvalProtocols  uniformChoice
	fastSyncs         uniformChoice
	stateSyncs        uniformChoice
	persistIntervals  uniformChoice
	snapshotIntervals uniformChoice
	retainBlocks      uniformChoice
	perturbations     probSetChoice
	misbehaviors      weightedChoice
}

type config struct {
	testnetCombinations map[string][]interface{}
	node                node
	quorumMembersCount  int
}

func defaultCfg() config {
	return config{
		// testnetCombinations defines global testnet options, where we generate a
		// separate testnet for each combination (Cartesian product) of options.
		testnetCombinations: map[string][]interface{}{
			"topology":                     {"single", "quad", "large"},
			"ipv6":                         {false, true},
			"initialHeight":                {0, 1000},
			"initialCoreChainLockedHeight": {1, 3000},
			"initialState": {
				map[string]string{},
				map[string]string{"initial01": "a", "initial02": "b", "initial03": "c"},
			},
			"validators":                   {"genesis", "initchain"},
		},
		node: node{
			// The following specify randomly chosen values for testnet nodes.
			databases: uniformChoice{"goleveldb", "cleveldb", "rocksdb", "boltdb", "badgerdb"},
			// FIXME: grpc disabled due to https://github.com/tendermint/tendermint/issues/5439
			abciProtocols:    uniformChoice{"unix", "tcp", "builtin"},
			privvalProtocols: uniformChoice{"file", "unix", "tcp", "dashcore"},
			// FIXME: v2 disabled due to flake
			fastSyncs:         uniformChoice{"", "v0"},
			stateSyncs:        uniformChoice{false, true},
			persistIntervals:  uniformChoice{0, 1, 5},
			snapshotIntervals: uniformChoice{0, 3},
			retainBlocks:      uniformChoice{0, 1, 5},
			perturbations: probSetChoice{
				"disconnect": 0.1,
				"pause":      0.1,
				"kill":       0.1,
				"restart":    0.1,
			},
			misbehaviors: weightedChoice{
				// FIXME: evidence disabled due to node panicing when not
				// having sufficient block history to process evidence.
				// https://github.com/tendermint/tendermint/issues/5617
				// misbehaviorOption{"double-prevote"}: 1,
				misbehaviorOption{}: 9,
			},
		},
	}
}

func dashCfg() config {
	cfg := defaultCfg()
	cfg.testnetCombinations["topology"] = []interface{}{"dash"}
	cfg.node.privvalProtocols = uniformChoice{"dashcore"}
	return cfg
}

func initConfig(preset string) error {
	var ok bool
	cfg, ok = configPresets[preset]
	if !ok {
		allowed := make([]string, 0, len(configPresets))
		for k := range configPresets {
			allowed = append(allowed, k)
		}
		return fmt.Errorf("passed unsupported config preset %q allowed values: %v", preset, allowed)
	}
	return nil
}
