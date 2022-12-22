package kv

import (
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/google/orderedcode"

	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/types"
)

func intInSlice(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}

	return false
}

func int64FromBytes(bz []byte) int64 {
	v, _ := binary.Varint(bz)
	return v
}

func int64ToBytes(i int64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(buf, i)
	return buf[:n]
}

func heightKey(height int64) ([]byte, error) {
	return orderedcode.Append(
		nil,
		types.BlockHeightKey,
		height,
	)
}

func eventKey(compositeKey, typ, eventValue string, height int64, eventSeq int64) ([]byte, error) {
	return orderedcode.Append(
		nil,
		compositeKey,
		eventValue,
		height,
		typ,
		eventSeq,
	)
}

func parseValueFromPrimaryKey(key []byte) (string, error) {
	var (
		compositeKey string
		height       int64
	)

	remaining, err := orderedcode.Parse(string(key), &compositeKey, &height)
	if err != nil {
		return "", fmt.Errorf("failed to parse event key: %w", err)
	}

	if len(remaining) != 0 {
		return "", fmt.Errorf("unexpected remainder in key: %s", remaining)
	}

	return strconv.FormatInt(height, 10), nil
}

func parseValueFromEventKey(key []byte) (string, error) {
	var (
		compositeKey, typ, eventValue string
		height                        int64
	)

	_, err := orderedcode.Parse(string(key), &compositeKey, &eventValue, &height, &typ)
	if err != nil {
		return "", fmt.Errorf("failed to parse event key: %w", err)
	}

	return eventValue, nil
}

func parseHeightFromEventKey(key []byte) (int64, error) {
	var (
		compositeKey, typ, eventValue string
		height                        int64
	)

	_, err := orderedcode.Parse(string(key), &compositeKey, &eventValue, &height, &typ)
	if err != nil {
		return -1, fmt.Errorf("failed to parse event key: %w", err)
	}

	return height, nil
}

func parseEventSeqFromEventKey(key []byte) (int64, error) {
	var (
		compositeKey, typ, eventValue string
		height                        int64
		eventSeq                      int64
	)

	remaining, err := orderedcode.Parse(string(key), &compositeKey, &eventValue, &height, &typ)
	if err != nil {
		return 0, fmt.Errorf("failed to parse event key: %w", err)
	}

	// This is done to support previous versions that did not have event sequence in their key
	if len(remaining) != 0 {
		remaining, err = orderedcode.Parse(remaining, &eventSeq)
		if err != nil {
			return 0, fmt.Errorf("failed to parse event key: %w", err)
		}
		if len(remaining) != 0 {
			return 0, fmt.Errorf("unexpected remainder in key: %s", remaining)
		}
	}

	return eventSeq, nil
}

func lookForHeight(conditions []query.Condition) (int64, bool, int) {
	for i, c := range conditions {
		if c.CompositeKey == types.BlockHeightKey && c.Op == query.OpEqual {
			return c.Operand.(int64), true, i
		}
	}

	return 0, false, -1
}

func dedupHeight(conditions []query.Condition) (dedupConditions []query.Condition, height int64, found bool, idx int) {
	idx = -1
	for i, c := range conditions {
		if c.CompositeKey == types.BlockHeightKey && c.Op == query.OpEqual {
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
