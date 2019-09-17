package lite

import (
	"errors"
	"time"

	"github.com/tendermint/tendermint/types"
)

type Client struct {
	chainID        string
	trustingPeriod time.Duration
}

func (c *Client) Bisection(newHeader *types.SignedHeader, vals *types.ValidatorSet, now time.Time) error {
	if err := c.Verify(newHeader, vals); err == nil {
		return nil
	}

	if newHeader.Height == v.state.LastHeader.Height+1 {
		// we have adjacent headers that are not matching.
		// TODO: submit evidence here
		return false
	}

	pivot := (v.state.LastHeader.Height + newHeader.Header.Height) / 2
	pivotHeader := c.commit(pivot)

	// ??
	c.store(pivotHeader)

	if err := c.Bisection(pivotHeader); err != nil {
		return Bisection(pivotHeader, newHeader)
	}

	return errors.New("bisection failed. restart with different full-node?")
}

func (c *Client) Verify(
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	if c.trustedHeaderExpired(now) {
		return erros.New("trusted header expired. reset light client subjectvely")
	}

	if !c.canNewHeaderBeTrusted(now) {
		return errors.New("newHeader cannot be trusted")
	}

	if newHeader.Height == c.state.Header.Height+1 {
		return Verify(c.chainID, c.state.Header, c.state.Vals, newHeader, newVals, now)
	}

	return VerifyTrusting(c.chainID, c.state.Header, c.state.Vals, newHeader, newVals, now)
}

func (c *Client) trustedHeaderExpired(now time.Time) error {
	expirationTime := c.state.Header.Time.Add(c.trustingPeriod)
	return expirationTime.Before(now)
}

func (c *Client) canNewHeaderBeTrusted(now time.Time) error {
	trustedHeaderExpirationTime := c.trustedHeader.Time.Add(c.trustingPeriod)
	return newHeader.Time.Before(trustedHeaderExpirationTime)
}
