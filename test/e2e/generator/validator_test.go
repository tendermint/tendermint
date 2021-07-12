package main

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatorUpdatesPopulator(t *testing.T) {
	testCases := []struct {
		populator          validatorUpdatesPopulator
		expectedUpdates    map[string]map[string]int64
		expectedValHeights map[string][]int64
	}{
		{
			populator: validatorUpdatesPopulator{
				rand:           rand.New(rand.NewSource(2)),
				validatorNames: generateValidatorNames(1),
				quorumMembers:  1,
				quorumRotate:   5,
			},
			expectedUpdates: map[string]map[string]int64{
				"0": {
					"validator01": 100,
				},
			},
			expectedValHeights: map[string][]int64{
				"validator01": {0},
			},
		},
		{
			populator: validatorUpdatesPopulator{
				rand:           rand.New(rand.NewSource(2)),
				validatorNames: generateValidatorNames(10),
				quorumMembers:  4,
				quorumRotate:   5,
			},
			expectedUpdates: map[string]map[string]int64{
				"0": {
					"validator07": 100,
					"validator08": 100,
					"validator05": 100,
					"validator04": 100,
				},
				"5": {
					"validator07": 0,
					"validator08": 0,
					"validator05": 0,
					"validator04": 0,
					"validator01": 100,
					"validator03": 100,
					"validator06": 100,
					"validator10": 100,
				},
				"10": {
					"validator01": 0,
					"validator03": 0,
					"validator06": 0,
					"validator10": 0,
					"validator09": 100,
					"validator07": 100,
					"validator02": 100,
					"validator05": 100,
				},
			},
			expectedValHeights: map[string][]int64{
				"validator01": {5},
				"validator02": {10},
				"validator03": {5},
				"validator04": {0},
				"validator05": {0, 10},
				"validator06": {5},
				"validator07": {0, 10},
				"validator08": {0},
				"validator09": {10},
				"validator10": {5},
			},
		},
	}
	for _, tc := range testCases {
		validatorUpdates := make(map[string]map[string]int64)
		valHeights := tc.populator.populate(validatorUpdates)
		assert.Equal(t, len(valHeights), len(tc.expectedValHeights))
		assert.Equal(t, len(validatorUpdates), len(tc.expectedUpdates))
		assert.Equal(t, tc.expectedValHeights, valHeights)
		assert.Equal(t, tc.expectedUpdates, validatorUpdates)
	}
}
