package core

import (
	"context"
	"fmt"
	"sort"

	tmquery "github.com/tendermint/tendermint/internal/pubsub/query"
	"github.com/tendermint/tendermint/internal/state/indexer"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

// BlockchainInfo gets block headers for minHeight <= height <= maxHeight.
//
// If maxHeight does not yet exist, blocks up to the current height will be
// returned. If minHeight does not exist (due to pruning), earliest existing
// height will be used.
//
// At most 20 items will be returned. Block headers are returned in descending
// order (highest first).
//
// More: https://docs.tendermint.com/master/rpc/#/Info/blockchain
func (env *Environment) BlockchainInfo(ctx context.Context, req *coretypes.RequestBlockchainInfo) (*coretypes.ResultBlockchainInfo, error) {
	const limit = 20
	minHeight, maxHeight, err := filterMinMax(
		env.BlockStore.Base(),
		env.BlockStore.Height(),
		int64(req.MinHeight),
		int64(req.MaxHeight),
		limit,
	)
	if err != nil {
		return nil, err
	}
	env.Logger.Debug("BlockchainInfo", "maxHeight", maxHeight, "minHeight", minHeight)

	blockMetas := make([]*types.BlockMeta, 0, maxHeight-minHeight+1)
	for height := maxHeight; height >= minHeight; height-- {
		blockMeta := env.BlockStore.LoadBlockMeta(height)
		if blockMeta != nil {
			blockMetas = append(blockMetas, blockMeta)
		}
	}

	return &coretypes.ResultBlockchainInfo{
		LastHeight: env.BlockStore.Height(),
		BlockMetas: blockMetas,
	}, nil
}

// error if either min or max are negative or min > max
// if 0, use blockstore base for min, latest block height for max
// enforce limit.
func filterMinMax(base, height, min, max, limit int64) (int64, int64, error) {
	// filter negatives
	if min < 0 || max < 0 {
		return min, max, coretypes.ErrZeroOrNegativeHeight
	}

	// adjust for default values
	if min == 0 {
		min = 1
	}
	if max == 0 {
		max = height
	}

	// limit max to the height
	max = tmmath.MinInt64(height, max)

	// limit min to the base
	min = tmmath.MaxInt64(base, min)

	// limit min to within `limit` of max
	// so the total number of blocks returned will be `limit`
	min = tmmath.MaxInt64(min, max-limit+1)

	if min > max {
		return min, max, fmt.Errorf("%w: min height %d can't be greater than max height %d",
			coretypes.ErrInvalidRequest, min, max)
	}
	return min, max, nil
}

// Block gets block at a given height.
// If no height is provided, it will fetch the latest block.
// More: https://docs.tendermint.com/master/rpc/#/Info/block
func (env *Environment) Block(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultBlock, error) {
	height, err := env.getHeight(env.BlockStore.Height(), (*int64)(req.Height))
	if err != nil {
		return nil, err
	}

	blockMeta := env.BlockStore.LoadBlockMeta(height)
	if blockMeta == nil {
		return &coretypes.ResultBlock{BlockID: types.BlockID{}, Block: nil}, nil
	}

	block := env.BlockStore.LoadBlock(height)
	return &coretypes.ResultBlock{BlockID: blockMeta.BlockID, Block: block}, nil
}

// BlockByHash gets block by hash.
// More: https://docs.tendermint.com/master/rpc/#/Info/block_by_hash
func (env *Environment) BlockByHash(ctx context.Context, req *coretypes.RequestBlockByHash) (*coretypes.ResultBlock, error) {
	block := env.BlockStore.LoadBlockByHash(req.Hash)
	if block == nil {
		return &coretypes.ResultBlock{BlockID: types.BlockID{}, Block: nil}, nil
	}
	// If block is not nil, then blockMeta can't be nil.
	blockMeta := env.BlockStore.LoadBlockMeta(block.Height)
	return &coretypes.ResultBlock{BlockID: blockMeta.BlockID, Block: block}, nil
}

// Header gets block header at a given height.
// If no height is provided, it will fetch the latest header.
// More: https://docs.tendermint.com/master/rpc/#/Info/header
func (env *Environment) Header(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultHeader, error) {
	height, err := env.getHeight(env.BlockStore.Height(), (*int64)(req.Height))
	if err != nil {
		return nil, err
	}

	blockMeta := env.BlockStore.LoadBlockMeta(height)
	if blockMeta == nil {
		return &coretypes.ResultHeader{}, nil
	}

	return &coretypes.ResultHeader{Header: &blockMeta.Header}, nil
}

// HeaderByHash gets header by hash.
// More: https://docs.tendermint.com/master/rpc/#/Info/header_by_hash
func (env *Environment) HeaderByHash(ctx context.Context, req *coretypes.RequestBlockByHash) (*coretypes.ResultHeader, error) {
	blockMeta := env.BlockStore.LoadBlockMetaByHash(req.Hash)
	if blockMeta == nil {
		return &coretypes.ResultHeader{}, nil
	}

	return &coretypes.ResultHeader{Header: &blockMeta.Header}, nil
}

// Commit gets block commit at a given height.
// If no height is provided, it will fetch the commit for the latest block.
// More: https://docs.tendermint.com/master/rpc/#/Info/commit
func (env *Environment) Commit(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultCommit, error) {
	height, err := env.getHeight(env.BlockStore.Height(), (*int64)(req.Height))
	if err != nil {
		return nil, err
	}

	blockMeta := env.BlockStore.LoadBlockMeta(height)
	if blockMeta == nil {
		return nil, nil
	}
	header := blockMeta.Header

	// If the next block has not been committed yet,
	// use a non-canonical commit
	if height == env.BlockStore.Height() {
		commit := env.BlockStore.LoadSeenCommit()
		// NOTE: we can't yet ensure atomicity of operations in asserting
		// whether this is the latest height and retrieving the seen commit
		if commit != nil && commit.Height == height {
			return coretypes.NewResultCommit(&header, commit, false), nil
		}
	}

	// Return the canonical commit (comes from the block at height+1)
	commit := env.BlockStore.LoadBlockCommit(height)
	if commit == nil {
		return nil, nil
	}
	return coretypes.NewResultCommit(&header, commit, true), nil
}

// BlockResults gets ABCIResults at a given height.
// If no height is provided, it will fetch results for the latest block.
//
// Results are for the height of the block containing the txs.
// More: https://docs.tendermint.com/master/rpc/#/Info/block_results
func (env *Environment) BlockResults(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultBlockResults, error) {
	height, err := env.getHeight(env.BlockStore.Height(), (*int64)(req.Height))
	if err != nil {
		return nil, err
	}

	results, err := env.StateStore.LoadFinalizeBlockResponses(height)
	if err != nil {
		return nil, err
	}

	var totalGasUsed int64
	for _, res := range results.GetTxResults() {
		totalGasUsed += res.GetGasUsed()
	}

	return &coretypes.ResultBlockResults{
		Height:                height,
		TxsResults:            results.TxResults,
		TotalGasUsed:          totalGasUsed,
		FinalizeBlockEvents:   results.Events,
		ValidatorUpdates:      results.ValidatorUpdates,
		ConsensusParamUpdates: results.ConsensusParamUpdates,
	}, nil
}

// BlockSearch searches for a paginated set of blocks matching the provided query.
func (env *Environment) BlockSearch(ctx context.Context, req *coretypes.RequestBlockSearch) (*coretypes.ResultBlockSearch, error) {
	if !indexer.KVSinkEnabled(env.EventSinks) {
		return nil, fmt.Errorf("block searching is disabled due to no kvEventSink")
	}

	q, err := tmquery.New(req.Query)
	if err != nil {
		return nil, err
	}

	var kvsink indexer.EventSink
	for _, sink := range env.EventSinks {
		if sink.Type() == indexer.KV {
			kvsink = sink
		}
	}

	results, err := kvsink.SearchBlockEvents(ctx, q)
	if err != nil {
		return nil, err
	}

	// sort results (must be done before pagination)
	switch req.OrderBy {
	case "desc", "":
		sort.Slice(results, func(i, j int) bool { return results[i] > results[j] })

	case "asc":
		sort.Slice(results, func(i, j int) bool { return results[i] < results[j] })

	default:
		return nil, fmt.Errorf("expected order_by to be either `asc` or `desc` or empty: %w", coretypes.ErrInvalidRequest)
	}

	// paginate results
	totalCount := len(results)
	perPage := env.validatePerPage(req.PerPage.IntPtr())

	page, err := validatePage(req.Page.IntPtr(), perPage, totalCount)
	if err != nil {
		return nil, err
	}

	skipCount := validateSkipCount(page, perPage)
	pageSize := tmmath.MinInt(perPage, totalCount-skipCount)

	apiResults := make([]*coretypes.ResultBlock, 0, pageSize)
	for i := skipCount; i < skipCount+pageSize; i++ {
		block := env.BlockStore.LoadBlock(results[i])
		if block != nil {
			blockMeta := env.BlockStore.LoadBlockMeta(block.Height)
			if blockMeta != nil {
				apiResults = append(apiResults, &coretypes.ResultBlock{
					Block:   block,
					BlockID: blockMeta.BlockID,
				})
			}
		}
	}

	return &coretypes.ResultBlockSearch{Blocks: apiResults, TotalCount: totalCount}, nil
}
