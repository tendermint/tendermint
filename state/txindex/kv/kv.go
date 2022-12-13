package kv

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/proto"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

const (
	tagKeySeparator = "/"
)

var _ txindex.TxIndexer = (*TxIndex)(nil)

// TxIndex is the simplest possible indexer, backed by key-value storage (levelDB).
type TxIndex struct {
	store dbm.DB
	// Number the events in the event list
	eventSeq int64
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
		return nil, txindex.ErrorEmptyHash
	}

	rawBytes, err := txi.store.Get(hash)
	if err != nil {
		panic(err)
	}
	if rawBytes == nil {
		return nil, nil
	}

	txResult := new(abci.TxResult)
	err = proto.Unmarshal(rawBytes, txResult)
	if err != nil {
		return nil, fmt.Errorf("error reading TxResult: %v", err)
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
		hash := types.Tx(result.Tx).Hash()

		// index tx by events
		err := txi.indexEvents(result, hash, storeBatch)
		if err != nil {
			return err
		}

		// index by height (always)
		err = storeBatch.Set(keyForHeight(result), hash)
		if err != nil {
			return err
		}

		rawBytes, err := proto.Marshal(result)
		if err != nil {
			return err
		}
		// index by hash (always)
		err = storeBatch.Set(hash, rawBytes)
		if err != nil {
			return err
		}
	}

	return storeBatch.WriteSync()
}

// Index indexes a single transaction using the given list of events. Each key
// that indexed from the tx's events is a composite of the event type and the
// respective attribute's key delimited by a "." (eg. "account.number").
// Any event with an empty type is not indexed.
//
// If a transaction is indexed with the same hash as a previous transaction, it will
// be overwritten unless the tx result was NOT OK and the prior result was OK i.e.
// more transactions that successfully executed overwrite transactions that failed
// or successful yet older transactions.
func (txi *TxIndex) Index(result *abci.TxResult) error {
	b := txi.store.NewBatch()
	defer b.Close()

	hash := types.Tx(result.Tx).Hash()

	if !result.Result.IsOK() {
		oldResult, err := txi.Get(hash)
		if err != nil {
			return err
		}

		// if the new transaction failed and it's already indexed in an older block and was successful
		// we skip it as we want users to get the older successful transaction when they query.
		if oldResult != nil && oldResult.Result.Code == abci.CodeTypeOK {
			return nil
		}
	}

	// index tx by events
	err := txi.indexEvents(result, hash, b)
	if err != nil {
		return err
	}

	// index by height (always)
	err = b.Set(keyForHeight(result), hash)
	if err != nil {
		return err
	}

	rawBytes, err := proto.Marshal(result)
	if err != nil {
		return err
	}
	// index by hash (always)
	err = b.Set(hash, rawBytes)
	if err != nil {
		return err
	}

	return b.WriteSync()
}

