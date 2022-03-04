package kv

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/google/orderedcode"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/pubsub/query"
	"github.com/tendermint/tendermint/internal/pubsub/query/syntax"
	indexer "github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/types"
)

var _ indexer.TxIndexer = (*TxIndex)(nil)

// TxIndex is the simplest possible indexer
// It is backed by two kv stores:
// 1. txhash - result  (primary key)
// 2. event - txhash   (secondary key)
type TxIndex struct {
	store dbm.DB
}

// NewTxIndex creates new KV indexer.
func NewTxIndex(store dbm.DB) *TxIndex {
	return &TxIndex{
		store: store,
	}
}

// Get gets transaction from the TxIndex storage and returns it or nil if the
// transaction is not found.
func (txi *TxIndex) Get(hash []byte) (*abci.TxResult, error) {
	if len(hash) == 0 {
		return nil, indexer.ErrorEmptyHash
	}

	rawBytes, err := txi.store.Get(primaryKey(hash))
	if err != nil {
		panic(err)
	}
	if rawBytes == nil {
		return nil, nil
	}

	txResult := new(abci.TxResult)
	err = proto.Unmarshal(rawBytes, txResult)
	if err != nil {
		return nil, fmt.Errorf("error reading TxResult: %w", err)
	}

	return txResult, nil
}

// Index indexes transactions using the given list of events. Each key
// that indexed from the tx's events is a composite of the event type and the
// respective attribute's key delimited by a "." (eg. "account.number").
// Any event with an empty type is not indexed.
func (txi *TxIndex) Index(results []*abci.TxResult) error {
	b := txi.store.NewBatch()
	defer b.Close()

	for _, result := range results {
		hash := types.Tx(result.Tx).Hash()

		// index tx by events
		err := txi.indexEvents(result, hash, b)
		if err != nil {
			return err
		}

		// index by height (always)
		err = b.Set(KeyFromHeight(result), hash)
		if err != nil {
			return err
		}

		rawBytes, err := proto.Marshal(result)
		if err != nil {
			return err
		}
		// index by hash (always)
		err = b.Set(primaryKey(hash), rawBytes)
		if err != nil {
			return err
		}
	}

	return b.WriteSync()
}

