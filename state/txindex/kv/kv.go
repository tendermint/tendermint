package kv

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	dbm "github.com/tendermint/tm-db"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

const (
	tagKeySeparator = "/"
)

var _ txindex.TxIndexer = (*TxIndex)(nil)

// TxIndex is the simplest possible indexer, backed by key-value storage (levelDB).
type TxIndex struct {
	store        dbm.DB
	tagsToIndex  []string
	indexAllTags bool
}

// NewTxIndex creates new KV indexer.
func NewTxIndex(store dbm.DB, options ...func(*TxIndex)) *TxIndex {
	txi := &TxIndex{store: store, tagsToIndex: make([]string, 0), indexAllTags: false}
	for _, o := range options {
		o(txi)
	}
	return txi
}

// IndexTags is an option for setting which tags to index.
func IndexTags(tags []string) func(*TxIndex) {
	return func(txi *TxIndex) {
		txi.tagsToIndex = tags
	}
}

// IndexAllTags is an option for indexing all tags.
func IndexAllTags() func(*TxIndex) {
	return func(txi *TxIndex) {
		txi.indexAllTags = true
	}
}

// Get gets transaction from the TxIndex storage and returns it or nil if the
// transaction is not found.
func (txi *TxIndex) Get(hash []byte) (*types.TxResult, error) {
	if len(hash) == 0 {
		return nil, txindex.ErrorEmptyHash
	}

	rawBytes := txi.store.Get(hash)
	if rawBytes == nil {
		return nil, nil
	}

	txResult := new(types.TxResult)
	err := cdc.UnmarshalBinaryBare(rawBytes, &txResult)
	if err != nil {
		return nil, fmt.Errorf("Error reading TxResult: %v", err)
	}

	return txResult, nil
}

// AddBatch indexes a batch of transactions using the given list of events. Each
// key that indexed from the tx's events is a composite of the event type and
// the respective attribute's key delimited by a "." (eg. "account.number").
// Any event with an empty type is not indexed.
func (txi *TxIndex) AddBatch(b *txindex.Batch) error {
	storeBatch := txi.store.NewBatch()
	defer storeBatch.Close()

	for _, result := range b.Ops {
		hash := result.Tx.Hash()

		// index tx by events
		txi.indexEvents(result, hash, storeBatch)

		// index tx by height
		if txi.indexAllTags || cmn.StringInSlice(types.TxHeightKey, txi.tagsToIndex) {
			storeBatch.Set(keyForHeight(result), hash)
		}

		// index tx by hash
		rawBytes, err := cdc.MarshalBinaryBare(result)
		if err != nil {
			return err
		}
		storeBatch.Set(hash, rawBytes)
	}

	storeBatch.Write()
	return nil
}

// Index indexes a single transaction using the given list of events. Each key
// that indexed from the tx's events is a composite of the event type and the
// respective attribute's key delimited by a "." (eg. "account.number").
// Any event with an empty type is not indexed.
func (txi *TxIndex) Index(result *types.TxResult) error {
	b := txi.store.NewBatch()
	defer b.Close()

	hash := result.Tx.Hash()

	// index tx by events
	txi.indexEvents(result, hash, b)

	// index tx by height
	if txi.indexAllTags || cmn.StringInSlice(types.TxHeightKey, txi.tagsToIndex) {
		b.Set(keyForHeight(result), hash)
	}

	// index tx by hash
	rawBytes, err := cdc.MarshalBinaryBare(result)
	if err != nil {
		return err
	}

	b.Set(hash, rawBytes)
	b.Write()

	return nil
}

