package lite

import (
	"errors"
	"time"

	"github.com/tendermint/tendermint/types"
)

type Client struct {
	chainID        string
	trustingPeriod time.Duration

	// trusted state
	trustedHeader *types.SignedHeader
	trustedVals   *types.ValidatorSet
}

func (c *Client) Verify(
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	if c.trustedHeaderExpired(now) {
		return erros.New("trusted header expired. reset light client subjectvely")
	}

	return c.bisection(c.trustedHeader, c.trustedVals, newHeader, newVals, now)
}

func (c *Client) trustedHeaderExpired(now time.Time) error {
	expirationTime := c.state.Header.Time.Add(c.trustingPeriod)
	return expirationTime.Before(now)
}

func (c *Client) bisection(lastHeader *types.SignedHeader,
	lastVals *types.ValidatorSet,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	err := Verify(c.chainID, lastHeader, lastVals, newHeader, newVals, now)
	if err != nil && IsErrTooMuchChange(err) {
		return err
	}

	if newHeader.Height == v.state.LastHeader.Height+1 {
		// TODO: submit evidence here
		return errors.New("adjacent headers that are not matching")
	}

	pivot := (v.state.LastHeader.Height + newHeader.Header.Height) / 2
	pivotHeader := c.signedHeader(pivot)

	c.store(pivotHeader)

	if err := c.bisection(lastHeader, lastVals, pivotHeader, pivotVals, now); err != nil {
		return c.bisection(pivotHeader, pivotVals, newHeader, newVals, now)
	}

	return errors.New("bisection failed. restart with different full-node?")
}
