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

	votingPowerNeeded := vals.TotalVotingPower() * 2 / 3
	var (
		talliedVotingPower int64 = 0
		cacheSignBytes           = make(map[string][]byte, len(commit.Signatures))
	)

	bv, ok := batch.CreateBatchVerifier(vals.GetProposer().PubKey)
	if ok && len(commit.Signatures) > 1 {
		for idx, commitSig := range commit.Signatures {
			if commitSig.Absent() {
				continue // OK, some signatures can be absent.
			}

			// The vals and commit have a 1-to-1 correspondance.
			// This means we don't need the validator address or to do any lookup.
			val := vals.Validators[idx]

			// Validate signature.
			voteSignBytes := commit.VoteSignBytes(chainID, int32(idx))
			// cache the signBytes in case batch verification fails
			cacheSignBytes[string(val.PubKey.Bytes())] = voteSignBytes
			// add the key, sig and message to the verifier
			if err := bv.Add(val.PubKey, voteSignBytes, commitSig.Signature); err != nil {
				return err
			}

			// Good!
			if commitSig.ForBlock() {
				talliedVotingPower += val.VotingPower
			}
		}

		// ensure that we have batched together enough signatures to exceed the
		// voting power needed
		if got, needed := talliedVotingPower, votingPowerNeeded; got <= needed {
			return ErrNotEnoughVotingPowerSigned{Got: got, Needed: needed}
		}

		// attempt to verify the batch. If this fails, fall back to single
		// verification
		if bv.Verify() {
			return nil
		}
	}

	// attempt with single verification
	return verifyCommitSingle(chainID, vals, commit, votingPowerNeeded, cacheSignBytes)
}

// LIGHT CLIENT VERIFICATION METHODS