func (txi *TxIndex) indexEvents(result *types.TxResult, hash []byte, store dbm.SetDeleter) {
	for _, event := range result.Result.Events {
		// only index events with a non-empty type
		if len(event.Type) == 0 {
			continue
		}

		for _, attr := range event.Attributes {
			if len(attr.Key) == 0 {
				continue
			}

			compositeTag := fmt.Sprintf("%s.%s", event.Type, string(attr.Key))
			if txi.indexAllTags || cmn.StringInSlice(compositeTag, txi.tagsToIndex) {
				store.Set(keyForEvent(compositeTag, attr.Value, result), hash)
			}
		}
	}
}

// Search performs a search using the given query. It breaks the query into
// conditions (like "tx.height > 5"). For each condition, it queries the DB
// index. One special use cases here: (1) if "tx.hash" is found, it returns tx
// result for it (2) for range queries it is better for the client to provide
// both lower and upper bounds, so we are not performing a full scan. Results
// from querying indexes are then intersected and returned to the caller.
func (txi *TxIndex) Search(q *query.Query) ([]*types.TxResult, error) {
	var hashesInitialized bool
	filteredHashes := make(map[string][]byte)

	// get a list of conditions (like "tx.height > 5")
	conditions := q.Conditions()

	// if there is a hash condition, return the result immediately
	hash, err, ok := lookForHash(conditions)
	if err != nil {
		return nil, errors.Wrap(err, "error during searching for a hash in the query")
	} else if ok {
		res, err := txi.Get(hash)
		if res == nil {
			return []*types.TxResult{}, nil
		}
		return []*types.TxResult{res}, errors.Wrap(err, "error while retrieving the result")
	}

	// conditions to skip because they're handled before "everything else"
	skipIndexes := make([]int, 0)

	// extract ranges
	// if both upper and lower bounds exist, it's better to get them in order not
	// no iterate over kvs that are not within range.
	ranges, rangeIndexes := lookForRanges(conditions)
	if len(ranges) > 0 {
		skipIndexes = append(skipIndexes, rangeIndexes...)

		for _, r := range ranges {
			if !hashesInitialized {
				filteredHashes = txi.matchRange(r, startKey(r.key), filteredHashes, true)
				hashesInitialized = true

				// Ignore any remaining conditions if the first condition resulted
				// in no matches (assuming implicit AND operand).
				if len(filteredHashes) == 0 {
					break
				}
			} else {
				filteredHashes = txi.matchRange(r, startKey(r.key), filteredHashes, false)
			}
		}
	}

	// if there is a height condition ("tx.height=3"), extract it
	height := lookForHeight(conditions)

	// for all other conditions
	for i, c := range conditions {
		if cmn.IntInSlice(i, skipIndexes) {
			continue
		}

		if !hashesInitialized {
			filteredHashes = txi.match(c, startKeyForCondition(c, height), filteredHashes, true)
			hashesInitialized = true

			// Ignore any remaining conditions if the first condition resulted
			// in no matches (assuming implicit AND operand).
			if len(filteredHashes) == 0 {
				break
			}
		} else {
			filteredHashes = txi.match(c, startKeyForCondition(c, height), filteredHashes, false)
		}
	}

	results := make([]*types.TxResult, 0, len(filteredHashes))
	for _, h := range filteredHashes {
		res, err := txi.Get(h)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get Tx{%X}", h)
		}
		results = append(results, res)
	}

	// sort by height & index by default
	sort.Slice(results, func(i, j int) bool {
		if results[i].Height == results[j].Height {
			return results[i].Index < results[j].Index
		}
		return results[i].Height < results[j].Height
	})

	return results, nil
}

func lookForHash(conditions []query.Condition) (hash []byte, err error, ok bool) {
	for _, c := range conditions {
		if c.Tag == types.TxHashKey {
			decoded, err := hex.DecodeString(c.Operand.(string))
			return decoded, err, true
		}
	}
	return
}

// lookForHeight returns a height if there is an "height=X" condition.
func lookForHeight(conditions []query.Condition) (height int64) {
	for _, c := range conditions {
		if c.Tag == types.TxHeightKey && c.Op == query.OpEqual {
			return c.Operand.(int64)
		}
	}
	return 0
}

