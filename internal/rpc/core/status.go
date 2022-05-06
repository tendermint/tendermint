package core

import (
	"context"
	"fmt"
	"time"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

// Status returns Tendermint status including node info, pubkey, latest block
// hash, app hash, block height, current max peer block height, and time.
// More: https://docs.tendermint.com/master/rpc/#/Info/status
func (env *Environment) Status(ctx context.Context) (*coretypes.ResultStatus, error) {
	var (
		earliestBlockHeight   int64
		earliestBlockHash     tmbytes.HexBytes
		earliestAppHash       tmbytes.HexBytes
		earliestBlockTimeNano int64
	)

	if earliestBlockMeta := env.BlockStore.LoadBaseMeta(); earliestBlockMeta != nil {
		earliestBlockHeight = earliestBlockMeta.Header.Height
		earliestAppHash = earliestBlockMeta.Header.AppHash
		earliestBlockHash = earliestBlockMeta.BlockID.Hash
		earliestBlockTimeNano = earliestBlockMeta.Header.Time.UnixNano()
	}

	var (
		latestBlockHash     tmbytes.HexBytes
		latestAppHash       tmbytes.HexBytes
		latestBlockTimeNano int64

		latestHeight = env.BlockStore.Height()
	)

	if latestHeight != 0 {
		if latestBlockMeta := env.BlockStore.LoadBlockMeta(latestHeight); latestBlockMeta != nil {
			latestBlockHash = latestBlockMeta.BlockID.Hash
			latestAppHash = latestBlockMeta.Header.AppHash
			latestBlockTimeNano = latestBlockMeta.Header.Time.UnixNano()
		}
	}

	validatorInfo := coretypes.ValidatorInfo{
		VotingPower: types.DefaultDashVotingPower,
	}

	if env.ProTxHash != nil {
		validatorInfo.ProTxHash = env.ProTxHash
	}

	var applicationInfo coretypes.ApplicationInfo
	if abciInfo, err := env.ABCIInfo(ctx); err == nil {
		applicationInfo.Version = fmt.Sprint(abciInfo.Response.AppVersion)
	}

	result := &coretypes.ResultStatus{
		NodeInfo:        env.NodeInfo,
		ApplicationInfo: applicationInfo,
		SyncInfo: coretypes.SyncInfo{
			LatestBlockHash:     latestBlockHash,
			LatestAppHash:       latestAppHash,
			LatestBlockHeight:   latestHeight,
			LatestBlockTime:     time.Unix(0, latestBlockTimeNano),
			EarliestBlockHash:   earliestBlockHash,
			EarliestAppHash:     earliestAppHash,
			EarliestBlockHeight: earliestBlockHeight,
			EarliestBlockTime:   time.Unix(0, earliestBlockTimeNano),
			// this should start as true, if consensus
			// hasn't started yet, and then flip to false
			// (or true,) depending on what's actually
			// happening.
			CatchingUp: true,
		},
		ValidatorInfo: validatorInfo,
	}

	if env.ConsensusReactor != nil {
		result.SyncInfo.CatchingUp = env.ConsensusReactor.WaitSync()
	}

	if env.BlockSyncReactor != nil {
		result.SyncInfo.MaxPeerBlockHeight = env.BlockSyncReactor.GetMaxPeerBlockHeight()
		result.SyncInfo.TotalSyncedTime = env.BlockSyncReactor.GetTotalSyncedTime()
		result.SyncInfo.RemainingTime = env.BlockSyncReactor.GetRemainingSyncTime()
	}

	if env.StateSyncMetricer != nil {
		result.SyncInfo.TotalSnapshots = env.StateSyncMetricer.TotalSnapshots()
		result.SyncInfo.ChunkProcessAvgTime = env.StateSyncMetricer.ChunkProcessAvgTime()
		result.SyncInfo.SnapshotHeight = env.StateSyncMetricer.SnapshotHeight()
		result.SyncInfo.SnapshotChunksCount = env.StateSyncMetricer.SnapshotChunksCount()
		result.SyncInfo.SnapshotChunksTotal = env.StateSyncMetricer.SnapshotChunksTotal()
		result.SyncInfo.BackFilledBlocks = env.StateSyncMetricer.BackFilledBlocks()
		result.SyncInfo.BackFillBlocksTotal = env.StateSyncMetricer.BackFillBlocksTotal()
	}

	return result, nil
}
