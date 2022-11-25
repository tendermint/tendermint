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

func lookForMatchEvent(conditions []query.Condition) bool {
	for _, c := range conditions {
		if c.CompositeKey == types.MatchEventKey {
			return true
		}
	}
	return false
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
