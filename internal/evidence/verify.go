package evidence

import (
	"bytes"
	"context"
	"fmt"

	"github.com/tendermint/tendermint/types"
)

// verify verifies the evidence fully by checking:
// - It has not already been committed
// - it is sufficiently recent (MaxAge)
// - it is from a key who was a validator at the given height
// - it is internally consistent with state
// - it was properly signed by the alleged equivocator and meets the individual evidence verification requirements
//
// NOTE: Evidence may be provided that we do not have the block or validator
// set for. In these cases, we do not return a ErrInvalidEvidence as not to have
// the sending peer disconnect. All other errors are treated as invalid evidence
// (i.e. ErrInvalidEvidence).
func (evpool *Pool) verify(ctx context.Context, evidence types.Evidence) error {
	var (
		state          = evpool.State()
		height         = state.LastBlockHeight
		evidenceParams = state.ConsensusParams.Evidence
		ageNumBlocks   = height - evidence.Height()
	)

	// ensure we have the block for the evidence height
	//
	// NOTE: It is currently possible for a peer to send us evidence we're not
	// able to process because we're too far behind (e.g. syncing), so we DO NOT
	// return an invalid evidence error because we do not want the peer to
	// disconnect or signal an error in this particular case.
	blockMeta := evpool.blockStore.LoadBlockMeta(evidence.Height())
	if blockMeta == nil {
		return fmt.Errorf("failed to verify evidence; missing block for height %d", evidence.Height())
	}

	// verify the time of the evidence
	evTime := blockMeta.Header.Time
	ageDuration := state.LastBlockTime.Sub(evTime)

	// check that the evidence hasn't expired
	if ageDuration > evidenceParams.MaxAgeDuration && ageNumBlocks > evidenceParams.MaxAgeNumBlocks {
		return types.NewErrInvalidEvidence(
			evidence,
			fmt.Errorf(
				"evidence from height %d (created at: %v) is too old; min height is %d and evidence can not be older than %v",
				evidence.Height(),
				evTime,
				height-evidenceParams.MaxAgeNumBlocks,
				state.LastBlockTime.Add(evidenceParams.MaxAgeDuration),
			),
		)
	}

	// apply the evidence-specific verification logic
	switch ev := evidence.(type) {
	case *types.DuplicateVoteEvidence:
		valSet, err := evpool.stateDB.LoadValidators(evidence.Height())
		if err != nil {
			return err
		}

		if err := VerifyDuplicateVote(ev, state.ChainID, valSet); err != nil {
			return types.NewErrInvalidEvidence(evidence, err)
		}

		_, val := valSet.GetByProTxHash(ev.VoteA.ValidatorProTxHash)

		if err := ev.ValidateABCI(val, valSet, evTime); err != nil {
			ev.GenerateABCI(val, valSet, evTime)
			if addErr := evpool.addPendingEvidence(ctx, ev); addErr != nil {
				evpool.logger.Error("adding pending duplicate vote evidence failed", "err", addErr)
			}
			return err
		}

		return nil

	default:
		return types.NewErrInvalidEvidence(evidence, fmt.Errorf("unrecognized evidence type: %T", evidence))
	}
}

// VerifyDuplicateVote verifies DuplicateVoteEvidence against the state of full node. This involves the
// following checks:
//      - the validator is in the validator set at the height of the evidence
//      - the height, round, type and validator address of the votes must be the same
//      - the block ID's must be different
//      - The signatures must both be valid
func VerifyDuplicateVote(e *types.DuplicateVoteEvidence, chainID string, valSet *types.ValidatorSet) error {
	_, val := valSet.GetByProTxHash(e.VoteA.ValidatorProTxHash)
	if val == nil {
		return fmt.Errorf("protxhash %X was not a validator at height %d", e.VoteA.ValidatorProTxHash, e.Height())
	}
	proTxHash := val.ProTxHash
	pubKey := val.PubKey

	// H/R/S must be the same
	if e.VoteA.Height != e.VoteB.Height ||
		e.VoteA.Round != e.VoteB.Round ||
		e.VoteA.Type != e.VoteB.Type {
		return fmt.Errorf("h/r/s does not match: %d/%d/%v vs %d/%d/%v",
			e.VoteA.Height, e.VoteA.Round, e.VoteA.Type,
			e.VoteB.Height, e.VoteB.Round, e.VoteB.Type)
	}

	// ProTxHash must be the same
	if !bytes.Equal(e.VoteA.ValidatorProTxHash, e.VoteB.ValidatorProTxHash) {
		return fmt.Errorf("validator proTxHash do not match: %X vs %X",
			e.VoteA.ValidatorProTxHash,
			e.VoteB.ValidatorProTxHash,
		)
	}

	// BlockIDs must be different
	if e.VoteA.BlockID.Equals(e.VoteB.BlockID) {
		return fmt.Errorf(
			"block IDs are the same (%v) - not a real duplicate vote",
			e.VoteA.BlockID,
		)
	}

	// proTxHash must match proTxHash (this should already be true, sanity check)
	voteProTxHash := e.VoteA.ValidatorProTxHash
	if !bytes.Equal(voteProTxHash, proTxHash) {
		return fmt.Errorf("vote proTxHash (%X) doesn't match proTxHash (%X)",
			voteProTxHash, proTxHash)
	}

	va := e.VoteA.ToProto()
	vb := e.VoteB.ToProto()
	// Signatures must be valid
	if !pubKey.VerifySignatureDigest(types.VoteBlockSignID(chainID, va, valSet.QuorumType, valSet.QuorumHash), e.VoteA.BlockSignature) {
		return fmt.Errorf("verifying VoteA: %w", types.ErrVoteInvalidBlockSignature)
	}
	if !pubKey.VerifySignatureDigest(types.VoteBlockSignID(chainID, vb, valSet.QuorumType, valSet.QuorumHash), e.VoteB.BlockSignature) {
		return fmt.Errorf("verifying VoteB: %w", types.ErrVoteInvalidBlockSignature)
	}

	return nil
}
