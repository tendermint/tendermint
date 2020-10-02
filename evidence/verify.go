package evidence

import (
	"bytes"
	"errors"
	"fmt"
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
func (evpool *Pool) verify(evidence types.Evidence) (*info, error) {
	var (
		state          = evpool.State()
		height         = state.LastBlockHeight
		evidenceParams = state.ConsensusParams.Evidence
		ageNumBlocks   = height - evidence.Height()
	)

	// check that the evidence isn't already committed
	if evpool.isCommitted(evidence) {
		return nil, errors.New("evidence was already committed")
	}

	// verify the time of the evidence
	blockMeta := evpool.blockStore.LoadBlockMeta(evidence.Height())
	if blockMeta == nil {
		return nil, fmt.Errorf("don't have header at height #%d", evidence.Height())
	}
	evTime := blockMeta.Header.Time
	ageDuration := state.LastBlockTime.Sub(evTime)

	// check that the evidence hasn't expired
	if ageDuration > evidenceParams.MaxAgeDuration && ageNumBlocks > evidenceParams.MaxAgeNumBlocks {
		return nil, fmt.Errorf(
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
			return nil, err
		}
		err = VerifyDuplicateVote(ev, state.ChainID, valSet)
		if err != nil {
			return nil, fmt.Errorf("verifying duplicate vote evidence: %w", err)
		}

		_, val := valSet.GetByAddress(ev.VoteA.ValidatorAddress)

		return &info{
			Evidence:         evidence,
			Time:             evTime,
			Validators:       []*types.Validator{val}, // just a single validator for duplicate vote evidence
			TotalVotingPower: valSet.TotalVotingPower(),
		}, nil

	case *types.LightClientAttackEvidence:
		commonHeader, err := getSignedHeader(evpool.blockStore, evidence.Height())
		if err != nil {
			return nil, err
		}
		commonVals, err := evpool.stateDB.LoadValidators(evidence.Height())
		if err != nil {
			return nil, err
		}
		trustedHeader := commonHeader
		// in the case of lunatic the trusted header is different to the common header
		if evidence.Height() != ev.ConflictingBlock.Height {
			trustedHeader, err = getSignedHeader(evpool.blockStore, ev.ConflictingBlock.Height)
			if err != nil {
				return nil, err
			}
		}

		err = VerifyLightClientAttack(ev, commonHeader, trustedHeader, commonVals, state.LastBlockTime,
			state.ConsensusParams.Evidence.MaxAgeDuration)
		if err != nil {
			return nil, err
		}
		// find out what type of attack this was and thus extract the malicious validators. Note in the case of an
		// Amnesia attack we don't have any malicious validators.
		validators, attackType := getMaliciousValidators(ev, commonVals, trustedHeader)
		totalVotingPower := ev.ConflictingBlock.ValidatorSet.TotalVotingPower()
		if attackType == lunaticType {
			totalVotingPower = commonVals.TotalVotingPower()
		}

		return &info{
			Evidence:         evidence,
			Time:             evTime,
			Validators:       validators,
			TotalVotingPower: totalVotingPower,
		}, nil
	default:
		return nil, fmt.Errorf("unrecognized evidence type: %T", evidence)
	}
}

// VerifyLightClientAttack verifies LightClientAttackEvidence against the state of the full node. This involves
// the following checks:
//     - the common header from the full node has at least 1/3 voting power which is also present in
//       the conflicting header's commit
//     - the nodes trusted header at the same height as the conflicting header has a different hash
func VerifyLightClientAttack(e *types.LightClientAttackEvidence, commonHeader, trustedHeader *types.SignedHeader,
	commonVals *types.ValidatorSet, now time.Time, trustPeriod time.Duration) error {
	// In the case of lunatic attack we need to perform a single verification jump between the
	// common header and the conflicting one
	if commonHeader.Height != trustedHeader.Height {
		err := light.Verify(commonHeader, commonVals, e.ConflictingBlock.SignedHeader, e.ConflictingBlock.ValidatorSet,
			trustPeriod, now, 0*time.Second, light.DefaultTrustLevel)
		if err != nil {
			return fmt.Errorf("skipping verification from common to conflicting header failed: %w", err)
		}
	} else {
		// in the case of equivocation and amnesia we expect some header hashes to be correctly derived
		if isInvalidHeader(trustedHeader.Header, e.ConflictingBlock.Header) {
			return errors.New("common height is the same as conflicting block height so expected the conflicting" +
				" block to be correctly derived yet it wasn't")
		}
		// ensure that 2/3 of the validator set did vote for this block
		if err := e.ConflictingBlock.ValidatorSet.VerifyCommitLight(trustedHeader.ChainID, e.ConflictingBlock.Commit.BlockID,
			e.ConflictingBlock.Height, e.ConflictingBlock.Commit); err != nil {
			return fmt.Errorf("invalid commit from conflicting block: %w", err)
		}
	}

	if bytes.Equal(trustedHeader.Hash(), e.ConflictingBlock.Hash()) {
		return fmt.Errorf("trusted header hash matches the evidence conflicting header hash: %X",
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
	_, val := valSet.GetByAddress(e.VoteA.ValidatorAddress)
	if val == nil {
		return fmt.Errorf("address %X was not a validator at height %d", e.VoteA.ValidatorAddress, e.Height())
	}
	pubKey := val.PubKey

	// H/R/S must be the same
	if e.VoteA.Height != e.VoteB.Height ||
		e.VoteA.Round != e.VoteB.Round ||
		e.VoteA.Type != e.VoteB.Type {
		return fmt.Errorf("h/r/s does not match: %d/%d/%v vs %d/%d/%v",
			e.VoteA.Height, e.VoteA.Round, e.VoteA.Type,
			e.VoteB.Height, e.VoteB.Round, e.VoteB.Type)
	}

	// Address must be the same
	if !bytes.Equal(e.VoteA.ValidatorAddress, e.VoteB.ValidatorAddress) {
		return fmt.Errorf("validator addresses do not match: %X vs %X",
			e.VoteA.ValidatorAddress,
			e.VoteB.ValidatorAddress,
		)
	}

	// BlockIDs must be different
	if e.VoteA.BlockID.Equals(e.VoteB.BlockID) {
		return fmt.Errorf(
			"block IDs are the same (%v) - not a real duplicate vote",
			e.VoteA.BlockID,
		)
	}

	// pubkey must match address (this should already be true, sanity check)
	addr := e.VoteA.ValidatorAddress
	if !bytes.Equal(pubKey.Address(), addr) {
		return fmt.Errorf("address (%X) doesn't match pubkey (%v - %X)",
			addr, pubKey, pubKey.Address())
	}
	va := e.VoteA.ToProto()
	vb := e.VoteB.ToProto()
	// Signatures must be valid
	if !pubKey.VerifySignature(types.VoteSignBytes(chainID, va), e.VoteA.Signature) {
		return fmt.Errorf("verifying VoteA: %w", types.ErrVoteInvalidSignature)
	}
	if !pubKey.VerifySignature(types.VoteSignBytes(chainID, vb), e.VoteB.Signature) {
		return fmt.Errorf("verifying VoteB: %w", types.ErrVoteInvalidSignature)
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

// getMaliciousValidators finds out what style of attack LightClientAttackEvidence was and then works out who
// the malicious validators were and returns them.
func getMaliciousValidators(evidence *types.LightClientAttackEvidence, commonVals *types.ValidatorSet,
	trusted *types.SignedHeader) ([]*types.Validator, lightClientAttackType) {
	var validators []*types.Validator
	// First check if the header is invalid. This means that it is a lunatic attack and therefore we take the
	// validators who are in the commonVals and voted for the lunatic header
	if isInvalidHeader(trusted.Header, evidence.ConflictingBlock.Header) {
		for _, commitSig := range evidence.ConflictingBlock.Commit.Signatures {
			if !commitSig.ForBlock() {
				continue
			}

			_, val := commonVals.GetByAddress(commitSig.ValidatorAddress)
			if val == nil {
				// validator wasn't in the common validator set
				continue
			}
			validators = append(validators, val)
		}
		return validators, lunaticType
		// Next, check to see if it is an equivocation attack and both commits are in the same round. If this is the
		// case then we take the validators from the conflicting light block validator set that voted in both headers.
	} else if trusted.Commit.Round == evidence.ConflictingBlock.Commit.Round {
		// validator hashes are the same therefore the indexing order of validators are the same and thus we
		// only need a single loop to find the validators that voted twice.
		for i := 0; i < len(evidence.ConflictingBlock.Commit.Signatures); i++ {
			sigA := evidence.ConflictingBlock.Commit.Signatures[i]
			if sigA.Absent() {
				continue
			}

			sigB := trusted.Commit.Signatures[i]
			if sigB.Absent() {
				continue
			}

			_, val := evidence.ConflictingBlock.ValidatorSet.GetByAddress(sigA.ValidatorAddress)
			validators = append(validators, val)
		}
		return validators, equivocationType

	}
	// if the rounds are different then this is an amnesia attack. Unfortunately, given the nature of the attack,
	// we aren't able yet to deduce which are malicious validators and which are not hence we return an
	// empty validator set.
	return validators, amnesiaType
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

type lightClientAttackType int

const (
	lunaticType lightClientAttackType = iota + 1
	equivocationType
	amnesiaType
)
