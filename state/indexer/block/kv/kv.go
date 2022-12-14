package kv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/google/orderedcode"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/types"
)

var _ indexer.BlockIndexer = (*BlockerIndexer)(nil)

// BlockerIndexer implements a block indexer, indexing BeginBlock and EndBlock
// events with an underlying KV store. Block events are indexed by their height,
// such that matching search criteria returns the respective block height(s).
type BlockerIndexer struct {
	store dbm.DB

	// Add unique event identifier to use when querying
	// Matching will be done both on height AND eventSeq
	eventSeq int64
}

func New(store dbm.DB) *BlockerIndexer {
	return &BlockerIndexer{
		store: store,
	}
}

// Has returns true if the given height has been indexed. An error is returned
// upon database query failure.
func (idx *BlockerIndexer) Has(height int64) (bool, error) {
	key, err := heightKey(height)
	if err != nil {
		return false, fmt.Errorf("failed to create block height index key: %w", err)
	}

	return idx.store.Has(key)
}

// Index indexes BeginBlock and EndBlock events for a given block by its height.
// The following is indexed:
//
// primary key: encode(block.height | height) => encode(height)
// BeginBlock events: encode(eventType.eventAttr|eventValue|height|begin_block) => encode(height)
// EndBlock events: encode(eventType.eventAttr|eventValue|height|end_block) => encode(height)
func (idx *BlockerIndexer) Index(bh types.EventDataNewBlockHeader) error {
	batch := idx.store.NewBatch()
	defer batch.Close()

	height := bh.Header.Height

	// 1. index by height
	key, err := heightKey(height)
	if err != nil {
		return fmt.Errorf("failed to create block height index key: %w", err)
	}
	if err := batch.Set(key, int64ToBytes(height)); err != nil {
		return err
	}

	// 2. index BeginBlock events
	if err := idx.indexEvents(batch, bh.ResultBeginBlock.Events, "begin_block", height); err != nil {
		return fmt.Errorf("failed to index BeginBlock events: %w", err)
	}

	// 3. index EndBlock events
	if err := idx.indexEvents(batch, bh.ResultEndBlock.Events, "end_block", height); err != nil {
		return fmt.Errorf("failed to index EndBlock events: %w", err)
	}

	return batch.WriteSync()
}