func (txi *TxIndex) indexEvents(result *abci.TxResult, hash []byte, store dbm.Batch) error {
	for _, event := range result.Result.Events {
		txi.eventSeq = txi.eventSeq + 1
		// only index events with a non-empty type
		if len(event.Type) == 0 {
			continue
		}

		for _, attr := range event.Attributes {
			if len(attr.Key) == 0 {
				continue
			}

			// index if `index: true` is set
			compositeTag := fmt.Sprintf("%s.%s", event.Type, string(attr.Key))
			if attr.GetIndex() {
				err := store.Set(keyForEvent(compositeTag, attr.Value, result, txi.eventSeq), hash)
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
	conditions, err := q.Conditions()
	if err != nil {
		return nil, fmt.Errorf("error during parsing conditions from query: %w", err)
	}

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

	var matchEvents bool
	var matchEventIdx int

	// If the match.events keyword is at the beginning of the query, we will only
	// return heights where the conditions are true within the same event
	// and set the matchEvents to true
	conditions, matchEvents = dedupMatchEvents(conditions)

	if matchEvents {
		matchEventIdx = 0
	} else {
		matchEventIdx = -1
	}

	// conditions to skip because they're handled before "everything else"
	skipIndexes := make([]int, 0)

	if matchEventIdx != -1 {
		skipIndexes = append(skipIndexes, matchEventIdx)
	}
	// extract ranges
	// if both upper and lower bounds exist, it's better to get them in order not
	// no iterate over kvs that are not within range.
	ranges, rangeIndexes := indexer.LookForRanges(conditions)
	var heightRanges indexer.QueryRange
	if len(ranges) > 0 {
		skipIndexes = append(skipIndexes, rangeIndexes...)

		for _, qr := range ranges {

			//If we have a query range over height and want to still look for
			// specific event values we do not want to simply return all
			// transactios in this height range. We remember the height range info
			// and pass it on to match() to take into account when processing events.
			if qr.Key == types.TxHeightKey && matchEvents {
				heightRanges = qr
				continue
			}
			if !hashesInitialized {
				filteredHashes = txi.matchRange(ctx, qr, startKey(qr.Key), filteredHashes, true, matchEvents)
				hashesInitialized = true

				// Ignore any remaining conditions if the first condition resulted
				// in no matches (assuming implicit AND operand).
				if len(filteredHashes) == 0 {
					break
				}
			} else {
				filteredHashes = txi.matchRange(ctx, qr, startKey(qr.Key), filteredHashes, false, matchEvents)
			}
		}
	}

	// if there is a height condition ("tx.height=3"), extract it
	var height int64
	var heightIdx int
	if matchEvents {
		// If we are not matching events and tx.height = 3 occurs more than once, the later value will
		// overwrite the first one. For match.events it will create problems.
		conditions, height, heightIdx = dedupHeight(conditions)
	} else {
		height, heightIdx = lookForHeight(conditions)
	}
	if matchEvents && (len(conditions) != 2) {
		skipIndexes = append(skipIndexes, heightIdx)
	}

	// for all other conditions
	for i, c := range conditions {
		if intInSlice(i, skipIndexes) {
			continue
		}

		if !hashesInitialized {
			filteredHashes = txi.match(ctx, c, startKeyForCondition(c, height), filteredHashes, true, matchEvents, height, heightRanges)
			hashesInitialized = true

			// Ignore any remaining conditions if the first condition resulted
			// in no matches (assuming implicit AND operand).
			if len(filteredHashes) == 0 {
				break
			}
		} else {
			filteredHashes = txi.match(ctx, c, startKeyForCondition(c, height), filteredHashes, false, matchEvents, height, heightRanges)
		}
	}

	results := make([]*abci.TxResult, 0, len(filteredHashes))
	for _, h := range filteredHashes {
		res, err := txi.Get(h)
		if err != nil {
			return nil, fmt.Errorf("failed to get Tx{%X}: %w", h, err)
		}
		results = append(results, res)

		// Potentially exit early.
		select {
		case <-ctx.Done():
			break
		default:
		}
	}

	return results, nil
}

func lookForHash(conditions []query.Condition) (hash []byte, ok bool, err error) {
	for _, c := range conditions {
		if c.CompositeKey == types.TxHashKey {
			decoded, err := hex.DecodeString(c.Operand.(string))
			return decoded, true, err
		}
	}
	return
}

// lookForHeight returns a height if there is an "height=X" condition.
func lookForHeight(conditions []query.Condition) (height int64, heightIdx int) {
	for i, c := range conditions {
		if c.CompositeKey == types.TxHeightKey && c.Op == query.OpEqual {
			return c.Operand.(int64), i
		}
	}
	return 0, -1
}
func (txi *TxIndex) setTmpHashes(tmpHeights map[string][]byte, it dbm.Iterator, matchEvents bool) {
	if matchEvents {
		eventSeq := extractEventSeqFromKey(it.Key())
		tmpHeights[string(it.Value())+eventSeq] = it.Value()
	} else {
		tmpHeights[string(it.Value())] = it.Value()
	}
}

// match returns all matching txs by hash that meet a given condition and start
// key. An already filtered result (filteredHashes) is provided such that any
// non-intersecting matches are removed.
//
// NOTE: filteredHashes may be empty if no previous condition has matched.
func (txi *TxIndex) match(
	ctx context.Context,
	c query.Condition,
	startKeyBz []byte,
	filteredHashes map[string][]byte,
	firstRun bool,
	matchEvents bool,
	height int64,
	heightRanges indexer.QueryRange,
) map[string][]byte {
	// A previous match was attempted but resulted in no matches, so we return
	// no matches (assuming AND operand).
	if !firstRun && len(filteredHashes) == 0 {
		return filteredHashes
	}

	tmpHashes := make(map[string][]byte)

	switch {
	case c.Op == query.OpEqual:
		it, err := dbm.IteratePrefix(txi.store, startKeyBz)
		if err != nil {
			panic(err)
		}
		defer it.Close()

		for ; it.Valid(); it.Next() {

			// If we have a height range in a query, we need only transactions
			// for this height
			if heightRanges.Key != "" {
				eventHeight, err := extractHeightFromKey(it.Key())
				if err != nil || !checkBounds(heightRanges, eventHeight) {
					continue
				}
			} else if height != 0 {
				// If we have a particular height in the query, return only transactions
				// matching this height.
				eventHeight, err := extractHeightFromKey(it.Key())
				if eventHeight != height || err != nil {
					continue
				}
			}

			txi.setTmpHashes(tmpHashes, it, matchEvents)
			// Potentially exit early.
			select {
			case <-ctx.Done():
				break
			default:
			}
		}
		if err := it.Error(); err != nil {
			panic(err)
		}

	case c.Op == query.OpExists:
		// XXX: can't use startKeyBz here because c.Operand is nil
		// (e.g. "account.owner/<nil>/" won't match w/ a single row)
		it, err := dbm.IteratePrefix(txi.store, startKey(c.CompositeKey))
		if err != nil {
			panic(err)
		}
		defer it.Close()

		for ; it.Valid(); it.Next() {
			txi.setTmpHashes(tmpHashes, it, matchEvents)

			// Potentially exit early.
			select {
			case <-ctx.Done():
				break
			default:
			}
		}
		if err := it.Error(); err != nil {
			panic(err)
		}

	case c.Op == query.OpContains:
		// XXX: startKey does not apply here.
		// For example, if startKey = "account.owner/an/" and search query = "account.owner CONTAINS an"
		// we can't iterate with prefix "account.owner/an/" because we might miss keys like "account.owner/Ulan/"
		it, err := dbm.IteratePrefix(txi.store, startKey(c.CompositeKey))
		if err != nil {
			panic(err)
		}
		defer it.Close()

		for ; it.Valid(); it.Next() {
			if !isTagKey(it.Key()) {
				continue
			}

			if strings.Contains(extractValueFromKey(it.Key()), c.Operand.(string)) {
				txi.setTmpHashes(tmpHashes, it, matchEvents)
			}

			// Potentially exit early.
			select {
			case <-ctx.Done():
				break
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
	for k, v := range filteredHashes {
		tmpHash := tmpHashes[k]
		if tmpHash == nil || !bytes.Equal(tmpHash, v) {
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
	matchEvents bool,
) map[string][]byte {
	// A previous match was attempted but resulted in no matches, so we return
	// no matches (assuming AND operand).
	if !firstRun && len(filteredHashes) == 0 {
		return filteredHashes
	}

	tmpHashes := make(map[string][]byte)

	it, err := dbm.IteratePrefix(txi.store, startKey)
	if err != nil {
		panic(err)
	}
	defer it.Close()

LOOP:
	for ; it.Valid(); it.Next() {
		if !isTagKey(it.Key()) {
			continue
		}

		if _, ok := qr.AnyBound().(int64); ok {
			v, err := strconv.ParseInt(extractValueFromKey(it.Key()), 10, 64)
			if err != nil {
				continue LOOP
			}

			if checkBounds(qr, v) {
				txi.setTmpHashes(tmpHashes, it, matchEvents)
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
			break
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
	for k, v := range filteredHashes {
		tmpHash := tmpHashes[k]
		if tmpHash == nil || !bytes.Equal(tmpHashes[k], v) {
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

// Keys

func isTagKey(key []byte) bool {
	// This should be always 4 if data is indexed together with event sequences
	// The check for 3 was added to allow data indexed before (w/o the event number)
	// to be retrieved.
	numTags := strings.Count(string(key), tagKeySeparator)
	return numTags == 4 || numTags == 3
}

func extractHeightFromKey(key []byte) (int64, error) {
	parts := strings.SplitN(string(key), tagKeySeparator, -1)
	return strconv.ParseInt(parts[2], 10, 64)
}
func extractValueFromKey(key []byte) string {
	parts := strings.SplitN(string(key), tagKeySeparator, -1)
	return parts[1]
}

func extractEventSeqFromKey(key []byte) string {
	parts := strings.SplitN(string(key), tagKeySeparator, -1)

	if len(parts) == 5 {
		return parts[4]
	}
	return "0"
}
func keyForEvent(key string, value []byte, result *abci.TxResult, eventSeq int64) []byte {
	return []byte(fmt.Sprintf("%s/%s/%d/%d/%d",
		key,
		value,
		result.Height,
		result.Index,
		eventSeq,
	))
}

func keyForHeight(result *abci.TxResult) []byte {
	return []byte(fmt.Sprintf("%s/%d/%d/%d/%d",
		types.TxHeightKey,
		result.Height,
		result.Height,
		result.Index,
		// Added to facilitate having the eventSeq in event keys
		// Otherwise queries break expecting 5 entries
		0,
	))
}

func startKeyForCondition(c query.Condition, height int64) []byte {
	if height > 0 {
		return startKey(c.CompositeKey, c.Operand, height)
	}
	return startKey(c.CompositeKey, c.Operand)
}

func startKey(fields ...interface{}) []byte {
	var b bytes.Buffer
	for _, f := range fields {
		b.Write([]byte(fmt.Sprintf("%v", f) + tagKeySeparator))
	}
	return b.Bytes()
}

func checkBounds(ranges indexer.QueryRange, v int64) bool {
	include := true
	lowerBound := ranges.LowerBoundValue()
	upperBound := ranges.UpperBoundValue()
	if lowerBound != nil && v < lowerBound.(int64) {
		include = false
	}

	if upperBound != nil && v > upperBound.(int64) {
		include = false
	}

	return include
}
