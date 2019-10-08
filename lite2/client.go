package lite

import (
	"bytes"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/lite2/store"
	"github.com/tendermint/tendermint/types"
)

// TrustOptions are the trust parameters needed for when a new light client
// connects to the network or when a light client that has been offline for
// longer than the unbonding period connects to the network.
//
// The expectation is the user will get this information from a trusted source
// like a validator, a friend, or a secure website. A more user friendly
// solution with trust tradeoffs is that we establish an https based protocol
// with a default end point that populates this information. Also an on-chain
// registry of roots-of-trust (e.g. on the Cosmos Hub) seems likely in the
// future.
type TrustOptions struct {
	// Only trust commits up to this old.
	// Should be equal to the unbonding period minus a configurable evidence
	// submission synchrony bound.
	Period time.Duration

	// Height and Hash must both be provided to force the trusting of a
	// particular height and hash.
	Height int64
	Hash   []byte
}

type mode int

const (
	sequential mode = iota
	bisecting
)

// Option sets a parameter for the light client.
type Option func(*Client)

// SequentialVerification option can be used to instruct Verifier to
// sequentially check the headers. Note this is much slower than
// BisectingVerification, albeit more secure.
func SequentialVerification() Option {
	return func(c *Client) {
		c.mode = sequential
	}
}

// BisectingVerification option can be used to instruct Verifier to check the
// headers using bisection algorithm described in XXX.
//
// trustLevel - maximum change between two not consequitive headers in terms of
// validators & their respective voting power, required to trust a new header
// (default: 1/3).
func BisectingVerification(trustLevel float32) Option {
	if trustLevel > 1 || trustLevel < 1/3 {
		panic(fmt.Sprintf("trustLevel must be within [1/3, 1], given %v", trustLevel))
	}
	return func(c *Client) {
		c.mode = bisecting
		c.trustLevel = trustLevel
	}
}

// AlternativeSources option can be used to supply alternative providers, which
// will be used for cross-checking the primary provider.
func AlternativeSources(providers []provider.Provider) Option {
	return func(c *Client) {
		c.alternatives = providers
	}
}

type Client struct {
	chainID        string
	trustingPeriod time.Duration
	mode           mode
	trustLevel     float32

	// Primary provider of new headers.
	primary provider.Provider

	// Alternative providers for checking the primary for misbehavior by
	// comparing data.
	alternatives []provider.Provider

	// Where trusted headers are stored.
	trustedStore  store.Store
	trustedHeader *types.SignedHeader
	trustedVals   *types.ValidatorSet

	logger log.Logger
}

