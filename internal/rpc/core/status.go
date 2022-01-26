package core

import (
	"bytes"
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

	// Return the very last voting power, not the voting power of this validator
	// during the last block.
	var votingPower int64
	if val := env.validatorAtHeight(env.latestUncommittedHeight()); val != nil {
		votingPower = val.VotingPower
	}
	validatorInfo := coretypes.ValidatorInfo{}
	if env.PubKey != nil {
		validatorInfo = coretypes.ValidatorInfo{
			Address:     env.PubKey.Address(),
			PubKey:      env.PubKey,
			VotingPower: votingPower,
		}
	}

	var applicationInfo coretypes.ApplicationInfo
	if abciInfo, err := env.ABCIInfo(ctx); err == nil {
		applicationInfo.Version = fmt.Sprint(abciInfo.Response.AppVersion)
	}

	result := &coretypes.ResultStatus{
		NodeInfo:        env.P2PTransport.NodeInfo(),
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
			MaxPeerBlockHeight:  env.BlockSyncReactor.GetMaxPeerBlockHeight(),
			CatchingUp:          env.ConsensusReactor.WaitSync(),
			TotalSyncedTime:     env.BlockSyncReactor.GetTotalSyncedTime(),
			RemainingTime:       env.BlockSyncReactor.GetRemainingSyncTime(),
		},
		ValidatorInfo: validatorInfo,
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

func (env *Environment) validatorAtHeight(h int64) *types.Validator {
	valsWithH, err := env.StateStore.LoadValidators(h)
	if err != nil {
		return nil
	}
	if env.PubKey == nil {
		return nil
	}
	privValAddress := env.PubKey.Address()

	// If we're still at height h, search in the current validator set.
	lastBlockHeight, vals := env.ConsensusState.GetValidators()
	if lastBlockHeight == h {
		for _, val := range vals {
			if bytes.Equal(val.Address, privValAddress) {
				return val
			}
		}
	}

	_, val := valsWithH.GetByAddress(privValAddress)
	return val
}
