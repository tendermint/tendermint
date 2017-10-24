package certifiers

import (
	"bytes"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/types"

	certerr "github.com/tendermint/tendermint/certifiers/errors"
)

var _ Certifier = &Static{}

// Static assumes a static set of validators, set on
// initilization and checks against them.
// The signatures on every header is checked for > 2/3 votes
// against the known validator set upon Certify
//
// Good for testing or really simple chains.  Building block
// to support real-world functionality.
type Static struct {
	chainID string
	vSet    *types.ValidatorSet
	vhash   []byte
}

func NewStatic(chainID string, vals *types.ValidatorSet) *Static {
	return &Static{
		chainID: chainID,
		vSet:    vals,
	}
}

func (c *Static) ChainID() string {
	return c.chainID
}

func (c *Static) Validators() *types.ValidatorSet {
	return c.vSet
}

func (c *Static) Hash() []byte {
	if len(c.vhash) == 0 {
		c.vhash = c.vSet.Hash()
	}
	return c.vhash
}

func (c *Static) Certify(commit *Commit) error {
	// do basic sanity checks
	err := commit.ValidateBasic(c.chainID)
	if err != nil {
		return err
	}

	// make sure it has the same validator set we have (static means static)
	if !bytes.Equal(c.Hash(), commit.Header.ValidatorsHash) {
		return certerr.ErrValidatorsChanged()
	}

	// then make sure we have the proper signatures for this
	err = c.vSet.VerifyCommit(c.chainID, commit.Commit.BlockID,
		commit.Header.Height, commit.Commit)
	return errors.WithStack(err)
}
