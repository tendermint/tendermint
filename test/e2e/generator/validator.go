package main

import (
	"fmt"
	"math/rand"
	"strconv"

	"github.com/tendermint/tendermint/types/time"
)

const (
	validatorEnabled  = 100
	validatorDisabled = 0
)

type validatorUpdatesPopulator struct {
	initialHeight  int64
	validatorNames []string
	quorumMembers  int
	quorumRotate   int64
}

func (v *validatorUpdatesPopulator) populate(validators map[string]int64, validatorUpdates map[string]map[string]int64) {
	v.generateValidators(validators)
	nums := len(v.validatorNames)
	updatesLen := (int64(nums/v.quorumMembers) * v.quorumRotate) + v.initialHeight
	var prevHs string
	for currHeight := v.initialHeight; currHeight < updatesLen; currHeight += v.quorumRotate {
		hs := strconv.FormatInt(currHeight, 10)
		validatorUpdates[hs] = make(map[string]int64)
		if _, ok := validatorUpdates[prevHs]; ok {
			for k, v := range validatorUpdates[prevHs] {
				if v == validatorEnabled {
					validatorUpdates[hs][k] = validatorDisabled
				}
			}
		}
		v.generateValidators(validatorUpdates[hs])
		prevHs = hs
	}
}

func (v *validatorUpdatesPopulator) generateValidators(validators map[string]int64) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for j := 0; j < v.quorumMembers; j++ {
		for {
			n := r.Intn(len(v.validatorNames))
			name := v.validatorNames[n]
			if _, ok := validators[name]; !ok {
				validators[name] = validatorEnabled
				break
			}
		}
	}
}

func makeValidatorNames(nums int) []string {
	names := make([]string, 0, nums)
	for i := 1; i <= nums; i++ {
		name := fmt.Sprintf("validator%02d", i)
		names = append(names, name)
	}
	return names
}

func makeValidatorStartSinceMap(validators map[string]int64, validatorUpdates map[string]map[string]int64) (map[string]int64, error) {
	valStartSince := make(map[string]int64)
	for k := range validators {
		if _, ok := valStartSince[k]; !ok {
			valStartSince[k] = 0
		}
	}
	for hs, validators := range validatorUpdates {
		for name, val := range validators {
			if _, ok := valStartSince[name]; !ok && val == validatorEnabled {
				h, err := strconv.ParseInt(hs, 10, 64)
				if err != nil {
					return nil, err
				}
				valStartSince[name] = h
			}
		}
	}
	return valStartSince, nil
}
