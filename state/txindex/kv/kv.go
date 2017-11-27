package kv

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	abci "github.com/tendermint/abci/types"
	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
	db "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/pubsub/query"
)

var _ txindex.TxIndexer = (*TxIndex)(nil)

// TxIndex is the simplest possible indexer, backed by Key-Value storage (levelDB).
// It can only index transaction by its identifier.
type TxIndex struct {
	store db.DB
}

// NewTxIndex returns new instance of TxIndex.
func NewTxIndex(store db.DB) *TxIndex {
	return &TxIndex{store: store}
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

	r := bytes.NewReader(rawBytes)
	var n int
	var err error
	txResult := wire.ReadBinary(&types.TxResult{}, r, 0, &n, &err).(*types.TxResult)
	if err != nil {
		return nil, fmt.Errorf("Error reading TxResult: %v", err)
	}

	return txResult, nil
}

// AddBatch indexes a batch of transactions using the given list of tags.
func (txi *TxIndex) AddBatch(b *txindex.Batch, allowedTags []string) error {
	storeBatch := txi.store.NewBatch()

	for _, result := range b.Ops {
		hash := result.Tx.Hash()

		// index tx by tags
		for _, tag := range result.Result.Tags {
			if stringInSlice(tag.Key, allowedTags) {
				storeBatch.Set(keyForTag(tag, result), hash)
			}
		}

		// index tx by hash
		rawBytes := wire.BinaryBytes(result)
		storeBatch.Set(hash, rawBytes)
	}

	storeBatch.Write()
	return nil
}

// Index indexes a single transaction using the given list of tags.
func (txi *TxIndex) Index(result *types.TxResult, allowedTags []string) error {
	batch := txindex.NewBatch(1)
	batch.Add(result)
	return txi.AddBatch(batch, allowedTags)
}