// special map to hold range conditions
// Example: account.number => queryRange{lowerBound: 1, upperBound: 5}
type queryRanges map[string]queryRange

type queryRange struct {
	key               string
	lowerBound        interface{} // int || time.Time
	includeLowerBound bool
	upperBound        interface{} // int || time.Time
	includeUpperBound bool
}

func (r queryRange) lowerBoundValue() interface{} {
	if r.lowerBound == nil {
		return nil
	}

	if r.includeLowerBound {
		return r.lowerBound
	} else {
		switch t := r.lowerBound.(type) {
		case int64:
			return t + 1
		case time.Time:
			return t.Unix() + 1
		default:
			panic("not implemented")
		}
	}
}

func (r queryRange) AnyBound() interface{} {
	if r.lowerBound != nil {
		return r.lowerBound
	} else {
		return r.upperBound
	}
}

func (r queryRange) upperBoundValue() interface{} {
	if r.upperBound == nil {
		return nil
	}

	if r.includeUpperBound {
		return r.upperBound
	} else {
		switch t := r.upperBound.(type) {
		case int64:
			return t - 1
		case time.Time:
			return t.Unix() - 1
		default:
			panic("not implemented")
		}
	}
}

func lookForRanges(conditions []query.Condition) (ranges queryRanges, indexes []int) {
	ranges = make(queryRanges)
	for i, c := range conditions {
		if isRangeOperation(c.Op) {
			r, ok := ranges[c.Tag]
			if !ok {
				r = queryRange{key: c.Tag}
			}
			switch c.Op {
			case query.OpGreater:
				r.lowerBound = c.Operand
			case query.OpGreaterEqual:
				r.includeLowerBound = true
				r.lowerBound = c.Operand
			case query.OpLess:
				r.upperBound = c.Operand
			case query.OpLessEqual:
				r.includeUpperBound = true
				r.upperBound = c.Operand
			}
			ranges[c.Tag] = r
			indexes = append(indexes, i)
		}
	}
	return ranges, indexes
}

func isRangeOperation(op query.Operator) bool {
	switch op {
	case query.OpGreater, query.OpGreaterEqual, query.OpLess, query.OpLessEqual:
		return true
	default:
		return false
	}
}

// match returns all matching txs by hash that meet a given condition and start
// key. An already filtered result (filteredHashes) is provided such that any
// non-intersecting matches are removed.
//
// NOTE: filteredHashes may be empty if no previous condition has matched.
func (txi *TxIndex) match(c query.Condition, startKeyBz []byte, filteredHashes map[string][]byte, firstRun bool) map[string][]byte {
	// A previous match was attempted but resulted in no matches, so we return
	// no matches (assuming AND operand).
	if !firstRun && len(filteredHashes) == 0 {
		return filteredHashes
	}

	tmpHashes := make(map[string][]byte)

	switch {
	case c.Op == query.OpEqual:
		it := dbm.IteratePrefix(txi.store, startKeyBz)
		defer it.Close()

		for ; it.Valid(); it.Next() {
			tmpHashes[string(it.Value())] = it.Value()
		}

	case c.Op == query.OpContains:
		// XXX: startKey does not apply here.
		// For example, if startKey = "account.owner/an/" and search query = "account.owner CONTAINS an"
		// we can't iterate with prefix "account.owner/an/" because we might miss keys like "account.owner/Ulan/"
		it := dbm.IteratePrefix(txi.store, startKey(c.Tag))
		defer it.Close()

		for ; it.Valid(); it.Next() {
			if !isTagKey(it.Key()) {
				continue
			}

			if strings.Contains(extractValueFromKey(it.Key()), c.Operand.(string)) {
				tmpHashes[string(it.Value())] = it.Value()
			}
		}
	default:
		panic("other operators should be handled already")
	}

	if len(tmpHashes) == 0 || firstRun {
		// Either:
		//
		// 1. Regardless if a previous match was attempted, which may have had
		// results, but no match was found for the current condition, then we
		// return no matches (assuming AND operand).
		//
		// 2. A previous match was not attempted, so we return all results.
		return tmpHashes
	}

	// Remove/reduce matches in filteredHashes that were not found in this
	// match (tmpHashes).
	for k := range filteredHashes {
		if tmpHashes[k] == nil {
			delete(filteredHashes, k)
		}
	}

	return filteredHashes
}

