package lite

import (
	"errors"
	"time"

	"github.com/tendermint/tendermint/types"
)

type Client struct {
	chainID        string
	trustingPeriod time.Duration
	trustLevel     float32 // trustLevel used in bisection for non-adjacent headers

	// trusted state
	trustedHeader *types.SignedHeader
	trustedVals   *types.ValidatorSet
}

func (c *Client) Verify(
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	return c.bisection(c.trustedHeader, c.trustedVals, newHeader, newVals, now)
}

func (c *Client) bisection(lastHeader *types.SignedHeader,
	lastVals *types.ValidatorSet,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	err := Verify(c.chainID, lastHeader, lastVals, newHeader, newVals, c.trustingPeriod, now, c.trustLevel)
	switch {
	case err == nil:
		return nil
	case IsErrNewHeaderTooFarIntoFuture(err):
		// continue bisection
		// if adjused headers, fail?
	case types.IsErrTooMuchChange(err):
		// continue bisection
	case err != nil:
		return err
	}

	if newHeader.Height == c.trustedHeader.Height+1 {
		// TODO: submit evidence here
		return errors.New("adjacent headers that are not matching")
	}

	pivot := (c.trustedHeader.Height + newHeader.Header.Height) / 2
	pivotHeader := c.signedHeader(pivot)
	c.storeSignedHeader(pivotHeader)

	pivotVals := c.validators(pivot)
	c.storeValidators(pivotVals)

	if err := c.bisection(lastHeader, lastVals, pivotHeader, pivotVals, now); err != nil {
		return c.bisection(pivotHeader, pivotVals, newHeader, newVals, now)
	}

	return errors.New("bisection failed. restart with different full-node?")
}

func (c *Client) storeSignedHeader(h *types.SignedHeader) {
	// TODO: save to DB
}

func (c *Client) storeValidators(vals *types.ValidatorSet) {
	// TODO: save to DB
}

func (c *Client) signedHeader(height int64) *types.SignedHeader {
	// TODO: use provider
	return nil
}

func (c *Client) validators(height int64) *types.ValidatorSet {
	// TODO: use provider
	return nil
}
