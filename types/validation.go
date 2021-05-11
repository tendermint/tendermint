package types

import (
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/crypto/batch"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmmath "github.com/tendermint/tendermint/libs/math"
)

// VerifyCommit verifies +2/3 of the set had signed the given commit.
//
// It checks all the signatures! While it's safe to exit as soon as we have
// 2/3+ signatures, doing so would impact incentivization logic in the ABCI
// application that depends on the LastCommitInfo sent in BeginBlock, which
// includes which validators signed. For instance, Gaia incentivizes proposers
// with a bonus for including more than +2/3 of the signatures.
func VerifyCommit(chainID string, vals *ValidatorSet, blockID BlockID,
	height int64, commit *Commit) error {

	// run a basic validation of the arguments
	if err := verifyBasicValsAndCommit(vals, commit, height, blockID); err != nil {
		return err
	}

	// calculate voting power needed. Note that total voting power is capped to
	// 1/8th of max int64 so this operation should never overflow
	votingPowerNeeded := vals.TotalVotingPower() * 2 / 3

	// ignore all absent signatures
	ignore := func(c CommitSig) bool { return c.Absent() }

	// only count the signatures that are for the block
	count := func(c CommitSig) bool { return c.ForBlock() }

	// attempt to batch verify
	cacheSignBytes, success, err := tryVerifyCommitBatch(
		chainID, vals, commit, votingPowerNeeded, ignore, count, true, true)
	if err != nil {
		return err
	}
	if success {
		return nil
	}

	// if verification failed or is not supported then fallback to single verification
	return verifyCommitSingle(chainID, vals, commit, votingPowerNeeded,
		cacheSignBytes, ignore, count, true, true)
}

// LIGHT CLIENT VERIFICATION METHODS

// VerifyCommitLight verifies +2/3 of the set had signed the given commit.
//
// This method is primarily used by the light client and does not check all the
// signatures.
func VerifyCommitLight(chainID string, vals *ValidatorSet, blockID BlockID,
	height int64, commit *Commit) error {

	// run a basic validation of the arguments
	if err := verifyBasicValsAndCommit(vals, commit, height, blockID); err != nil {
		return err
	}

	// calculate voting power needed
	votingPowerNeeded := vals.TotalVotingPower() * 2 / 3

	// ignore all commit signatures that are not for the block
	ignore := func(c CommitSig) bool { return !c.ForBlock() }

	// count all the remaining signatures
	count := func(c CommitSig) bool { return true }

	// attempt to batch verify
	cacheSignBytes, success, err := tryVerifyCommitBatch(
		chainID, vals, commit, votingPowerNeeded, ignore, count, false, true)
	if err != nil {
		return err
	}
	if success {
		return nil
	}

	// if verification failed or is not supported then fallback to single verification
	return verifyCommitSingle(chainID, vals, commit, votingPowerNeeded,
		cacheSignBytes, ignore, count, false, true)

}

// VerifyCommitLightTrusting verifies that trustLevel of the validator set signed
// this commit.
//
// NOTE the given validators do not necessarily correspond to the validator set
// for this commit, but there may be some intersection.
//
// This method is primarily used by the light client and does not check all the
// signatures.
func VerifyCommitLightTrusting(chainID string, vals *ValidatorSet, commit *Commit, trustLevel tmmath.Fraction) error {
	// sanity checks
	if vals == nil {
		return errors.New("nil validator set")
	}
	if trustLevel.Denominator == 0 {
		return errors.New("trustLevel has zero Denominator")
	}
	if commit == nil {
		return errors.New("nil commit")
	}

	// safely calculate voting power needed.
	totalVotingPowerMulByNumerator, overflow := safeMul(vals.TotalVotingPower(), int64(trustLevel.Numerator))
	if overflow {
		return errors.New("int64 overflow while calculating voting power needed. please provide smaller trustLevel numerator")
	}
	votingPowerNeeded := totalVotingPowerMulByNumerator / int64(trustLevel.Denominator)

	// ignore all commit signatures that are not for the block
	ignore := func(c CommitSig) bool { return !c.ForBlock() }

	// count all the remaining signatures
	count := func(c CommitSig) bool { return true }

	// attempt to batch verify commit. As the validator set doesn't necessarily
	// correspond with the validator set that signed the block we need to look
	// up by address rather than index.
	cacheSignBytes, success, err := tryVerifyCommitBatch(
		chainID, vals, commit, votingPowerNeeded, ignore, count, false, false)
	if err != nil {
		return err
	}
	if success {
		return nil
	}

	// attempt with single verification
	return verifyCommitSingle(chainID, vals, commit, votingPowerNeeded,
		cacheSignBytes, ignore, count, false, false)
}

