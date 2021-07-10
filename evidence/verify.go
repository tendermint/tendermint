package evidence

import (
	"bytes"
	"fmt"

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
	default:
		return fmt.Errorf("unrecognized evidence type: %T", evidence)
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
	blockSignID := types.VoteBlockSignId(chainID, va, valSet.QuorumType, valSet.QuorumHash)
	if !pubKey.VerifySignatureDigest(blockSignID, e.VoteA.BlockSignature) {
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
