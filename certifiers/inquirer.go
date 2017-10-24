package certifiers

import (
	"github.com/tendermint/tendermint/types"

	certerr "github.com/tendermint/tendermint/certifiers/errors"
)

type Inquiring struct {
	cert *Dynamic
	// These are only properly validated data, from local system
	trusted Provider
	// This is a source of new info, like a node rpc, or other import method
	Source Provider
}

func NewInquiring(chainID string, fc FullCommit, trusted Provider, source Provider) *Inquiring {
	// store the data in trusted
	trusted.StoreCommit(fc)

	return &Inquiring{
		cert:    NewDynamic(chainID, fc.Validators, fc.Height()),
		trusted: trusted,
		Source:  source,
	}
}

func (c *Inquiring) ChainID() string {
	return c.cert.ChainID()
}

func (c *Inquiring) Validators() *types.ValidatorSet {
	return c.cert.cert.vSet
}

func (c *Inquiring) LastHeight() int {
	return c.cert.lastHeight
}

// Certify makes sure this is checkpoint is valid.
//
// If the validators have changed since the last know time, it looks
// for a path to prove the new validators.
//
// On success, it will store the checkpoint in the store for later viewing
func (c *Inquiring) Certify(commit *Commit) error {
	err := c.useClosestTrust(commit.Height())
	if err != nil {
		return err
	}

	err = c.cert.Certify(commit)
	if !certerr.IsValidatorsChangedErr(err) {
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
	c.trusted.StoreCommit(
		NewFullCommit(commit, c.Validators()))
	return nil
}

func (c *Inquiring) Update(fc FullCommit) error {
	err := c.useClosestTrust(fc.Height())
	if err != nil {
		return err
	}

	err = c.cert.Update(fc)
	if err == nil {
		c.trusted.StoreCommit(fc)
	}
	return err
}

func (c *Inquiring) useClosestTrust(h int) error {
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
	if certerr.IsTooMuchChangeErr(err) {
		err = c.updateToHeight(fc.Height())
	}
	return err
}

// updateToHeight will use divide-and-conquer to find a path to h
func (c *Inquiring) updateToHeight(h int) error {
	// try to update to this height (with checks)
	fc, err := c.Source.GetByHeight(h)
	if err != nil {
		return err
	}
	start, end := c.LastHeight(), fc.Height()
	if end <= start {
		return certerr.ErrNoPathFound()
	}
	err = c.Update(fc)

	// we can handle IsTooMuchChangeErr specially
	if !certerr.IsTooMuchChangeErr(err) {
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