// ValidateHash returns an error if the hash is not empty, but its
// size != tmhash.Size.
func ValidateHash(h []byte) error {
	if len(h) > 0 && len(h) != tmhash.Size {
		return fmt.Errorf("expected size to be %d bytes, got %d bytes",
			tmhash.Size,
			len(h),
		)
	}
	return nil
}

// Batch verification

// tryVerifyCommitBatch attempts to batch verify. If it is not supported or
// verification fails it returns false. If there is an error in the signatures
// or the way that they are counted an error is returned. A cache of all the
// commits in byte form is returned in case it needs to be used again for single
// verification
func tryVerifyCommitBatch(
	chainID string,
	vals *ValidatorSet,
	commit *Commit,
	votingPowerNeeded int64,
	ignoreSig func(CommitSig) bool,
	countSig func(CommitSig) bool,
	countAllSignatures bool,
	lookUpByIndex bool,
) (map[string][]byte, bool, error) {
	var (
		val                *Validator
		valIdx             int32
		seenVals                 = make(map[int32]int, len(commit.Signatures))
		talliedVotingPower int64 = 0
		// we keep a cache of the signed bytes to make it quicker to verify
		// individually if we need to
		cacheSignBytes = make(map[string][]byte, len(commit.Signatures))
	)
	// attempt to create a batch verifier
	bv, ok := batch.CreateBatchVerifier(vals.GetProposer().PubKey)
	// check if batch verification is supported
	if !ok || len(commit.Signatures) < 2 {
		return cacheSignBytes, false, nil
	}

	for idx, commitSig := range commit.Signatures {
		// skip over signatures that should be ignored
		if ignoreSig(commitSig) {
			continue
		}

		// If the vals and commit have a 1-to-1 correspondance we can retrieve
		// them by index else we need to retrieve them by address
		if lookUpByIndex {
			val = vals.Validators[idx]
		} else {
			valIdx, val = vals.GetByAddress(commitSig.ValidatorAddress)

			// if the signature doesn't belong to anyone in the validator set
			// then we just skip over it
			if val == nil {
				continue
			}

			// because we are getting validators by address we need to make sure
			// that the same validator doesn't commit twice
			if firstIndex, ok := seenVals[valIdx]; ok {
				secondIndex := idx
				return cacheSignBytes, false, fmt.Errorf("double vote from %v (%d and %d)", val, firstIndex, secondIndex)
			}
			seenVals[valIdx] = idx
		}

		// Validate signature.
		voteSignBytes := commit.VoteSignBytes(chainID, int32(idx))
		// cache the signBytes in case batch verification fails
		cacheSignBytes[string(val.PubKey.Bytes())] = voteSignBytes

		// add the key, sig and message to the verifier
		if err := bv.Add(val.PubKey, voteSignBytes, commitSig.Signature); err != nil {
			return cacheSignBytes, false, err
		}

		// If this signature counts then add the voting power of the validator
		// to the tally
		if countSig(commitSig) {
			talliedVotingPower += val.VotingPower
		}

		// if we don't need to verify all signatures and already have sufficient
		// voting power we can break from batching and verify all the signatures
		if !countAllSignatures && talliedVotingPower > votingPowerNeeded {
			break
		}
	}

	// ensure that we have batched together enough signatures to exceed the
	// voting power needed else there is no need to even verify
	if got, needed := talliedVotingPower, votingPowerNeeded; got <= needed {
		return cacheSignBytes, false, ErrNotEnoughVotingPowerSigned{Got: got, Needed: needed}
	}

	// attempt to verify the batch. If this fails, fall back to single
	// verification
	if bv.Verify() {
		// success
		return cacheSignBytes, true, nil
	}

	// verification failed
	return cacheSignBytes, false, nil
}

