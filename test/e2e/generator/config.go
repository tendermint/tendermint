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
		quorumHeightRotate: 15,
	},
	"quad": {
		validators:         topologyItem{val: 4},
		chainLocks:         topologyItem{val: 10},
		quorumMembersCount: 2,
		quorumHeightRotate: 15,
	},
	"large": {
		// FIXME Networks are kept small since large ones use too much CPU.
		seeds:              topologyItem{rand: 1},
		lightClients:       topologyItem{rand: 2},
		validators:         topologyItem{val: 4, rand: 4},
		fulls:              topologyItem{rand: 4},
		chainLocks:         topologyItem{val: 10},
		quorumMembersCount: 2,
		quorumHeightRotate: 15,
	},
	"dash": {
		seeds:              topologyItem{val: 3},
		validators:         topologyItem{val: 15},
		fulls:              topologyItem{val: 5},
		chainLocks:         topologyItem{val: 10},
		lightClients:       topologyItem{rand: 2},
		quorumMembersCount: 3,
		quorumHeightRotate: 15,
	},
}

var configPresets = map[string]func(){
	"default": func() {},
	"dash": func() {
		testnetCombinations["topology"] = []interface{}{"dash"}
		nodePrivvalProtocols = weightedChoice{"dashcore": 100}
	},
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
	quorumHeightRotate int
}

func initConfig(preset string) error {
	initFn, ok := configPresets[preset]
	if !ok {
		allowed := make([]string, 0, len(configPresets))
		for k := range configPresets {
			allowed = append(allowed, k)
		}
		return fmt.Errorf("passed unsupported config preset %q allowed values: %v", preset, allowed)
	}
	initFn()
	return nil
}