func (txi *TxIndex) Search(q *query.Query) ([]*types.TxResult, error) {
	hashes := make(map[string][]byte) // key - (base 16, upper-case hash)

	// get a list of conditions (like "tx.height > 5")
	conditions := q.Conditions()

	// if there is a hash condition, return the result immediately
	hash, err, ok := lookForHash(conditions)
	if err != nil {
		return []*types.TxResult{}, errors.Wrap(err, "error during searching for a hash in the query")
	} else if ok {
		res, err := txi.Get(hash)
		return []*types.TxResult{res}, errors.Wrap(err, "error while retrieving the result")
	}

	// conditions to skip
	skipIndexes := make([]int, 0)

	// if there is a height condition ("tx.height=3"), extract it for faster lookups
	height, heightIndex := lookForHeight(conditions)
	if heightIndex >= 0 {
		skipIndexes = append(skipIndexes, heightIndex)
	}

	var hashes2 [][]byte

	// extract ranges
	// if both upper and lower bounds exist, it's better to get them in order not
	// no iterate over kvs that are not within range.
	ranges, rangeIndexes := lookForRanges(conditions)
	if len(ranges) > 0 {
		skipIndexes = append(skipIndexes, rangeIndexes...)
	}
	for _, r := range ranges {
		hashes2 = txi.matchRange(r, startKeyForRange(r, height, heightIndex > 0))

		// initialize hashes if we're running the first time
		if len(hashes) == 0 {
			for _, h := range hashes2 {
				hashes[hashKey(h)] = h
			}
			continue
		}

		// no matches
		if len(hashes2) == 0 {
			hashes = make(map[string][]byte)
		} else {
			// perform intersection as we go
			for _, h := range hashes2 {
				k := hashKey(h)
				if _, ok := hashes[k]; !ok {
					delete(hashes, k)
				}
			}
		}
	}

	// for all other conditions
	for i, c := range conditions {
		if intInSlice(i, skipIndexes) {
			continue
		}

		hashes2 = txi.match(c, startKey(c, height, heightIndex > 0))

		// initialize hashes if we're running the first time
		if len(hashes) == 0 {
			for _, h := range hashes2 {
				hashes[hashKey(h)] = h
			}
			continue
		}

		// no matches
		if len(hashes2) == 0 {
			hashes = make(map[string][]byte)
		} else {
			// perform intersection as we go
			for _, h := range hashes2 {
				k := hashKey(h)
				if _, ok := hashes[k]; !ok {
					delete(hashes, k)
				}
			}
		}
	}

	results := make([]*types.TxResult, len(hashes))
	i := 0
	for _, h := range hashes {
		results[i], err = txi.Get(h)
		if err != nil {
			return []*types.TxResult{}, errors.Wrapf(err, "failed to get Tx{%X}", h)
		}
		i++
	}

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

func lookForHeight(conditions []query.Condition) (height uint64, index int) {
	for i, c := range conditions {
		if c.Tag == types.TxHeightKey {
			return uint64(c.Operand.(int64)), i
		}
	}
	return 0, -1
}

type queryRanges map[string]queryRange

type queryRange struct {
	key               string
	lowerBound        interface{} // int || time.Time
	includeLowerBound bool
	upperBound        interface{} // int || time.Time
	includeUpperBound bool
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
		it := txi.store.IteratorPrefix(startKey)
		for it.Next() {
			hashes = append(hashes, it.Value())
		}
	} else if c.Op == query.OpContains {
		// XXX: full scan
		it := txi.store.Iterator()
		for it.Next() {
			// if it is a hash key, continue
			if !strings.Contains(string(it.Key()), "/") {
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

func startKey(c query.Condition, height uint64, heightSpecified bool) []byte {
	var key string
	if heightSpecified {
		key = fmt.Sprintf("%s/%v/%d", c.Tag, c.Operand, height)
	} else {
		key = fmt.Sprintf("%s/%v", c.Tag, c.Operand)
	}
	return []byte(key)
}

func startKeyForRange(r queryRange, height uint64, heightSpecified bool) []byte {
	var lowerBound interface{}
	if r.includeLowerBound {
		lowerBound = r.lowerBound
	} else {
		switch t := r.lowerBound.(type) {
		case int64:
			lowerBound = t + 1
		case time.Time:
			lowerBound = t.Unix() + 1
		default:
			panic("not implemented")
		}
	}
	var key string
	if heightSpecified {
		key = fmt.Sprintf("%s/%v/%d", r.key, lowerBound, height)
	} else {
		key = fmt.Sprintf("%s/%v", r.key, lowerBound)
	}
	return []byte(key)
}

func (txi *TxIndex) matchRange(r queryRange, startKey []byte) (hashes [][]byte) {
	it := txi.store.IteratorPrefix(startKey)
	defer it.Release()
	for it.Next() {
		// no other way to stop iterator other than checking for upperBound
		switch (r.upperBound).(type) {
		case int64:
			v, err := strconv.ParseInt(extractValueFromKey(it.Key()), 10, 64)
			if err == nil && v == r.upperBound {
				if r.includeUpperBound {
					hashes = append(hashes, it.Value())
				}
				break
			}
			// XXX: passing time in a ABCI Tags is not yet implemented
			// case time.Time:
			// 	v := strconv.ParseInt(extractValueFromKey(it.Key()), 10, 64)
			// 	if v == r.upperBound {
			// 		break
			// 	}
		}
		hashes = append(hashes, it.Value())
	}
	return
}

func extractValueFromKey(key []byte) string {
	s := string(key)
	parts := strings.SplitN(s, "/", 3)
	return parts[1]
}

func keyForTag(tag *abci.KVPair, result *types.TxResult) []byte {
	switch tag.ValueType {
	case abci.KVPair_STRING:
		return []byte(fmt.Sprintf("%s/%v/%d/%d", tag.Key, tag.ValueString, result.Height, result.Index))
	case abci.KVPair_INT:
		return []byte(fmt.Sprintf("%s/%v/%d/%d", tag.Key, tag.ValueInt, result.Height, result.Index))
	// case abci.KVPair_TIME:
	// 	return []byte(fmt.Sprintf("%s/%d/%d/%d", tag.Key, tag.ValueTime.Unix(), result.Height, result.Index))
	default:
		panic(fmt.Sprintf("Undefined value type: %v", tag.ValueType))
	}
}

func hashKey(hash []byte) string {
	return fmt.Sprintf("%X", hash)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func intInSlice(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
