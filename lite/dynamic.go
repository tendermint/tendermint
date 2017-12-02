package lite

import (
	"github.com/tendermint/tendermint/types"

	liteErr "github.com/tendermint/tendermint/lite/errors"
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
	lastHeight int64
}

// NewDynamic returns a new dynamic certifier.
func NewDynamic(chainID string, vals *types.ValidatorSet, height int64) *Dynamic {
	return &Dynamic{
		cert:       NewStatic(chainID, vals),
		lastHeight: height,
	}
}

// ChainID returns the chain id of this certifier.
func (c *Dynamic) ChainID() string {
	return c.cert.ChainID()
}

// Validators returns the validators of this certifier.
func (c *Dynamic) Validators() *types.ValidatorSet {
	return c.cert.vSet
}

// Hash returns the hash of this certifier.
func (c *Dynamic) Hash() []byte {
	return c.cert.Hash()
}

// LastHeight returns the last height of this certifier.
func (c *Dynamic) LastHeight() int64 {
	return c.lastHeight
}

// Certify will verify whether the commit is valid and will update the height if it is or return an
// error if it is not.
func (c *Dynamic) Certify(check Commit) error {
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
		return liteErr.ErrPastTime()
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
	err = c.Validators().VerifyCommitAny(fc.Validators, c.ChainID(),
		commit.BlockID, h, commit)
	if err != nil {
		return liteErr.ErrTooMuchChange()
	}

	// looks good, we can update
	c.cert = NewStatic(c.ChainID(), fc.Validators)
	c.lastHeight = h
	return nil
}
