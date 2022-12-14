package kv

import (
	"fmt"

	"github.com/google/orderedcode"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/types"
)

// IntInSlice returns true if a is found in the list.
func intInSlice(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func dedupMatchEvents(conditions []query.Condition) ([]query.Condition, bool) {
	var dedupConditions []query.Condition
	matchEvents := false
	for i, c := range conditions {
		if c.CompositeKey == types.MatchEventKey {
			// Match events should be added only via RPC as the very first query condition
			if i == 0 {
				dedupConditions = append(dedupConditions, c)
				matchEvents = true
			}
		} else {
			dedupConditions = append(dedupConditions, c)
		}

	}
	return dedupConditions, matchEvents
}

func ParseEventSeqFromEventKey(key []byte) (int64, error) {
	var (
		compositeKey, typ, eventValue string
		height                        int64
		eventSeq                      int64
	)

	remaining, err := orderedcode.Parse(string(key), &compositeKey, &eventValue, &height, &typ, &eventSeq)
	if err != nil {
		return 0, fmt.Errorf("failed to parse event key: %w", err)
	}

	if len(remaining) != 0 {
		return 0, fmt.Errorf("unexpected remainder in key: %s", remaining)
	}

	return eventSeq, nil
}
func dedupHeight(conditions []query.Condition) (dedupConditions []query.Condition, height int64, idx int) {
	found := false
	idx = -1
	height = 0
	for i, c := range conditions {
		if c.CompositeKey == types.TxHeightKey && c.Op == query.OpEqual {
			if found {
				continue
			} else {
				dedupConditions = append(dedupConditions, c)
				height = c.Operand.(int64)
				found = true
				idx = i
			}
		} else {
			dedupConditions = append(dedupConditions, c)
		}
	}
	return
}
