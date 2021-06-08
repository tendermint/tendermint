package state

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	tmtime "github.com/tendermint/tendermint/types/time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------
// Validate block

func validateBlock(state State, block *types.Block) error {
	// Validate internal consistency.
	if err := block.ValidateBasic(); err != nil {
		return err
	}

	if block.CoreChainLock != nil {
		if err := block.CoreChainLock.ValidateBasic(); err != nil {
			return err
		}
	}

	// Validate basic info.
	if block.Version.App != state.Version.Consensus.App ||
		block.Version.Block != state.Version.Consensus.Block {
		return fmt.Errorf("wrong Block.Header.Version. Expected %v, got %v",
			state.Version.Consensus,
			block.Version,
		)
	}
	if block.ChainID != state.ChainID {
		return fmt.Errorf("wrong Block.Header.ChainID. Expected %v, got %v",
			state.ChainID,
			block.ChainID,
		)
	}
	if state.LastBlockHeight == 0 && block.Height != state.InitialHeight {
		return fmt.Errorf("wrong Block.Header.Height. Expected %v for initial block, got %v",
			block.Height, state.InitialHeight)
	}
	if state.LastBlockHeight > 0 && block.Height != state.LastBlockHeight+1 {
		return fmt.Errorf("wrong Block.Header.Height. Expected %v, got %v",
			state.LastBlockHeight+1,
			block.Height,
		)
	}
	// Validate prev block info.
	if !block.LastBlockID.Equals(state.LastBlockID) {
		return fmt.Errorf("wrong Block.Header.LastBlockID.  Expected %v, got %v",
			state.LastBlockID,
			block.LastBlockID,
		)
	}

	// Validate app info
	if !bytes.Equal(block.AppHash, state.AppHash) {
		return fmt.Errorf("wrong Block.Header.AppHash.  Expected %X, got %v",
			state.AppHash,
			block.AppHash,
		)
	}
	hashCP := types.HashConsensusParams(state.ConsensusParams)
	if !bytes.Equal(block.ConsensusHash, hashCP) {
		return fmt.Errorf("wrong Block.Header.ConsensusHash.  Expected %X, got %v",
			hashCP,
			block.ConsensusHash,
		)
	}
	if !bytes.Equal(block.LastResultsHash, state.LastResultsHash) {
		return fmt.Errorf("wrong Block.Header.LastResultsHash.  Expected %X, got %v",
			state.LastResultsHash,
			block.LastResultsHash,
		)
	}
	if !bytes.Equal(block.ValidatorsHash, state.Validators.Hash()) {
		return fmt.Errorf("wrong Block.Header.ValidatorsHash.  Expected %X, got %v",
			state.Validators.Hash(),
			block.ValidatorsHash,
		)
	}
	if !bytes.Equal(block.NextValidatorsHash, state.NextValidators.Hash()) {
		return fmt.Errorf("wrong Block.Header.NextValidatorsHash.  Expected %X, got %v",
			state.NextValidators.Hash(),
			block.NextValidatorsHash,
		)
	}

	// Validate block LastPrecommits.
	if block.Height == state.InitialHeight {
		if len(block.LastCommit.ThresholdBlockSignature) != 0 {
			return errors.New("initial block can't have ThresholdBlockSignature set")
		}
	} else {
		// fmt.Printf("validating against state with lastBlockId %s lastStateId %s\n", state.LastBlockID.String(),
		//  state.LastStateID.String())
		// LastPrecommits.Signatures length is checked in VerifyCommit.
		if err := state.LastValidators.VerifyCommit(
			state.ChainID, state.LastBlockID, state.LastStateID, block.Height-1, block.LastCommit); err != nil {
			return err
		}
	}

	// NOTE: We can't actually verify it's the right proposer because we don't
	// know what round the block was first proposed. So just check that it's
	// a legit pro_tx_hash and a known validator.
	if len(block.ProposerProTxHash) != crypto.DefaultHashSize {
		return fmt.Errorf("expected ProposerProTxHash size %d, got %d",
			crypto.DefaultHashSize,
			len(block.ProposerProTxHash),
		)
	}
	if !state.Validators.HasProTxHash(block.ProposerProTxHash) {
		return fmt.Errorf("block.Header.ProposerProTxHash %X is not a validator",
			block.ProposerProTxHash,
		)
	}

	// Validate block Time is after previous block
	switch {
	case block.Height > state.InitialHeight:
		if !block.Time.After(state.LastBlockTime) {
			return fmt.Errorf("block time %v not greater than last block time %v",
				block.Time,
				state.LastBlockTime,
			)
		}

	case block.Height == state.InitialHeight:
		genesisTime := state.LastBlockTime
		if !block.Time.Equal(genesisTime) {
			return fmt.Errorf("block time %v is not equal to genesis time %v",
				block.Time,
				genesisTime,
			)
		}

	default:
		return fmt.Errorf("block height %v lower than initial height %v",
			block.Height, state.InitialHeight)
	}

	if block.CoreChainLock != nil {
		// If there is a new Chain Lock we need to make sure the height in the header is the same as the chain lock
		if block.Header.CoreChainLockedHeight != block.CoreChainLock.CoreBlockHeight {
			return fmt.Errorf("wrong Block.Header.CoreChainLockedHeight. CoreChainLock CoreBlockHeight %d, got %d",
				block.CoreChainLock.CoreBlockHeight,
				block.Header.CoreChainLockedHeight,
			)
		}

		// We also need to make sure that the new height is superior to the old height
		if block.Header.CoreChainLockedHeight <= state.LastCoreChainLockedBlockHeight {
			return fmt.Errorf("wrong Block.Header.CoreChainLockedHeight. Previous CoreChainLockedHeight %d, got %d",
				state.LastCoreChainLockedBlockHeight,
				block.Header.CoreChainLockedHeight,
			)
		}

		// If there is no new Chain Lock we need to make sure the height has stayed the same
	} else if block.Header.CoreChainLockedHeight != state.LastCoreChainLockedBlockHeight {
		return fmt.Errorf("wrong Block.Header.CoreChainLockedHeight when no new Chain Lock. "+
			"Previous CoreChainLockedHeight %d, got %d",
			state.LastCoreChainLockedBlockHeight, block.Header.CoreChainLockedHeight)
	}

	// Check evidence doesn't exceed the limit amount of bytes.
	if max, got := state.ConsensusParams.Evidence.MaxBytes, block.Evidence.ByteSize(); got > max {
		return types.NewErrEvidenceOverflow(max, got)
	}

	return nil
}