// Single Verification

// verifyCommitSingle single verifies commits.
// If a key does not support batch verification, or batch verification fails this will be used
// This method is used to check all the signatures included in a commit.
// It is used in consensus for validating a block LastCommit.
// CONTRACT: both commit and validator set should have passed validate basic
func verifyCommitSingle(
	chainID string,
	vals *ValidatorSet,
	commit *Commit,
	votingPowerNeeded int64,
	cachedVals map[string][]byte,
	ignoreSig func(CommitSig) bool,
	countSig func(CommitSig) bool,
	countAllSignatures bool,
	lookUpByIndex bool,
) error {
	var (
		val                *Validator
		valIdx             int32
		seenVals                 = make(map[int32]int, len(commit.Signatures))
		talliedVotingPower int64 = 0
		voteSignBytes      []byte
	)
	for idx, commitSig := range commit.Signatures {
		if ignoreSig(commitSig) {
			continue
		}

		// If the vals and commit have a 1-to-1 correspondance we can retrieve
		// them by index else we need to retrieve them by address
		if lookUpByIndex {
			val = vals.Validators[idx]
		} else {
			valIdx, val = vals.GetByAddress(commitSig.ValidatorAddress)

			// if the signature doesn't belong to anyone in the validator set
			// then we just skip over it
			if val == nil {
				continue
			}

			// because we are getting validators by address we need to make sure
			// that the same validator doesn't commit twice
			if firstIndex, ok := seenVals[valIdx]; ok {
				secondIndex := idx
				return fmt.Errorf("double vote from %v (%d and %d)", val, firstIndex, secondIndex)
			}
			seenVals[valIdx] = idx
		}

		// Check if we have the validator in the cache
		if cachedVote, ok := cachedVals[string(val.PubKey.Bytes())]; !ok {
			voteSignBytes = commit.VoteSignBytes(chainID, int32(idx))
		} else {
			voteSignBytes = cachedVote
		}

		if !val.PubKey.VerifySignature(voteSignBytes, commitSig.Signature) {
			return fmt.Errorf("wrong signature (#%d): %X", idx, commitSig.Signature)
		}

		// If this signature counts then add the voting power of the validator
		// to the tally
		if countSig(commitSig) {
			talliedVotingPower += val.VotingPower
		}

		// check if we have enough signatures and can thus exit early
		if !countAllSignatures && talliedVotingPower > votingPowerNeeded {
			return nil
		}
	}

	if got, needed := talliedVotingPower, votingPowerNeeded; got <= needed {
		return ErrNotEnoughVotingPowerSigned{Got: got, Needed: needed}
	}

	return nil
}

func verifyBasicValsAndCommit(vals *ValidatorSet, commit *Commit, height int64, blockID BlockID) error {
	if vals == nil {
		return errors.New("nil validator set")
	}

	if commit == nil {
		return errors.New("nil commit")
	}

	if vals.Size() != len(commit.Signatures) {
		return NewErrInvalidCommitSignatures(vals.Size(), len(commit.Signatures))
	}

	// Validate Height and BlockID.
	if height != commit.Height {
		return NewErrInvalidCommitHeight(height, commit.Height)
	}
	if !blockID.Equals(commit.BlockID) {
		return fmt.Errorf("invalid commit -- wrong block ID: want %v, got %v",
			blockID, commit.BlockID)
	}

	return nil
}
