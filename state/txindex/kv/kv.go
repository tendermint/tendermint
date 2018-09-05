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
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"

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

// AddBatch indexes a batch of transactions using the given list of tags.
func (txi *TxIndex) AddBatch(b *txindex.Batch) error {
	storeBatch := txi.store.NewBatch()

	for _, result := range b.Ops {
		hash := result.Tx.Hash()

		// index tx by tags
		for _, tag := range result.Result.Tags {
			if txi.indexAllTags || cmn.StringInSlice(string(tag.Key), txi.tagsToIndex) {
				storeBatch.Set(keyForTag(tag, result), hash)
			}
		}

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

// Index indexes a single transaction using the given list of tags.
func (txi *TxIndex) Index(result *types.TxResult) error {
	b := txi.store.NewBatch()

	hash := result.Tx.Hash()

	// index tx by tags
	for _, tag := range result.Result.Tags {
		if txi.indexAllTags || cmn.StringInSlice(string(tag.Key), txi.tagsToIndex) {
			b.Set(keyForTag(tag, result), hash)
		}
	}

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

// Search performs a search using the given query. It breaks the query into
// conditions (like "tx.height > 5"). For each condition, it queries the DB
// index. One special use cases here: (1) if "tx.hash" is found, it returns tx
// result for it (2) for range queries it is better for the client to provide
// both lower and upper bounds, so we are not performing a full scan. Results
// from querying indexes are then intersected and returned to the caller.
func (txi *TxIndex) Search(q *query.Query) ([]*types.TxResult, error) {
	var hashes [][]byte
	var hashesInitialized bool

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
				hashes = txi.matchRange(r, []byte(r.key))
				hashesInitialized = true
			} else {
				hashes = intersect(hashes, txi.matchRange(r, []byte(r.key)))
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
			hashes = txi.match(c, startKey(c, height))
			hashesInitialized = true
		} else {
			hashes = intersect(hashes, txi.match(c, startKey(c, height)))
		}
	}

	results := make([]*types.TxResult, len(hashes))
	i := 0
	for _, h := range hashes {
		results[i], err = txi.Get(h)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get Tx{%X}", h)
		}
		i++
	}

	// sort by height by default
	sort.Slice(results, func(i, j int) bool {
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

func lookForHeight(conditions []query.Condition) (height int64) {
	for _, c := range conditions {
		if c.Tag == types.TxHeightKey {
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

func (txi *TxIndex) match(c query.Condition, startKey []byte) (hashes [][]byte) {
	if c.Op == query.OpEqual {
		it := dbm.IteratePrefix(txi.store, startKey)
		defer it.Close()
		for ; it.Valid(); it.Next() {
			hashes = append(hashes, it.Value())
		}
	} else if c.Op == query.OpContains {
		// XXX: doing full scan because startKey does not apply here
		// For example, if startKey = "account.owner=an" and search query = "accoutn.owner CONSISTS an"
		// we can't iterate with prefix "account.owner=an" because we might miss keys like "account.owner=Ulan"
		it := txi.store.Iterator(nil, nil)
		defer it.Close()
		for ; it.Valid(); it.Next() {
			if !isTagKey(it.Key()) {
				continue
			}
			if strings.Contains(extractValueFromKey(it.Key()), c.Operand.(string)) {
				hashes = append(hashes, it.Value())
			}
		}
	} else {
		panic("other operators should be handled already")
	}
	return
}

func (txi *TxIndex) matchRange(r queryRange, prefix []byte) (hashes [][]byte) {
	// create a map to prevent duplicates
	hashesMap := make(map[string][]byte)

	lowerBound := r.lowerBoundValue()
	upperBound := r.upperBoundValue()

	it := dbm.IteratePrefix(txi.store, prefix)
	defer it.Close()
LOOP:
	for ; it.Valid(); it.Next() {
		if !isTagKey(it.Key()) {
			continue
		}
		switch r.AnyBound().(type) {
		case int64:
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
				hashesMap[fmt.Sprintf("%X", it.Value())] = it.Value()
			}
			// XXX: passing time in a ABCI Tags is not yet implemented
			// case time.Time:
			// 	v := strconv.ParseInt(extractValueFromKey(it.Key()), 10, 64)
			// 	if v == r.upperBound {
			// 		break
			// 	}
		}
	}
	hashes = make([][]byte, len(hashesMap))
	i := 0
	for _, h := range hashesMap {
		hashes[i] = h
		i++
	}
	return
}

///////////////////////////////////////////////////////////////////////////////
// Keys

func startKey(c query.Condition, height int64) []byte {
	var key string
	if height > 0 {
		key = fmt.Sprintf("%s/%v/%d", c.Tag, c.Operand, height)
	} else {
		key = fmt.Sprintf("%s/%v", c.Tag, c.Operand)
	}
	return []byte(key)
}

func isTagKey(key []byte) bool {
	return strings.Count(string(key), tagKeySeparator) == 3
}

func extractValueFromKey(key []byte) string {
	parts := strings.SplitN(string(key), tagKeySeparator, 3)
	return parts[1]
}

func keyForTag(tag cmn.KVPair, result *types.TxResult) []byte {
	return []byte(fmt.Sprintf("%s/%s/%d/%d", tag.Key, tag.Value, result.Height, result.Index))
}

func keyForHeight(result *types.TxResult) []byte {
	return []byte(fmt.Sprintf("%s/%d/%d/%d", types.TxHeightKey, result.Height, result.Height, result.Index))
}

///////////////////////////////////////////////////////////////////////////////
// Utils

func intersect(as, bs [][]byte) [][]byte {
	i := make([][]byte, 0, cmn.MinInt(len(as), len(bs)))
	for _, a := range as {
		for _, b := range bs {
			if bytes.Equal(a, b) {
				i = append(i, a)
			}
		}
	}
	return i
}