// Search performs a query for block heights that match a given BeginBlock
// and Endblock event search criteria. The given query can match against zero,
// one or more block heights. In the case of height queries, i.e. block.height=H,
// if the height is indexed, that height alone will be returned. An error and
// nil slice is returned. Otherwise, a non-nil slice and nil error is returned.
func (idx *BlockerIndexer) Search(ctx context.Context, q *query.Query) ([]int64, error) {
	results := make([]int64, 0)
	select {
	case <-ctx.Done():
		return results, nil

	default:
	}

	conditions, err := q.Conditions()
	if err != nil {
		return nil, fmt.Errorf("failed to parse query conditions: %w", err)
	}
	// conditions to skip because they're handled before "everything else"
	skipIndexes := make([]int, 0)

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

	if matchEventIdx != -1 {
		skipIndexes = append(skipIndexes, matchEventIdx)
	}
	// If there is an exact height query, return the result immediately
	// (if it exists).
	var height int64
	var ok bool
	var heightIdx int
	if matchEvents {
		// If we are not matching events and block.height = 3 occurs more than once, the later value will
		// overwrite the first one. For match.events it will create problems.
		conditions, height, ok, heightIdx = dedupHeight(conditions)
	} else {
		height, ok, heightIdx = lookForHeight(conditions)
	}

	// If we have additional constraints and want to query per event
	// attributes, we cannot simply return all blocks for a height.
	// But we remember the height we want to find and forward it to
	// match(). If we only have the height constraint and match.events keyword
	// in the query (the second part of the ||), we don't need to query
	// per event conditions and return all events within the height range.
	if ok && (!matchEvents || (matchEvents && len(conditions) == 2)) {
		ok, err := idx.Has(height)
		if err != nil {
			return nil, err
		}

		if ok {
			return []int64{height}, nil
		}

		return results, nil
	}

	var heightsInitialized bool
	filteredHeights := make(map[string][]byte)
	if matchEvents && heightIdx != -1 {
		skipIndexes = append(skipIndexes, heightIdx)
	}

	// Extract ranges. If both upper and lower bounds exist, it's better to get
	// them in order as to not iterate over kvs that are not within range.
	ranges, rangeIndexes := indexer.LookForRanges(conditions)
	var heightRanges indexer.QueryRange
	if len(ranges) > 0 {
		skipIndexes = append(skipIndexes, rangeIndexes...)

		for _, qr := range ranges {
			// If we have a query range over height and want to still look for
			// specific event values we do not want to simply return all
			// blocks in this height range. We remember the height range info
			// and pass it on to match() to take into account when processing events.
			if qr.Key == types.BlockHeightKey && matchEvents {
				heightRanges = qr
				// If the query contains ranges other than the height then we need to treat the height
				// range when querying the conditions of the other range.
				// Otherwise we can just return all the blocks within the height range (as there is no
				// additional constraint on events)
				if len(ranges)+1 != 2 {
					continue
				}
			}
			prefix, err := orderedcode.Append(nil, qr.Key)
			if err != nil {
				return nil, fmt.Errorf("failed to create prefix key: %w", err)
			}

			if !heightsInitialized {
				filteredHeights, err = idx.matchRange(ctx, qr, prefix, filteredHeights, true, matchEvents)
				if err != nil {
					return nil, err
				}

				heightsInitialized = true

				// Ignore any remaining conditions if the first condition resulted in no
				// matches (assuming implicit AND operand).
				if len(filteredHeights) == 0 {
					break
				}
			} else {
				filteredHeights, err = idx.matchRange(ctx, qr, prefix, filteredHeights, false, matchEvents)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// for all other conditions
	for i, c := range conditions {
		if intInSlice(i, skipIndexes) {
			continue
		}

		startKey, err := orderedcode.Append(nil, c.CompositeKey, fmt.Sprintf("%v", c.Operand))

		if err != nil {
			return nil, err
		}

		if !heightsInitialized {
			filteredHeights, err = idx.match(ctx, c, startKey, filteredHeights, true, matchEvents, height, heightRanges)
			if err != nil {
				return nil, err
			}

			heightsInitialized = true

			// Ignore any remaining conditions if the first condition resulted in no
			// matches (assuming implicit AND operand).
			if len(filteredHeights) == 0 {
				break
			}
		} else {
			filteredHeights, err = idx.match(ctx, c, startKey, filteredHeights, false, matchEvents, height, heightRanges)
			if err != nil {
				return nil, err
			}
		}
	}

	// fetch matching heights
	results = make([]int64, 0, len(filteredHeights))
	for _, hBz := range filteredHeights {
		h := int64FromBytes(hBz)

		ok, err := idx.Has(h)
		if err != nil {
			return nil, err
		}
		if ok {
			results = append(results, h)
		}

		select {
		case <-ctx.Done():
			break

		default:
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i] < results[j] })

	return results, nil
}

// matchRange returns all matching block heights that match a given QueryRange
// and start key. An already filtered result (filteredHeights) is provided such
// that any non-intersecting matches are removed.
//
// NOTE: The provided filteredHeights may be empty if no previous condition has
// matched.
func (idx *BlockerIndexer) matchRange(
	ctx context.Context,
	qr indexer.QueryRange,
	startKey []byte,
	filteredHeights map[string][]byte,
	firstRun bool,
	matchEvents bool,
) (map[string][]byte, error) {

	// A previous match was attempted but resulted in no matches, so we return
	// no matches (assuming AND operand).
	if !firstRun && len(filteredHeights) == 0 {
		return filteredHeights, nil
	}

	tmpHeights := make(map[string][]byte)

	it, err := dbm.IteratePrefix(idx.store, startKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create prefix iterator: %w", err)
	}
	defer it.Close()

LOOP:
	for ; it.Valid(); it.Next() {
		var (
			eventValue string
			err        error
		)

		if qr.Key == types.BlockHeightKey {
			eventValue, err = parseValueFromPrimaryKey(it.Key())
		} else {
			eventValue, err = parseValueFromEventKey(it.Key())
		}

		if err != nil {
			continue
		}

		if _, ok := qr.AnyBound().(int64); ok {
			v, err := strconv.ParseInt(eventValue, 10, 64)
			if err != nil {
				continue LOOP
			}
			if checkBounds(qr, v) {
				idx.setTmpHeights(tmpHeights, it, matchEvents)
			}
		}

		select {
		case <-ctx.Done():
			break

		default:
		}
	}

	if err := it.Error(); err != nil {
		return nil, err
	}

	if len(tmpHeights) == 0 || firstRun {
		// Either:
		//
		// 1. Regardless if a previous match was attempted, which may have had
		// results, but no match was found for the current condition, then we
		// return no matches (assuming AND operand).
		//
		// 2. A previous match was not attempted, so we return all results.
		return tmpHeights, nil
	}

	// Remove/reduce matches in filteredHashes that were not found in this
	// match (tmpHashes).
	for k, v := range filteredHeights {
		tmpHeight := tmpHeights[k]

		// Check whether in this iteration we have not found an overlapping height (tmpHeight == nil)
		// or whether the events in which the attributed occurred do not match (first part of the condition)
		if tmpHeight == nil || !bytes.Equal(tmpHeight, v) {
			delete(filteredHeights, k)

			select {
			case <-ctx.Done():
				break

			default:
			}
		}
	}

	return filteredHeights, nil
}

func (idx *BlockerIndexer) setTmpHeights(tmpHeights map[string][]byte, it dbm.Iterator, matchEvents bool) {
	// If we return attributes that occur within the same events, then store the event sequence in the
	// result map as well
	if matchEvents {
		eventSeq, _ := parseEventSeqFromEventKey(it.Key())
		retVal := it.Value()
		tmpHeights[string(retVal)+strconv.FormatInt(eventSeq, 10)] = it.Value()
	} else {
		tmpHeights[string(it.Value())] = it.Value()
	}
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

// match returns all matching heights that meet a given query condition and start
// key. An already filtered result (filteredHeights) is provided such that any
// non-intersecting matches are removed.
//
// NOTE: The provided filteredHeights may be empty if no previous condition has
// matched.
func (idx *BlockerIndexer) match(
	ctx context.Context,
	c query.Condition,
	startKeyBz []byte,
	filteredHeights map[string][]byte,
	firstRun bool,
	matchEvents bool,
	height int64,
	heightRanges indexer.QueryRange,
) (map[string][]byte, error) {

	// A previous match was attempted but resulted in no matches, so we return
	// no matches (assuming AND operand).
	if !firstRun && len(filteredHeights) == 0 {
		return filteredHeights, nil
	}

	tmpHeights := make(map[string][]byte)

	switch {
	case c.Op == query.OpEqual:
		it, err := dbm.IteratePrefix(idx.store, startKeyBz)
		if err != nil {
			return nil, fmt.Errorf("failed to create prefix iterator: %w", err)
		}
		defer it.Close()

		for ; it.Valid(); it.Next() {
			if matchEvents {

				if heightRanges.Key != "" {
					eventHeight, err := parseHeightFromEventKey(it.Key())
					if err != nil || !checkBounds(heightRanges, eventHeight) {
						continue
					}
				} else {
					if height != 0 {
						eventHeight, _ := parseHeightFromEventKey(it.Key())
						if eventHeight != height {
							continue
						}
					}
				}
			}
			idx.setTmpHeights(tmpHeights, it, matchEvents)

			if err := ctx.Err(); err != nil {
				break
			}
		}

		if err := it.Error(); err != nil {
			return nil, err
		}

	case c.Op == query.OpExists:
		prefix, err := orderedcode.Append(nil, c.CompositeKey)
		if err != nil {
			return nil, err
		}

		it, err := dbm.IteratePrefix(idx.store, prefix)
		if err != nil {
			return nil, fmt.Errorf("failed to create prefix iterator: %w", err)
		}
		defer it.Close()

		for ; it.Valid(); it.Next() {
			idx.setTmpHeights(tmpHeights, it, matchEvents)

			select {
			case <-ctx.Done():
				break

			default:
			}
		}

		if err := it.Error(); err != nil {
			return nil, err
		}

	case c.Op == query.OpContains:
		prefix, err := orderedcode.Append(nil, c.CompositeKey)
		if err != nil {
			return nil, err
		}

		it, err := dbm.IteratePrefix(idx.store, prefix)
		if err != nil {
			return nil, fmt.Errorf("failed to create prefix iterator: %w", err)
		}
		defer it.Close()

		for ; it.Valid(); it.Next() {
			eventValue, err := parseValueFromEventKey(it.Key())
			if err != nil {
				continue
			}

			if strings.Contains(eventValue, c.Operand.(string)) {
				idx.setTmpHeights(tmpHeights, it, matchEvents)
			}

			select {
			case <-ctx.Done():
				break

			default:
			}
		}
		if err := it.Error(); err != nil {
			return nil, err
		}

	default:
		return nil, errors.New("other operators should be handled already")
	}

	if len(tmpHeights) == 0 || firstRun {
		// Either:
		//
		// 1. Regardless if a previous match was attempted, which may have had
		// results, but no match was found for the current condition, then we
		// return no matches (assuming AND operand).
		//
		// 2. A previous match was not attempted, so we return all results.
		return tmpHeights, nil
	}

	// Remove/reduce matches in filteredHeights that were not found in this
	// match (tmpHeights).
	for k, v := range filteredHeights {
		tmpHeight := tmpHeights[k]
		if tmpHeight == nil || !bytes.Equal(tmpHeight, v) {
			delete(filteredHeights, k)

			select {
			case <-ctx.Done():
				break

			default:
			}
		}
	}

	return filteredHeights, nil
}

func (idx *BlockerIndexer) indexEvents(batch dbm.Batch, events []abci.Event, typ string, height int64) error {
	heightBz := int64ToBytes(height)

	for _, event := range events {
		idx.eventSeq = idx.eventSeq + 1
		// only index events with a non-empty type
		if len(event.Type) == 0 {
			continue
		}

		for _, attr := range event.Attributes {
			if len(attr.Key) == 0 {
				continue
			}

			// index iff the event specified index:true and it's not a reserved event
			compositeKey := fmt.Sprintf("%s.%s", event.Type, string(attr.Key))
			if compositeKey == types.BlockHeightKey {
				return fmt.Errorf("event type and attribute key \"%s\" is reserved; please use a different key", compositeKey)
			}

			if attr.GetIndex() {
				key, err := eventKey(compositeKey, typ, string(attr.Value), height, idx.eventSeq)
				if err != nil {
					return fmt.Errorf("failed to create block index key: %w", err)
				}

				if err := batch.Set(key, heightBz); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
