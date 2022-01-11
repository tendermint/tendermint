package core

import (
	"context"
	"errors"
	"fmt"
	"sort"

	tmquery "github.com/tendermint/tendermint/internal/pubsub/query"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/libs/bytes"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

// Tx allows you to query the transaction results. `nil` could mean the
// transaction is in the mempool, invalidated, or was not sent in the first
// place.
// More: https://docs.tendermint.com/master/rpc/#/Info/tx
func (env *Environment) Tx(ctx context.Context, hash bytes.HexBytes, prove bool) (*coretypes.ResultTx, error) {
	// if index is disabled, return error

	// N.B. The hash parameter is HexBytes so that the reflective parameter
	// decoding logic in the HTTP service will correctly translate from JSON.
	// See https://github.com/tendermint/tendermint/issues/6802 for context.

	if !indexer.KVSinkEnabled(env.EventSinks) {
		return nil, errors.New("transaction querying is disabled due to no kvEventSink")
	}

	for _, sink := range env.EventSinks {
		if sink.Type() == indexer.KV {
			r, err := sink.GetTxByHash(hash)
			if r == nil {
				return nil, fmt.Errorf("tx (%X) not found, err: %w", hash, err)
			}

			height := r.Height
			index := r.Index

			var proof types.TxProof
			if prove {
				block := env.BlockStore.LoadBlock(height)
				proof = block.Data.Txs.Proof(int(index)) // XXX: overflow on 32-bit machines
			}

			return &coretypes.ResultTx{
				Hash:     hash,
				Height:   height,
				Index:    index,
				TxResult: r.Result,
				Tx:       r.Tx,
				Proof:    proof,
			}, nil
		}
	}

	return nil, fmt.Errorf("transaction querying is disabled on this node due to the KV event sink being disabled")
}

// TxSearch allows you to query for multiple transactions results. It returns a
// list of transactions (maximum ?per_page entries) and the total count.
// More: https://docs.tendermint.com/master/rpc/#/Info/tx_search
func (env *Environment) TxSearch(
	ctx context.Context,
	query string,
	prove bool,
	pagePtr, perPagePtr *int,
	orderBy string,
) (*coretypes.ResultTxSearch, error) {

	if !indexer.KVSinkEnabled(env.EventSinks) {
		return nil, fmt.Errorf("transaction searching is disabled due to no kvEventSink")
	} else if len(query) > maxQueryLength {
		return nil, errors.New("maximum query length exceeded")
	}

	q, err := tmquery.New(query)
	if err != nil {
		return nil, err
	}

	for _, sink := range env.EventSinks {
		if sink.Type() == indexer.KV {
			results, err := sink.SearchTxEvents(ctx, q)
			if err != nil {
				return nil, err
			}

			// sort results (must be done before pagination)
			switch orderBy {
			case "desc", "":
				sort.Slice(results, func(i, j int) bool {
					if results[i].Height == results[j].Height {
						return results[i].Index > results[j].Index
					}
					return results[i].Height > results[j].Height
				})
			case "asc":
				sort.Slice(results, func(i, j int) bool {
					if results[i].Height == results[j].Height {
						return results[i].Index < results[j].Index
					}
					return results[i].Height < results[j].Height
				})
			default:
				return nil, fmt.Errorf("expected order_by to be either `asc` or `desc` or empty: %w", coretypes.ErrInvalidRequest)
			}

			// paginate results
			totalCount := len(results)
			perPage := env.validatePerPage(perPagePtr)

			page, err := validatePage(pagePtr, perPage, totalCount)
			if err != nil {
				return nil, err
			}

			skipCount := validateSkipCount(page, perPage)
			pageSize := tmmath.MinInt(perPage, totalCount-skipCount)

			apiResults := make([]*coretypes.ResultTx, 0, pageSize)
			for i := skipCount; i < skipCount+pageSize; i++ {
				r := results[i]

				var proof types.TxProof
				if prove {
					block := env.BlockStore.LoadBlock(r.Height)
					proof = block.Data.Txs.Proof(int(r.Index)) // XXX: overflow on 32-bit machines
				}

				apiResults = append(apiResults, &coretypes.ResultTx{
					Hash:     types.Tx(r.Tx).Hash(),
					Height:   r.Height,
					Index:    r.Index,
					TxResult: r.Result,
					Tx:       r.Tx,
					Proof:    proof,
				})
			}

			return &coretypes.ResultTxSearch{Txs: apiResults, TotalCount: totalCount}, nil
		}
	}

	return nil, fmt.Errorf("transaction searching is disabled on this node due to the KV event sink being disabled")
}