// matchRange returns all matching txs by hash that meet a given queryRange and
// start key. An already filtered result (filteredHashes) is provided such that
// any non-intersecting matches are removed.
//
// NOTE: filteredHashes may be empty if no previous condition has matched.
func (txi *TxIndex) matchRange(r queryRange, startKey []byte, filteredHashes map[string][]byte, firstRun bool) map[string][]byte {
	// A previous match was attempted but resulted in no matches, so we return
	// no matches (assuming AND operand).
	if !firstRun && len(filteredHashes) == 0 {
		return filteredHashes
	}

	tmpHashes := make(map[string][]byte)
	lowerBound := r.lowerBoundValue()
	upperBound := r.upperBoundValue()

	it := dbm.IteratePrefix(txi.store, startKey)
	defer it.Close()

LOOP:
	for ; it.Valid(); it.Next() {
		if !isTagKey(it.Key()) {
			continue
		}

		if _, ok := r.AnyBound().(int64); ok {
			v, err := strconv.ParseInt(extractValueFromKey(it.Key()), 10, 64)
			if err != nil {
				continue LOOP
			}

			include := true
			if lowerBound != nil && v < lowerBound.(int64) {
				include = false
			}

			if upperBound != nil && v > upperBound.(int64) {
				include = false
			}

			if include {
				tmpHashes[string(it.Value())] = it.Value()
			}

			// XXX: passing time in a ABCI Tags is not yet implemented
			// case time.Time:
			// 	v := strconv.ParseInt(extractValueFromKey(it.Key()), 10, 64)
			// 	if v == r.upperBound {
			// 		break
			// 	}
		}
	}

	if len(tmpHashes) == 0 || firstRun {
		// Either:
		//
		// 1. Regardless if a previous match was attempted, which may have had
		// results, but no match was found for the current condition, then we
		// return no matches (assuming AND operand).
		//
		// 2. A previous match was not attempted, so we return all results.
		return tmpHashes
	}

	// Remove/reduce matches in filteredHashes that were not found in this
	// match (tmpHashes).
	for k := range filteredHashes {
		if tmpHashes[k] == nil {
			delete(filteredHashes, k)
		}
	}

	return filteredHashes
}

///////////////////////////////////////////////////////////////////////////////
// Keys

func isTagKey(key []byte) bool {
	return strings.Count(string(key), tagKeySeparator) == 3
}

func extractValueFromKey(key []byte) string {
	parts := strings.SplitN(string(key), tagKeySeparator, 3)
	return parts[1]
}

func keyForEvent(key string, value []byte, result *types.TxResult) []byte {
	return []byte(fmt.Sprintf("%s/%s/%d/%d",
		key,
		value,
		result.Height,
		result.Index,
	))
}

func keyForHeight(result *types.TxResult) []byte {
	return []byte(fmt.Sprintf("%s/%d/%d/%d",
		types.TxHeightKey,
		result.Height,
		result.Height,
		result.Index,
	))
}

func startKeyForCondition(c query.Condition, height int64) []byte {
	if height > 0 {
		return startKey(c.Tag, c.Operand, height)
	}
	return startKey(c.Tag, c.Operand)
}

func startKey(fields ...interface{}) []byte {
	var b bytes.Buffer
	for _, f := range fields {
		b.Write([]byte(fmt.Sprintf("%v", f) + tagKeySeparator))
	}
	return b.Bytes()
}