// VerifyCommitLight verifies +2/3 of the set had signed the given commit.
//
// This method is primarily used by the light client and does not check all the
// signatures.
func VerifyCommitLight(chainID string, vals *ValidatorSet, blockID BlockID,
	height int64, commit *Commit) error {
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

	talliedVotingPower := int64(0)
	votingPowerNeeded := vals.TotalVotingPower() * 2 / 3
	cacheSignBytes := make(map[string][]byte, len(commit.Signatures))

	// need to check if batch verification is supported
	// if batch is supported and the there are more than x key(s) run batch, otherwise run single.
	// if batch verification fails reset tally votes to 0 and single verify until we have 2/3+
	// check if the key supports batch verification
	bv, ok := batch.CreateBatchVerifier(vals.GetProposer().PubKey)
	if ok && len(commit.Signatures) > 1 {
		for idx, commitSig := range commit.Signatures {
			// No need to verify absent or nil votes.
			if !commitSig.ForBlock() {
				continue
			}

			// The vals and commit have a 1-to-1 correspondance.
			// This means we don't need the validator address or to do any lookup.
			val := vals.Validators[idx]
			voteSignBytes := commit.VoteSignBytes(chainID, int32(idx))
			cacheSignBytes[string(val.PubKey.Bytes())] = voteSignBytes
			// add the key, sig and message to the verifier
			if err := bv.Add(val.PubKey, voteSignBytes, commitSig.Signature); err != nil {
				return err
			}

			// add the voting power. If we have more than needed, then we can
			// stop adding to the batch and verify
			talliedVotingPower += val.VotingPower
			if talliedVotingPower > votingPowerNeeded {
				break
			}
		}
		// ensure that we have batched together enough signatures to exceed the
		// voting power needed
		if got, needed := talliedVotingPower, votingPowerNeeded; got <= needed {
			return ErrNotEnoughVotingPowerSigned{Got: got, Needed: needed}
		}

		// attempt to bactch verify. If this fails then fall back to single
		// verification.
		if bv.Verify() {
			// success
			return nil
		}
	}

	// attempt with single verification
	return verifyCommitLightSingle(
		chainID, vals, commit, votingPowerNeeded, cacheSignBytes)

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

	var (
		talliedVotingPower int64
		seenVals           = make(map[int32]int, len(commit.Signatures)) // validator index -> commit index
		cacheSignBytes     = make(map[string][]byte, len(commit.Signatures))
	)

	// Safely calculate voting power needed.
	totalVotingPowerMulByNumerator, overflow := safeMul(vals.TotalVotingPower(), int64(trustLevel.Numerator))
	if overflow {
		return errors.New("int64 overflow while calculating voting power needed. please provide smaller trustLevel numerator")
	}
	votingPowerNeeded := totalVotingPowerMulByNumerator / int64(trustLevel.Denominator)

	bv, ok := batch.CreateBatchVerifier(vals.GetProposer().PubKey)
	if ok && len(commit.Signatures) > 1 {
		for idx, commitSig := range commit.Signatures {
			// No need to verify absent or nil votes.
			if !commitSig.ForBlock() {
				continue
			}

			// We don't know the validators that committed this block, so we have to
			// check for each vote if its validator is already known.
			valIdx, val := vals.GetByAddress(commitSig.ValidatorAddress)

			if val != nil {
				// check for double vote of validator on the same commit
				if firstIndex, ok := seenVals[valIdx]; ok {
					secondIndex := idx
					return fmt.Errorf("double vote from %v (%d and %d)", val, firstIndex, secondIndex)
				}
				seenVals[valIdx] = idx

				// Validate signature.
				voteSignBytes := commit.VoteSignBytes(chainID, int32(idx))
				// cache the signed bytes in case we fail verification
				cacheSignBytes[string(val.PubKey.Bytes())] = voteSignBytes
				// if batch verification is supported add the key, sig and message to the verifier
				if err := bv.Add(val.PubKey, voteSignBytes, commitSig.Signature); err != nil {
					return err
				}

				// add the voting power of the validator. If we exceed the total
				// voting power needed then we can stop adding signatures and
				// run batch verification
				talliedVotingPower += val.VotingPower
				if talliedVotingPower > votingPowerNeeded {
					break
				}
			}
		}
		// ensure that we have batched together enough signatures to exceed the
		// voting power needed
		if got, needed := talliedVotingPower, votingPowerNeeded; got <= needed {
			return ErrNotEnoughVotingPowerSigned{Got: got, Needed: needed}
		}

		// attempt to bactch verify. If this fails then fall back to single
		// verification.
		if bv.Verify() {
			// success
			return nil
		}
	}

	// attempt with single verification
	return verifyCommitLightTrustingSingle(
		chainID, vals, commit, votingPowerNeeded, cacheSignBytes)
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

// Single Verification

// verifyCommitSingle single verifies commits.
// If a key does not support batch verification, or batch verification fails this will be used
// This method is used to check all the signatures included in a commit.
// It is used in consensus for validating a block LastCommit.
func verifyCommitSingle(chainID string, vals *ValidatorSet, commit *Commit,
	votingPowerNeeded int64, cachedVals map[string][]byte) error {
	var talliedVotingPower int64 = 0
	for idx, commitSig := range commit.Signatures {
		if commitSig.Absent() {
			continue // OK, some signatures can be absent.
		}

		var voteSignBytes []byte
		val := vals.Validators[idx]

		// Check if we have the validator in the cache
		if val, ok := cachedVals[string(val.PubKey.Bytes())]; !ok {
			voteSignBytes = commit.VoteSignBytes(chainID, int32(idx))
		} else {
			voteSignBytes = val
		}

		if !val.PubKey.VerifySignature(voteSignBytes, commitSig.Signature) {
			return fmt.Errorf("wrong signature (#%d): %X", idx, commitSig.Signature)
		}

		// Good!
		if commitSig.ForBlock() {
			talliedVotingPower += val.VotingPower
		}
	}

	if got, needed := talliedVotingPower, votingPowerNeeded; got <= needed {
		return ErrNotEnoughVotingPowerSigned{Got: got, Needed: needed}
	}

	return nil
}

// verifyCommitLightSingle single verifies commits.
// If a key does not support batch verification, or batch verification fails this will be used
// This method is used for light client and block sync verification, it will only check 2/3+ signatures
func verifyCommitLightSingle(
	chainID string, vals *ValidatorSet, commit *Commit, votingPowerNeeded int64,
	cachedVals map[string][]byte) error {
	var talliedVotingPower int64 = 0
	for idx, commitSig := range commit.Signatures {
		// No need to verify absent or nil votes.
		if !commitSig.ForBlock() {
			continue
		}

		// The vals and commit have a 1-to-1 correspondance.
		// This means we don't need the validator address or to do any lookup.
		var voteSignBytes []byte
		val := vals.Validators[idx]

		// Check if we have the validators vote in the cache
		if val, ok := cachedVals[string(val.PubKey.Bytes())]; !ok {
			voteSignBytes = commit.VoteSignBytes(chainID, int32(idx))
		} else {
			voteSignBytes = val
		}
		// Validate signature.
		if !val.PubKey.VerifySignature(voteSignBytes, commitSig.Signature) {
			return fmt.Errorf("wrong signature (#%d): %X", idx, commitSig.Signature)
		}

		talliedVotingPower += val.VotingPower

		// return as soon as +2/3 of the signatures are verified
		if talliedVotingPower > votingPowerNeeded {
			return nil
		}
	}
	return ErrNotEnoughVotingPowerSigned{Got: talliedVotingPower, Needed: votingPowerNeeded}
}

// verifyCommitLightTrustingSingle single verifies commits
// If a key does not support batch verification, or batch verification fails this will be used
// This method is used for light clients, it only checks 2/3+ of the signatures
func verifyCommitLightTrustingSingle(
	chainID string, vals *ValidatorSet, commit *Commit, votingPowerNeeded int64,
	cachedVals map[string][]byte) error {
	var (
		seenVals                 = make(map[int32]int, len(commit.Signatures))
		talliedVotingPower int64 = 0
	)
	for idx, commitSig := range commit.Signatures {
		// No need to verify absent or nil votes.
		if !commitSig.ForBlock() {
			continue
		}

		var voteSignBytes []byte

		// We don't know the validators that committed this block, so we have to
		// check for each vote if its validator is already known.
		valIdx, val := vals.GetByAddress(commitSig.ValidatorAddress)

		if val != nil {
			// check for double vote of validator on the same commit
			if firstIndex, ok := seenVals[valIdx]; ok {
				secondIndex := idx
				return fmt.Errorf("double vote from %v (%d and %d)", val, firstIndex, secondIndex)
			}
			seenVals[valIdx] = idx

			// Retrieve the signature from the cache if it's there
			if val, ok := cachedVals[string(val.PubKey.Bytes())]; !ok {
				voteSignBytes = commit.VoteSignBytes(chainID, int32(idx))
			} else {
				voteSignBytes = val
			}
			if !val.PubKey.VerifySignature(voteSignBytes, commitSig.Signature) {
				return fmt.Errorf("wrong signature (#%d): %X", idx, commitSig.Signature)
			}

			talliedVotingPower += val.VotingPower
			if talliedVotingPower > votingPowerNeeded {
				return nil
			}
		}
	}

	return ErrNotEnoughVotingPowerSigned{Got: talliedVotingPower, Needed: votingPowerNeeded}
}
