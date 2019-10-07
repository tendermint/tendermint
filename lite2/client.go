package lite

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"

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

	// Height and Hash can both be provided to force the trusting of a
	// particular height and hash.
	Height int64
	Hash   []byte
}

type mode int

const (
	sequential mode = iota
	bisecting
)

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

// TrustedStore option can be used to change default store for trusted headers.
func TrustedStore(s store.Store) Option {
	return func(c *Client) {
		c.trustedStore = s
	}
}

// AlternativeSources option can be used to supply alternative providers, which
// will be used for cross-checking the primary provider of new headers.
func AlternativeSources(providers []provider.Provider) Option {
	return func(c *Client) {
		c.alternatives = providers
	}
}

type Client struct {
	chainID            string
	trustingPeriod     time.Duration
	mode               mode
	trustLevel         float32
	lastVerifiedHeight int64

	// Primary provider of new headers.
	primary provider.Provider

	// Alternative providers for checking the primary for misbehavior by
	// comparing data.
	alternatives []provider.Provider

	// Where trusted headers are stored.
	trustedStore store.Store

	trustedHeader *types.SignedHeader
	trustedVals   *types.ValidatorSet

	logger log.Logger
}

// NewClient returns a new Client.
//
// If no trusted store is configured using TrustedStore option, goleveldb
// database will be used (./trusted.lvl).
func NewClient(
	chainID string,
	trustOptions TrustOptions,
	primary provider.Provider,
	trustedStore store.Store,
	options ...Option) (*Client, error) {

	c := &Client{
		chainID:      chainID,
		primary:      primary,
		trustedStore: trustedStore,
		mode:         bisecting,
		trustLevel:   DefaultTrustLevel,
		logger:       log.NopLogger(),
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

func (c *Client) initializeWithTrustOptions(options TrustOptions) error {
	h, err := c.primary.SignedHeader(trustOptions.Height)
	if err != nil {
		return err
	}

	// NOTE: Verify func will check if it's expired or not.
	if err := h.ValidateBasic(c.chainID); err != nil {
		return errors.Wrap(err, "ValidateBasic failed")
	}

	if !bytes.Equal(h.Hash(), trustOptions.Hash) {
		return fmt.Errorf("expected header's hash %X, but got %X", options.Hash, signedHeader.Hash())
	}

	vals, err := c.primary.ValidatorSet(trustOptions.Height + 1)
	if err != nil {
		return err
	}

	if !bytes.Equal(h.NextValidatorsHash, vals.Hash()) {
		return fmt.Errorf("expected next validator's hash %X, but got %X", h.NextValidatorsHash, vals.Hash())
	}

	// Persist header and vals.
	err = c.trustedStore.SaveSignedHeader(h)
	if err != nil {
		return errors.Wrap(err, "failed to save trusted header")
	}
	err = c.trustedStore.SaveValidatorSet(vals)
	if err != nil {
		return errors.Wrap(err, "failed to save trusted vals")
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

	c.VerifyHeader(c.trustedHeader, c.trustedVals, newHeader, newVals, now)
}

func (c *Client) VerifyHeader(newHeader *types.SignedHeader, newVals *types.ValidatorSet, now time.Time) error {
	return c.bisection(c.trustedHeader, c.trustedVals, newHeader, newVals, now)
}

func (c *Client) bisection(lastHeader *types.SignedHeader,
	lastVals *types.ValidatorSet,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	err := Verify(c.chainID, lastHeader, lastVals, newHeader, newVals, c.trustOptions.Period, now, c.trustLevel)
	switch err.(type) {
	case nil:
		return nil
	case ErrNewHeaderTooFarIntoFuture:
		// continue bisection
		// if adjused headers, fail?
	case types.ErrTooMuchChange:
		// continue bisection
	default:
		return err
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

	if err := c.bisection(lastHeader, lastVals, pivotHeader, pivotVals, now); err != nil {
		return c.bisection(pivotHeader, pivotVals, newHeader, newVals, now)
	}

	return errors.New("bisection failed. restart with different full-node?")
}
