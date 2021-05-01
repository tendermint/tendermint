package evidence

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/types"
)

// verify verifies the evidence fully by checking:
// - It has not already been committed
// - it is sufficiently recent (MaxAge)
// - it is from a key who was a validator at the given height
// - it is internally consistent with state
// - it was properly signed by the alleged equivocator and meets the individual evidence verification requirements
func (evpool *Pool) verify(evidence types.Evidence) error {
	var (
		state          = evpool.State()
		height         = state.LastBlockHeight
		evidenceParams = state.ConsensusParams.Evidence
		ageNumBlocks   = height - evidence.Height()
	)

	// verify the time of the evidence
	blockMeta := evpool.blockStore.LoadBlockMeta(evidence.Height())
	if blockMeta == nil {
		return fmt.Errorf("don't have header #%d %v", evidence.Height(), evpool.blockStore)
	}
	evTime := blockMeta.Header.Time
	if evidence.Time() != evTime {
		return fmt.Errorf("evidence has a different time to the block it is associated with (%v != %v)",
			evidence.Time(), evTime)
	}
	ageDuration := state.LastBlockTime.Sub(evTime)

	// check that the evidence hasn't expired
	if ageDuration > evidenceParams.MaxAgeDuration && ageNumBlocks > evidenceParams.MaxAgeNumBlocks {
		return fmt.Errorf(
			"evidence from height %d (created at: %v) is too old; min height is %d and evidence can not be older than %v",
			evidence.Height(),
			evTime,
			height-evidenceParams.MaxAgeNumBlocks,
			state.LastBlockTime.Add(evidenceParams.MaxAgeDuration),
		)
	}

	// apply the evidence-specific verification logic
	switch ev := evidence.(type) {
	case *types.DuplicateVoteEvidence:
		valSet, err := evpool.stateDB.LoadValidators(evidence.Height())
		if err != nil {
			return err
		}
		return VerifyDuplicateVote(ev, state.ChainID, valSet)

	case *types.LightClientAttackEvidence:
		commonHeader, err := getSignedHeader(evpool.blockStore, evidence.Height())
		if err != nil {
			return err
		}
		commonVals, err := evpool.stateDB.LoadValidators(evidence.Height())
		if err != nil {
			return err
		}
		trustedHeader := commonHeader
		// in the case of lunatic the trusted header is different to the common header
		if evidence.Height() != ev.ConflictingBlock.Height {
			trustedHeader, err = getSignedHeader(evpool.blockStore, ev.ConflictingBlock.Height)
			if err != nil {
				// FIXME: This multi step process is a bit unergonomic. We may want to consider a more efficient process
				// that doesn't require as much io and is atomic.

				// If the node doesn't have a block at the height of the conflicting block, then this could be
				// a forward lunatic attack. Thus the node must get the latest height it has
				latestHeight := evpool.blockStore.Height()
				trustedHeader, err = getSignedHeader(evpool.blockStore, latestHeight)
				if err != nil {
					return err
				}
				if trustedHeader.Time.Before(ev.ConflictingBlock.Time) {
					return fmt.Errorf("latest block time (%v) is before conflicting block time (%v)",
						trustedHeader.Time, ev.ConflictingBlock.Time,
					)
				}
			}
		}

		err = VerifyLightClientAttack(ev, commonHeader, trustedHeader, commonVals, state.LastBlockTime,
			state.ConsensusParams.Evidence.MaxAgeDuration)
		if err != nil {
			return err
		}
		// find out what type of attack this was and thus extract the malicious validators. Note in the case of an
		// Amnesia attack we don't have any malicious validators.
		validators := ev.GetByzantineValidators(commonVals, trustedHeader)
		// ensure this matches the validators that are listed in the evidence. They should be ordered based on power.
		if validators == nil && ev.ByzantineValidators != nil {
			return fmt.Errorf("expected nil validators from an amnesia light client attack but got %d",
				len(ev.ByzantineValidators))
		}

		if exp, got := len(validators), len(ev.ByzantineValidators); exp != got {
			return fmt.Errorf("expected %d byzantine validators from evidence but got %d",
				exp, got)
		}

		// ensure that both validator arrays are in the same order
		sort.Sort(types.ValidatorsByVotingPower(ev.ByzantineValidators))

		for idx, val := range validators {
			if !bytes.Equal(ev.ByzantineValidators[idx].Address, val.Address) {
				return fmt.Errorf("evidence contained a different byzantine validator address to the one we were expecting."+
					"Expected %v, got %v", val.Address, ev.ByzantineValidators[idx].Address)
			}
			if ev.ByzantineValidators[idx].VotingPower != val.VotingPower {
				return fmt.Errorf("evidence contained a byzantine validator with a different power to the one we were expecting."+
					"Expected %d, got %d", val.VotingPower, ev.ByzantineValidators[idx].VotingPower)
			}
		}

		return nil
	default:
		return fmt.Errorf("unrecognized evidence type: %T", evidence)
	}

}