func validateBlockTime(state State, block *types.Block) error {
	if block.Height == state.InitialHeight {
		afterLast := state.LastBlockTime.Add(5 * time.Second)
		beforeLast := state.LastBlockTime.Add(-5 * time.Second)
		if block.Time.After(afterLast) || block.Time.Before(beforeLast) {
			return fmt.Errorf("block time %v is out of window [%v, %v]",
				block.Time, afterLast, beforeLast)
		}
	} else {
		// Validate block Time is within a range of current time
		after := tmtime.Now().Add(20 * time.Second)
		before := tmtime.Now().Add(-20 * time.Second)
		if block.Time.After(after) || block.Time.Before(before) {
			return fmt.Errorf("block time %v is out of window [%v, %v]",
				block.Time, before, after)
		}
	}
	return nil
}

func validateBlockChainLock(proxyAppQueryConn proxy.AppConnQuery, state State, block *types.Block) error {
	if block.CoreChainLock != nil {
		// If there is a new Chain Lock we need to make sure the height in the header is the same as the chain lock
		if block.Header.CoreChainLockedHeight != block.CoreChainLock.CoreBlockHeight {
			return fmt.Errorf("wrong Block.Header.CoreChainLockedHeight. CoreChainLock CoreBlockHeight %d, got %d",
				block.CoreChainLock.CoreBlockHeight,
				block.Header.CoreChainLockedHeight,
			)
		}

		// We also need to make sure that the new height is superior to the old height
		if block.Header.CoreChainLockedHeight <= state.LastCoreChainLockedBlockHeight {
			return fmt.Errorf("wrong Block.Header.CoreChainLockedHeight. Previous CoreChainLockedHeight %d, got %d",
				state.LastCoreChainLockedBlockHeight,
				block.Header.CoreChainLockedHeight,
			)
		}
		coreChainLocksBytes, err := block.CoreChainLock.ToProto().Marshal()
		if err != nil {
			panic(err)
		}

		verifySignatureQueryRequest := abci.RequestQuery{
			Data: coreChainLocksBytes,
			Path: "/verify-chainlock",
		}

		// We need to query our abci application to make sure the chain lock signature is valid

		checkQuorumSignatureResponse, err := proxyAppQueryConn.QuerySync(verifySignatureQueryRequest)
		if err != nil {
			return err
		}

		if checkQuorumSignatureResponse.Code != 0 {
			return fmt.Errorf("chain Lock signature deemed invalid by abci application")
		}

		// If there is no new Chain Lock we need to make sure the height has stayed the same
	} else if block.Header.CoreChainLockedHeight != state.LastCoreChainLockedBlockHeight {
		return fmt.Errorf("wrong Block.Header.CoreChainLockedHeight when no new Chain Lock. "+
			"Previous CoreChainLockedHeight %d, got %d",
			state.LastCoreChainLockedBlockHeight, block.Header.CoreChainLockedHeight)
	}

	return nil
}
