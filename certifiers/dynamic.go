package certifiers

import (
	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/types"

	certerr "github.com/tendermint/tendermint/certifiers/errors"
)

var _ Certifier = &Dynamic{}

// Dynamic uses a Static for Certify, but adds an
// Update method to allow for a change of validators.
//
// You can pass in a FullCommit with another validator set,
// and if this is a provably secure transition (< 1/3 change,
// sufficient signatures), then it will update the
// validator set for the next Certify call.
// For security, it will only follow validator set changes
// going forward.
type Dynamic struct {
	cert       *Static
	lastHeight int
}

func NewDynamic(chainID string, vals *types.ValidatorSet, height int) *Dynamic {
	return &Dynamic{
		cert:       NewStatic(chainID, vals),
		lastHeight: height,
	}
}

func (c *Dynamic) ChainID() string {
	return c.cert.ChainID()
}

func (c *Dynamic) Validators() *types.ValidatorSet {
	return c.cert.vSet
}

func (c *Dynamic) Hash() []byte {
	return c.cert.Hash()
}

func (c *Dynamic) LastHeight() int {
	return c.lastHeight
}

// Certify handles this with
func (c *Dynamic) Certify(check *Commit) error {
	err := c.cert.Certify(check)
	if err == nil {
		// update last seen height if input is valid
		c.lastHeight = check.Height()
	}
	return err
}

// Update will verify if this is a valid change and update
// the certifying validator set if safe to do so.
//
// Returns an error if update is impossible (invalid proof or IsTooMuchChangeErr)
func (c *Dynamic) Update(fc FullCommit) error {
	// ignore all checkpoints in the past -> only to the future
	h := fc.Height()
	if h <= c.lastHeight {
		return certerr.ErrPastTime()
	}

	// first, verify if the input is self-consistent....
	err := fc.ValidateBasic(c.ChainID())
	if err != nil {
		return err
	}

	// now, make sure not too much change... meaning this commit
	// would be approved by the currently known validator set
	// as well as the new set
	commit := fc.Commit.Commit
	err = VerifyCommitAny(c.Validators(), fc.Validators, c.ChainID(),
		commit.BlockID, h, commit)
	if err != nil {
		return certerr.ErrTooMuchChange()
	}

	// looks good, we can update
	c.cert = NewStatic(c.ChainID(), fc.Validators)
	c.lastHeight = h
	return nil
}

// VerifyCommitAny will check to see if the set would
// be valid with a different validator set.
//
// old is the validator set that we know
// * over 2/3 of the power in old signed this block
//
// cur is the validator set that signed this block
// * only votes from old are sufficient for 2/3 majority
//   in the new set as well
//
// That means that:
// * 10% of the valset can't just declare themselves kings
// * If the validator set is 3x old size, we need more proof to trust
//
// *** TODO: move this.
// It belongs in tendermint/types/validator_set.go: VerifyCommitAny
func VerifyCommitAny(old, cur *types.ValidatorSet, chainID string,
	blockID types.BlockID, height int, commit *types.Commit) error {

	if cur.Size() != len(commit.Precommits) {
		return errors.Errorf("Invalid commit -- wrong set size: %v vs %v", cur.Size(), len(commit.Precommits))
	}
	if height != commit.Height() {
		return errors.Errorf("Invalid commit -- wrong height: %v vs %v", height, commit.Height())
	}

	oldVotingPower := int64(0)
	curVotingPower := int64(0)
	seen := map[int]bool{}
	round := commit.Round()

	for idx, precommit := range commit.Precommits {
		// first check as in VerifyCommit
		if precommit == nil {
			continue
		}
		if precommit.Height != height {
			return certerr.ErrHeightMismatch(height, precommit.Height)
		}
		if precommit.Round != round {
			return errors.Errorf("Invalid commit -- wrong round: %v vs %v", round, precommit.Round)
		}
		if precommit.Type != types.VoteTypePrecommit {
			return errors.Errorf("Invalid commit -- not precommit @ index %v", idx)
		}
		if !blockID.Equals(precommit.BlockID) {
			continue // Not an error, but doesn't count
		}

		// we only grab by address, ignoring unknown validators
		vi, ov := old.GetByAddress(precommit.ValidatorAddress)
		if ov == nil || seen[vi] {
			continue // missing or double vote...
		}
		seen[vi] = true

		// Validate signature old school
		precommitSignBytes := types.SignBytes(chainID, precommit)
		if !ov.PubKey.VerifyBytes(precommitSignBytes, precommit.Signature) {
			return errors.Errorf("Invalid commit -- invalid signature: %v", precommit)
		}
		// Good precommit!
		oldVotingPower += ov.VotingPower

		// check new school
		_, cv := cur.GetByIndex(idx)
		if cv.PubKey.Equals(ov.PubKey) {
			// make sure this is properly set in the current block as well
			curVotingPower += cv.VotingPower
		}
	}

	if oldVotingPower <= old.TotalVotingPower()*2/3 {
		return errors.Errorf("Invalid commit -- insufficient old voting power: got %v, needed %v",
			oldVotingPower, (old.TotalVotingPower()*2/3 + 1))
	} else if curVotingPower <= cur.TotalVotingPower()*2/3 {
		return errors.Errorf("Invalid commit -- insufficient cur voting power: got %v, needed %v",
			curVotingPower, (cur.TotalVotingPower()*2/3 + 1))
	}
	return nil
}
