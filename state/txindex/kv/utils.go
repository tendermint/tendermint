package kv

import (
	"fmt"

	"github.com/google/orderedcode"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/types"
)

type HeightInfo struct {
	heightRange     indexer.QueryRange
	height          int64
	heightEqIdx     int
	onlyHeightRange bool
	onlyHeightEq    bool
}

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

func dedupHeight(conditions []query.Condition) (dedupConditions []query.Condition, heightInfo HeightInfo) {
	heightInfo.heightEqIdx = -1
	heightRangeExists := false
	var heightCondition []query.Condition
	heightInfo.onlyHeightEq = true
	heightInfo.onlyHeightRange = true
	for _, c := range conditions {
		if c.CompositeKey == types.TxHeightKey {
			if c.Op == query.OpEqual {
				if heightRangeExists {
					continue
				} else {
					heightCondition = append(heightCondition, c)
					heightInfo.height = c.Operand.(int64)
				}
			} else {
				heightInfo.onlyHeightEq = false
				heightRangeExists = true
				dedupConditions = append(dedupConditions, c)
			}
		} else {
			if c.CompositeKey != types.MatchEventKey {
				heightInfo.onlyHeightRange = false
				heightInfo.onlyHeightEq = false
			}
			dedupConditions = append(dedupConditions, c)
		}
	}
	if !heightRangeExists && len(heightCondition) != 0 {
		heightInfo.heightEqIdx = len(dedupConditions)
		heightInfo.onlyHeightRange = false
		dedupConditions = append(dedupConditions, heightCondition...)
	} else {
		// If we found a range make sure we set the hegiht idx to -1 as the height equality
		// will be removed
		heightInfo.heightEqIdx = -1
		heightInfo.height = 0
		heightInfo.onlyHeightEq = false
	}
	return dedupConditions, heightInfo
}

func checkHeightConditions(heightInfo HeightInfo, keyHeight int64) bool {
	if heightInfo.heightRange.Key != "" && !checkBounds(heightInfo.heightRange, keyHeight) {
		return false
	} else {
		if heightInfo.height != 0 && keyHeight != heightInfo.height {
			return false
		}
	}
	return true
}