func (txi *TxIndex) indexEvents(result *abci.TxResult, hash []byte, store dbm.Batch) error {
	for _, event := range result.Result.Events {
		// only index events with a non-empty type
		if len(event.Type) == 0 {
			continue
		}

		for _, attr := range event.Attributes {
			if len(attr.Key) == 0 {
				continue
			}

			// index if `index: true` is set
			compositeTag := fmt.Sprintf("%s.%s", event.Type, attr.Key)
			// ensure event does not conflict with a reserved prefix key
			if compositeTag == types.TxHashKey || compositeTag == types.TxHeightKey {
				return fmt.Errorf("event type and attribute key \"%s\" is reserved; please use a different key", compositeTag)
			}
			if attr.GetIndex() {
				err := store.Set(keyFromEvent(compositeTag, attr.Value, result), hash)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Search performs a search using the given query.
//
// It breaks the query into conditions (like "tx.height > 5"). For each
// condition, it queries the DB index. One special use cases here: (1) if
// "tx.hash" is found, it returns tx result for it (2) for range queries it is
// better for the client to provide both lower and upper bounds, so we are not
// performing a full scan. Results from querying indexes are then intersected
// and returned to the caller, in no particular order.
//
// Search will exit early and return any result fetched so far,
// when a message is received on the context chan.
func (txi *TxIndex) Search(ctx context.Context, q *query.Query) ([]*abci.TxResult, error) {
	select {
	case <-ctx.Done():
		return make([]*abci.TxResult, 0), nil

	default:
	}

	var hashesInitialized bool
	filteredHashes := make(map[string][]byte)

	// get a list of conditions (like "tx.height > 5")
	conditions := q.Syntax()

	// if there is a hash condition, return the result immediately
	hash, ok, err := lookForHash(conditions)
	if err != nil {
		return nil, fmt.Errorf("error during searching for a hash in the query: %w", err)
	} else if ok {
		res, err := txi.Get(hash)
		switch {
		case err != nil:
			return []*abci.TxResult{}, fmt.Errorf("error while retrieving the result: %w", err)
		case res == nil:
			return []*abci.TxResult{}, nil
		default:
			return []*abci.TxResult{res}, nil
		}
	}

	// conditions to skip because they're handled before "everything else"
	skipIndexes := make([]int, 0)

	// extract ranges
	// if both upper and lower bounds exist, it's better to get them in order not
	// no iterate over kvs that are not within range.
	ranges, rangeIndexes := indexer.LookForRanges(conditions)
	if len(ranges) > 0 {
		skipIndexes = append(skipIndexes, rangeIndexes...)

		for _, qr := range ranges {
			if !hashesInitialized {
				filteredHashes = txi.matchRange(ctx, qr, prefixFromCompositeKey(qr.Key), filteredHashes, true)
				hashesInitialized = true

				// Ignore any remaining conditions if the first condition resulted
				// in no matches (assuming implicit AND operand).
				if len(filteredHashes) == 0 {
					break
				}
			} else {
				filteredHashes = txi.matchRange(ctx, qr, prefixFromCompositeKey(qr.Key), filteredHashes, false)
			}
		}
	}

	// if there is a height condition ("tx.height=3"), extract it
	height := lookForHeight(conditions)

	// for all other conditions
	for i, c := range conditions {
		if intInSlice(i, skipIndexes) {
			continue
		}

		if !hashesInitialized {
			filteredHashes = txi.match(ctx, c, prefixForCondition(c, height), filteredHashes, true)
			hashesInitialized = true

			// Ignore any remaining conditions if the first condition resulted
			// in no matches (assuming implicit AND operand).
			if len(filteredHashes) == 0 {
				break
			}
		} else {
			filteredHashes = txi.match(ctx, c, prefixForCondition(c, height), filteredHashes, false)
		}
	}

	results := make([]*abci.TxResult, 0, len(filteredHashes))
hashes:
	for _, h := range filteredHashes {
		res, err := txi.Get(h)
		if err != nil {
			return nil, fmt.Errorf("failed to get Tx{%X}: %w", h, err)
		}
		results = append(results, res)

		// Potentially exit early.
		select {
		case <-ctx.Done():
			break hashes
		default:
		}
	}

	return results, nil
}

func lookForHash(conditions []syntax.Condition) (hash []byte, ok bool, err error) {
	for _, c := range conditions {
		if c.Tag == types.TxHashKey {
			decoded, err := hex.DecodeString(c.Arg.Value())
			return decoded, true, err
		}
	}
	return
}

// lookForHeight returns a height if there is an "height=X" condition.
func lookForHeight(conditions []syntax.Condition) (height int64) {
	for _, c := range conditions {
		if c.Tag == types.TxHeightKey && c.Op == syntax.TEq {
			return int64(c.Arg.Number())
		}
	}
	return 0
}

// match returns all matching txs by hash that meet a given condition and start
// key. An already filtered result (filteredHashes) is provided such that any
// non-intersecting matches are removed.
//
// NOTE: filteredHashes may be empty if no previous condition has matched.
func (txi *TxIndex) match(
	ctx context.Context,
	c syntax.Condition,
	startKeyBz []byte,
	filteredHashes map[string][]byte,
	firstRun bool,
) map[string][]byte {
	// A previous match was attempted but resulted in no matches, so we return
	// no matches (assuming AND operand).
	if !firstRun && len(filteredHashes) == 0 {
		return filteredHashes
	}

	tmpHashes := make(map[string][]byte)

	switch {
	case c.Op == syntax.TEq:
		it, err := dbm.IteratePrefix(txi.store, startKeyBz)
		if err != nil {
			panic(err)
		}
		defer it.Close()

	iterEqual:
		for ; it.Valid(); it.Next() {
			tmpHashes[string(it.Value())] = it.Value()

			// Potentially exit early.
			select {
			case <-ctx.Done():
				break iterEqual
			default:
			}
		}
		if err := it.Error(); err != nil {
			panic(err)
		}

	case c.Op == syntax.TExists:
		// XXX: can't use startKeyBz here because c.Operand is nil
		// (e.g. "account.owner/<nil>/" won't match w/ a single row)
		it, err := dbm.IteratePrefix(txi.store, prefixFromCompositeKey(c.Tag))
		if err != nil {
			panic(err)
		}
		defer it.Close()

	iterExists:
		for ; it.Valid(); it.Next() {
			tmpHashes[string(it.Value())] = it.Value()

			// Potentially exit early.
			select {
			case <-ctx.Done():
				break iterExists
			default:
			}
		}
		if err := it.Error(); err != nil {
			panic(err)
		}

	case c.Op == syntax.TContains:
		// XXX: startKey does not apply here.
		// For example, if startKey = "account.owner/an/" and search query = "account.owner CONTAINS an"
		// we can't iterate with prefix "account.owner/an/" because we might miss keys like "account.owner/Ulan/"
		it, err := dbm.IteratePrefix(txi.store, prefixFromCompositeKey(c.Tag))
		if err != nil {
			panic(err)
		}
		defer it.Close()

	iterContains:
		for ; it.Valid(); it.Next() {
			value, err := parseValueFromKey(it.Key())
			if err != nil {
				continue
			}
			if strings.Contains(value, c.Arg.Value()) {
				tmpHashes[string(it.Value())] = it.Value()
			}

			// Potentially exit early.
			select {
			case <-ctx.Done():
				break iterContains
			default:
			}
		}
		if err := it.Error(); err != nil {
			panic(err)
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

			// Potentially exit early.
			select {
			case <-ctx.Done():
				break
			default:
			}
		}
	}

	return filteredHashes
}

// matchRange returns all matching txs by hash that meet a given queryRange and
// start key. An already filtered result (filteredHashes) is provided such that
// any non-intersecting matches are removed.
//
// NOTE: filteredHashes may be empty if no previous condition has matched.
func (txi *TxIndex) matchRange(
	ctx context.Context,
	qr indexer.QueryRange,
	startKey []byte,
	filteredHashes map[string][]byte,
	firstRun bool,
) map[string][]byte {
	// A previous match was attempted but resulted in no matches, so we return
	// no matches (assuming AND operand).
	if !firstRun && len(filteredHashes) == 0 {
		return filteredHashes
	}

	tmpHashes := make(map[string][]byte)
	lowerBound := qr.LowerBoundValue()
	upperBound := qr.UpperBoundValue()

	it, err := dbm.IteratePrefix(txi.store, startKey)
	if err != nil {
		panic(err)
	}
	defer it.Close()

iter:
	for ; it.Valid(); it.Next() {
		value, err := parseValueFromKey(it.Key())
		if err != nil {
			continue
		}
		if _, ok := qr.AnyBound().(int64); ok {
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				continue iter
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

			// XXX: passing time in a ABCI Events is not yet implemented
			// case time.Time:
			// 	v := strconv.ParseInt(extractValueFromKey(it.Key()), 10, 64)
			// 	if v == r.upperBound {
			// 		break
			// 	}
		}

		// Potentially exit early.
		select {
		case <-ctx.Done():
			break iter
		default:
		}
	}
	if err := it.Error(); err != nil {
		panic(err)
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

			// Potentially exit early.
			select {
			case <-ctx.Done():
				break
			default:
			}
		}
	}

	return filteredHashes
}

// ##########################  Keys  #############################
//
// The indexer has two types of kv stores:
// 1. txhash - result  (primary key)
// 2. event - txhash   (secondary key)
//
// The event key can be decomposed into 4 parts.
// 1. A composite key which can be any string.
// Usually something like "tx.height" or "account.owner"
// 2. A value. That corresponds to the key. In the above
// example the value could be "5" or "Ivan"
// 3. The height of the Tx that aligns with the key and value.
// 4. The index of the Tx that aligns with the key and value

// the hash/primary key
func primaryKey(hash []byte) []byte {
	key, err := orderedcode.Append(
		nil,
		types.TxHashKey,
		string(hash),
	)
	if err != nil {
		panic(err)
	}
	return key
}

// The event/secondary key
func secondaryKey(compositeKey, value string, height int64, index uint32) []byte {
	key, err := orderedcode.Append(
		nil,
		compositeKey,
		value,
		height,
		int64(index),
	)
	if err != nil {
		panic(err)
	}
	return key
}

// parseValueFromKey parses an event key and extracts out the value, returning an error if one arises.
// This will also involve ensuring that the key has the correct format.
// CONTRACT: function doesn't check that the prefix is correct. This should have already been done by the iterator
func parseValueFromKey(key []byte) (string, error) {
	var (
		compositeKey, value string
		height, index       int64
	)
	remaining, err := orderedcode.Parse(string(key), &compositeKey, &value, &height, &index)
	if err != nil {
		return "", err
	}
	if len(remaining) != 0 {
		return "", fmt.Errorf("unexpected remainder in key: %s", remaining)
	}
	return value, nil
}

func keyFromEvent(compositeKey string, value string, result *abci.TxResult) []byte {
	return secondaryKey(compositeKey, value, result.Height, result.Index)
}

func KeyFromHeight(result *abci.TxResult) []byte {
	return secondaryKey(types.TxHeightKey, fmt.Sprintf("%d", result.Height), result.Height, result.Index)
}

// Prefixes: these represent an initial part of the key and are used by iterators to iterate over a small
// section of the kv store during searches.

func prefixFromCompositeKey(compositeKey string) []byte {
	key, err := orderedcode.Append(nil, compositeKey)
	if err != nil {
		panic(err)
	}
	return key
}

func prefixFromCompositeKeyAndValue(compositeKey, value string) []byte {
	key, err := orderedcode.Append(nil, compositeKey, value)
	if err != nil {
		panic(err)
	}
	return key
}

// a small utility function for getting a keys prefix based on a condition and a height
func prefixForCondition(c syntax.Condition, height int64) []byte {
	key := prefixFromCompositeKeyAndValue(c.Tag, c.Arg.Value())
	if height > 0 {
		var err error
		key, err = orderedcode.Append(key, height)
		if err != nil {
			panic(err)
		}
	}
	return key
}