// NewClient returns a new light client.
func NewClient(
	chainID string,
	trustOptions TrustOptions,
	primary provider.Provider,
	trustedStore store.Store,
	options ...Option) (*Client, error) {

	c := &Client{
		chainID:        chainID,
		trustingPeriod: trustOptions.Period,
		mode:           bisecting,
		trustLevel:     DefaultTrustLevel,
		primary:        primary,
		trustedStore:   trustedStore,
		logger:         log.NewNopLogger(),
	}

	for _, o := range options {
		o(c)
	}

	err := c.initializeWithTrustOptions(trustOptions)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// TODO: cross check primary with alternatives providers
// func (c *Client) crossCheckPrimary() {
// }

func (c *Client) initializeWithTrustOptions(options TrustOptions) error {
	h, err := c.primary.SignedHeader(options.Height)
	if err != nil {
		return err
	}

	// NOTE: Verify func will check if it's expired or not.
	if err := h.ValidateBasic(c.chainID); err != nil {
		return errors.Wrap(err, "ValidateBasic failed")
	}

	if !bytes.Equal(h.Hash(), options.Hash) {
		return errors.Errorf("expected header's hash %X, but got %X", options.Hash, h.Hash())
	}

	vals, err := c.primary.ValidatorSet(options.Height + 1)
	if err != nil {
		return err
	}

	if !bytes.Equal(h.NextValidatorsHash, vals.Hash()) {
		return errors.Errorf("expected next validator's hash %X, but got %X", h.NextValidatorsHash, vals.Hash())
	}

	// Persist header and vals.
	err = c.saveTrustedHeaderAndVals(h, vals)
	if err != nil {
		return err
	}

	c.trustedHeader = h
	c.trustedVals = vals

	return nil
}

// SetLogger sets a logger.
func (c *Client) SetLogger(l log.Logger) {
	c.logger = l
}

func (c *Client) VerifyHeaderAtHeight(height int64, now time.Time) error {
	// Request the header and the vals.
	newHeader, err := c.primary.SignedHeader(height)
	if err != nil {
		return err
	}
	newVals, err := c.primary.ValidatorSet(height)
	if err != nil {
		return err
	}

	return c.VerifyHeader(newHeader, newVals, now)
}

func (c *Client) VerifyHeader(newHeader *types.SignedHeader, newVals *types.ValidatorSet, now time.Time) error {
	switch c.mode {
	case sequential:
		err := c.sequence(newHeader, newVals, now)
		if err != nil {
			return err
		}
	case bisecting:
		err := c.bisection(c.trustedHeader, c.trustedVals, newHeader, newVals, now)
		if err != nil {
			return err
		}
	}

	nextVals, err := c.primary.ValidatorSet(newHeader.Height + 1)
	if err != nil {
		return err
	}
	err = c.saveTrustedHeaderAndVals(newHeader, nextVals)
	if err != nil {
		return err
	}

	c.trustedHeader = newHeader
	c.trustedVals = nextVals

	return nil
}

func (c *Client) bisection(
	lastHeader *types.SignedHeader,
	lastVals *types.ValidatorSet,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	err := Verify(c.chainID, lastHeader, lastVals, newHeader, newVals, c.trustingPeriod, now, c.trustLevel)
	switch err.(type) {
	case nil:
		return nil
	case ErrNewHeaderTooFarIntoFuture:
		// continue bisection
		// if adjused headers, fail?
	case types.ErrTooMuchChange:
		// continue bisection
	default:
		return errors.Wrapf(err, "failed to bisect #%d and #%d headers", lastHeader.Height, newHeader.Height)
	}

	if newHeader.Height == c.trustedHeader.Height+1 {
		// TODO: submit evidence here
		return errors.New("adjacent headers that are not matching")
	}

	pivot := (c.trustedHeader.Height + newHeader.Header.Height) / 2

	pivotHeader, err := c.primary.SignedHeader(pivot)
	if err != nil {
		return err
	}

	pivotVals, err := c.primary.ValidatorSet(pivot)
	if err != nil {
		return err
	}

	if err := c.bisection(lastHeader, lastVals, pivotHeader, pivotVals, now); err == nil {
		pivotVals2, err := c.primary.ValidatorSet(pivot + 1)
		if err != nil {
			return err
		}
		return c.bisection(pivotHeader, pivotVals2, newHeader, newVals, now)
	}

	return errors.New("bisection failed. restart with different full-node?")
}

func (c *Client) sequence(
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	// Verify intermediate headers (if any)
	lastHeader := c.trustedHeader
	lastVals := c.trustedVals
	for height := lastHeader.Height + 1; height < newHeader.Height; height++ {
		interimHeader, err := c.primary.SignedHeader(height)
		if err != nil {
			return err
		}
		interimVals, err := c.primary.ValidatorSet(height)
		if err != nil {
			return err
		}
		err = Verify(c.chainID, lastHeader, lastVals, interimHeader, interimVals, c.trustingPeriod, now, c.trustLevel)
		if err != nil {
			return err
		}
		lastHeader = interimHeader
		lastVals = interimVals
	}

	return Verify(c.chainID, lastHeader, lastVals, newHeader, newVals, c.trustingPeriod, now, c.trustLevel)
}

func (c *Client) saveTrustedHeaderAndVals(h *types.SignedHeader, vals *types.ValidatorSet) error {
	if err := c.trustedStore.SaveSignedHeader(h); err != nil {
		return errors.Wrap(err, "failed to save trusted header")
	}
	if err := c.trustedStore.SaveValidatorSet(vals, h.Height+1); err != nil {
		return errors.Wrap(err, "failed to save trusted vals")
	}
	return nil
}