// VerifyLightClientAttack verifies LightClientAttackEvidence against the state of the full node. This involves
// the following checks:
//     - the common header from the full node has at least 1/3 voting power which is also present in
//       the conflicting header's commit
//     - 2/3+ of the conflicting validator set correctly signed the conflicting block
//     - the nodes trusted header at the same height as the conflicting header has a different hash
//
// CONTRACT: must run ValidateBasic() on the evidence before verifying
//           must check that the evidence has not expired (i.e. is outside the maximum age threshold)
func VerifyLightClientAttack(e *types.LightClientAttackEvidence, commonHeader, trustedHeader *types.SignedHeader,
	commonVals *types.ValidatorSet, now time.Time, trustPeriod time.Duration) error {
	// In the case of lunatic attack there will be a different commonHeader height. Therefore the node perform a single
	// verification jump between the common header and the conflicting one
	if commonHeader.Height != e.ConflictingBlock.Height {
		err := commonVals.VerifyCommitLightTrusting(trustedHeader.ChainID, e.ConflictingBlock.Commit, light.DefaultTrustLevel)
		if err != nil {
			return fmt.Errorf("skipping verification of conflicting block failed: %w", err)
		}

		// In the case of equivocation and amnesia we expect all header hashes to be correctly derived
	} else if isInvalidHeader(trustedHeader.Header, e.ConflictingBlock.Header) {
		return errors.New("common height is the same as conflicting block height so expected the conflicting" +
			" block to be correctly derived yet it wasn't")
	}

	// Verify that the 2/3+ commits from the conflicting validator set were for the conflicting header
	if err := e.ConflictingBlock.ValidatorSet.VerifyCommitLight(trustedHeader.ChainID, e.ConflictingBlock.Commit.BlockID,
		e.ConflictingBlock.Commit.StateID, e.ConflictingBlock.Height, e.ConflictingBlock.Commit); err != nil {
		return fmt.Errorf("invalid commit from conflicting block: %w", err)
	}

	// Assert the correct amount of voting power of the validator set
	if evTotal, valsTotal := e.TotalVotingPower, commonVals.TotalVotingPower(); evTotal != valsTotal {
		return fmt.Errorf("total voting power from the evidence and our validator set does not match (%d != %d)",
			evTotal, valsTotal)
	}

	// check in the case of a forward lunatic attack that monotonically increasing time has been violated
	if e.ConflictingBlock.Height > trustedHeader.Height && e.ConflictingBlock.Time.After(trustedHeader.Time) {
		return fmt.Errorf("conflicting block doesn't violate monotonically increasing time (%v is after %v)",
			e.ConflictingBlock.Time, trustedHeader.Time,
		)

		// In all other cases check that the hashes of the conflicting header and the trusted header are different
	} else if bytes.Equal(trustedHeader.Hash(), e.ConflictingBlock.Hash()) {
		return fmt.Errorf("trusted header hash matches the evidence's conflicting header hash: %X",
			trustedHeader.Hash())
	}

	return nil
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
		return fmt.Errorf("proTxHash %X was not a validator at height %d", e.VoteA.ValidatorProTxHash, e.Height())
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

	// ProTxHashes must be the same
	if !bytes.Equal(e.VoteA.ValidatorProTxHash, e.VoteB.ValidatorProTxHash) {
		return fmt.Errorf("validator proTxHashes do not match: %X vs %X",
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

	// proTxHash must match address (this should already be true, sanity check)
	if !bytes.Equal(proTxHash, e.VoteA.ValidatorProTxHash) {
		return fmt.Errorf("proTxHash (%X) doesn't match pubkey (%v)", e.VoteA.ValidatorProTxHash, proTxHash)
	}

	// validator voting power and total voting power must match
	if val.VotingPower != e.ValidatorPower {
		return fmt.Errorf("validator power from evidence and our validator set does not match (%d != %d)",
			e.ValidatorPower, val.VotingPower)
	}
	if valSet.TotalVotingPower() != e.TotalVotingPower {
		return fmt.Errorf("total voting power from the evidence and our validator set does not match (%d != %d)",
			e.TotalVotingPower, valSet.TotalVotingPower())
	}

	va := e.VoteA.ToProto()
	vb := e.VoteB.ToProto()
	// Signatures must be valid
	blockSignId := types.VoteBlockSignId(chainID, va, valSet.QuorumType, valSet.QuorumHash)
	if !pubKey.VerifySignatureDigest(blockSignId, e.VoteA.BlockSignature) {
		return fmt.Errorf("verifying VoteA: %s", types.ErrVoteInvalidBlockSignature.Error())
	}
	if !pubKey.VerifySignatureDigest(types.VoteBlockSignId(chainID, vb, valSet.QuorumType, valSet.QuorumHash), e.VoteB.BlockSignature) {
		return fmt.Errorf("verifying VoteB: %s", types.ErrVoteInvalidStateSignature.Error())
	}

	return nil
}

func getSignedHeader(blockStore BlockStore, height int64) (*types.SignedHeader, error) {
	blockMeta := blockStore.LoadBlockMeta(height)
	if blockMeta == nil {
		return nil, fmt.Errorf("don't have header at height #%d", height)
	}
	commit := blockStore.LoadBlockCommit(height)
	if commit == nil {
		return nil, fmt.Errorf("don't have commit at height #%d", height)
	}
	return &types.SignedHeader{
		Header: &blockMeta.Header,
		Commit: commit,
	}, nil
}

// isInvalidHeader takes a trusted header and matches it againt a conflicting header
// to determine whether the conflicting header was the product of a valid state transition
// or not. If it is then all the deterministic fields of the header should be the same.
// If not, it is an invalid header and constitutes a lunatic attack.
func isInvalidHeader(trusted, conflicting *types.Header) bool {
	return !bytes.Equal(trusted.ValidatorsHash, conflicting.ValidatorsHash) ||
		!bytes.Equal(trusted.NextValidatorsHash, conflicting.NextValidatorsHash) ||
		!bytes.Equal(trusted.ConsensusHash, conflicting.ConsensusHash) ||
		!bytes.Equal(trusted.AppHash, conflicting.AppHash) ||
		!bytes.Equal(trusted.LastResultsHash, conflicting.LastResultsHash)
}
