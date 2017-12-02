package lite

import (
	"github.com/tendermint/tendermint/types"

	liteErr "github.com/tendermint/tendermint/lite/errors"
)

// Inquiring wraps a dynamic certifier and implements an auto-update strategy. If a call to Certify
// fails due to a change it validator set, Inquiring will try and find a previous FullCommit which
// it can use to safely update the validator set. It uses a source provider to obtain the needed
// FullCommits. It stores properly validated data on the local system.
type Inquiring struct {
	cert *Dynamic
	// These are only properly validated data, from local system
	trusted Provider
	// This is a source of new info, like a node rpc, or other import method
	Source Provider
}

// NewInquiring returns a new Inquiring object. It uses the trusted provider to store validated
// data and the source provider to obtain missing FullCommits.
//
// Example: The trusted provider should a CacheProvider, MemProvider or files.Provider. The source
// provider should be a client.HTTPProvider.
func NewInquiring(chainID string, fc FullCommit, trusted Provider, source Provider) *Inquiring {
	// store the data in trusted
	// TODO: StoredCommit() can return an error and we need to handle this.
	trusted.StoreCommit(fc)

	return &Inquiring{
		cert:    NewDynamic(chainID, fc.Validators, fc.Height()),
		trusted: trusted,
		Source:  source,
	}
}

// ChainID returns the chain id.
func (c *Inquiring) ChainID() string {
	return c.cert.ChainID()
}

// Validators returns the validator set.
func (c *Inquiring) Validators() *types.ValidatorSet {
	return c.cert.cert.vSet
}

// LastHeight returns the last height.
func (c *Inquiring) LastHeight() int64 {
	return c.cert.lastHeight
}

// Certify makes sure this is checkpoint is valid.
//
// If the validators have changed since the last know time, it looks
// for a path to prove the new validators.
//
// On success, it will store the checkpoint in the store for later viewing
func (c *Inquiring) Certify(commit Commit) error {
	err := c.useClosestTrust(commit.Height())
	if err != nil {
		return err
	}

	err = c.cert.Certify(commit)
	if !liteErr.IsValidatorsChangedErr(err) {
		return err
	}
	err = c.updateToHash(commit.Header.ValidatorsHash)
	if err != nil {
		return err
	}

	err = c.cert.Certify(commit)
	if err != nil {
		return err
	}

	// store the new checkpoint
	return c.trusted.StoreCommit(NewFullCommit(commit, c.Validators()))
}

// Update will verify if this is a valid change and update
// the certifying validator set if safe to do so.
func (c *Inquiring) Update(fc FullCommit) error {
	err := c.useClosestTrust(fc.Height())
	if err != nil {
		return err
	}

	err = c.cert.Update(fc)
	if err == nil {
		err = c.trusted.StoreCommit(fc)
	}
	return err
}

func (c *Inquiring) useClosestTrust(h int64) error {
	closest, err := c.trusted.GetByHeight(h)
	if err != nil {
		return err
	}

	// if the best seed is not the one we currently use,
	// let's just reset the dynamic validator
	if closest.Height() != c.LastHeight() {
		c.cert = NewDynamic(c.ChainID(), closest.Validators, closest.Height())
	}
	return nil
}

// updateToHash gets the validator hash we want to update to
// if IsTooMuchChangeErr, we try to find a path by binary search over height
func (c *Inquiring) updateToHash(vhash []byte) error {
	// try to get the match, and update
	fc, err := c.Source.GetByHash(vhash)
	if err != nil {
		return err
	}
	err = c.cert.Update(fc)
	// handle IsTooMuchChangeErr by using divide and conquer
	if liteErr.IsTooMuchChangeErr(err) {
		err = c.updateToHeight(fc.Height())
	}
	return err
}

// updateToHeight will use divide-and-conquer to find a path to h
func (c *Inquiring) updateToHeight(h int64) error {
	// try to update to this height (with checks)
	fc, err := c.Source.GetByHeight(h)
	if err != nil {
		return err
	}
	start, end := c.LastHeight(), fc.Height()
	if end <= start {
		return liteErr.ErrNoPathFound()
	}
	err = c.Update(fc)

	// we can handle IsTooMuchChangeErr specially
	if !liteErr.IsTooMuchChangeErr(err) {
		return err
	}

	// try to update to mid
	mid := (start + end) / 2
	err = c.updateToHeight(mid)
	if err != nil {
		return err
	}

	// if we made it to mid, we recurse
	return c.updateToHeight(h)
}
