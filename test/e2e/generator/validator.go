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

func (v *validatorUpdatesPopulator) populate(validatorUpdates map[string]map[string]int64) map[string][]int64 {
	nums := len(v.validatorNames)
	updatesLen := (int64(nums/v.quorumMembers) * v.quorumRotate) + v.initialHeight
	if nums%v.quorumMembers > 0 {
		updatesLen++
	}
	var prevHs string
	valHeights := make(map[string][]int64)
	valNames := append([]string(nil), v.validatorNames...)
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
		valNames = v.generateValidators(validatorUpdates[hs], valNames)
		for name, val := range validatorUpdates[hs] {
			if val == validatorEnabled {
				if _, ok := valHeights[name]; ok && valHeights[name][len(valHeights[name])-1] == currHeight {
					continue
				}
				valHeights[name] = append(valHeights[name], currHeight)
			}
		}
		prevHs = hs
	}
	return valHeights
}

func (v *validatorUpdatesPopulator) generateValidators(validators map[string]int64, names []string) []string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for j := 0; j < v.quorumMembers; j++ {
		for {
			n := r.Intn(len(names))
			name := names[n]
			if _, ok := validators[name]; !ok {
				names = append(names[0:n], names[n+1:]...)
				validators[name] = validatorEnabled
				break
			}
		}
		if len(names) == 0 {
			names = append([]string(nil), v.validatorNames...)
		}
	}
	return names
}
func generateValidatorNames(nums int) []string {
	names := make([]string, 0, nums)
	for i := 1; i <= nums; i++ {
		name := fmt.Sprintf("validator%02d", i)
		names = append(names, name)
	}
	return names
}
