package lite

import (
	"bytes"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/lite2/store"
	"github.com/tendermint/tendermint/types"
)

// TrustOptions are the trust parameters needed when a new light client
// connects to the network or when an existing light client that has been
// offline for longer than the trusting period connects to the network.
//
// The expectation is the user will get this information from a trusted source
// like a validator, a friend, or a secure website. A more user friendly
// solution with trust tradeoffs is that we establish an https based protocol
// with a default end point that populates this information. Also an on-chain
// registry of roots-of-trust (e.g. on the Cosmos Hub) seems likely in the
// future.
type TrustOptions struct {
	// tp: trusting period.
	//
	// Should be significantly less than the unbonding period (e.g. unbonding
	// period = 3 weeks, trusting period = 2 weeks).
	//
	// More specifically, trusting period + time needed to check headers + time
	// needed to report and punish misbehavior should be less than the unbonding
	// period.
	Period time.Duration

	// Header's Height and Hash must both be provided to force the trusting of a
	// particular header.
	Height int64
	Hash   []byte
}

type mode byte

const (
	sequential mode = iota + 1
	skipping
)

// Option sets a parameter for the light client.
type Option func(*Client)

// SequentialVerification option configures the light client to sequentially
// check the headers. Note this is much slower than SkippingVerification,
// albeit more secure.
func SequentialVerification() Option {
	return func(c *Client) {
		c.verificationMode = sequential
	}
}

// SkippingVerification option configures the light client to skip headers as
// long as {trustLevel} of the old validator set signed the new header. The
// bisection algorithm from the specification is used for finding the minimal
// "trust path".
//
// trustLevel - fraction of the old validator set (in terms of voting power),
// which must sign the new header in order for us to trust it. NOTE this only
// applies to non-adjusted headers. For adjusted headers, sequential
// verification is used.
func SkippingVerification(trustLevel tmmath.Fraction) Option {
	if err := ValidateTrustLevel(trustLevel); err != nil {
		panic(err)
	}
	return func(c *Client) {
		c.verificationMode = skipping
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

// Client represents a light client, connected to a single chain, which gets
// headers from a primary provider, verifies them either sequentially or by
// skipping some and stores them in a trusted store (usually, a local FS).
//
// Default verification: SkippingVerification(DefaultTrustLevel)
type Client struct {
	chainID          string
	trustingPeriod   time.Duration // see TrustOptions.Period
	verificationMode mode
	trustLevel       tmmath.Fraction

	// Primary provider of new headers.
	primary provider.Provider

	// Alternative providers for checking the primary for misbehavior by
	// comparing data.
	alternatives []provider.Provider

	// Where trusted headers are stored.
	trustedStore store.Store
	// Highest trusted header from the store (height=H).
	trustedHeader *types.SignedHeader
	// Highest next validator set from the store (height=H+1).
	trustedNextVals *types.ValidatorSet

	logger log.Logger
}

// NewClient returns a new light client. It returns an error if it fails to
// obtain the header & vals from the primary or they are invalid (e.g. trust
// hash does not match with the one from the header).
//
// See all Option(s) for the additional configuration.
func NewClient(
	chainID string,
	trustOptions TrustOptions,
	primary provider.Provider,
	trustedStore store.Store,
	options ...Option) (*Client, error) {

	c := &Client{
		chainID:          chainID,
		trustingPeriod:   trustOptions.Period,
		verificationMode: skipping,
		trustLevel:       DefaultTrustLevel,
		primary:          primary,
		trustedStore:     trustedStore,
		logger:           log.NewNopLogger(),
	}

	for _, o := range options {
		o(c)
	}

	if err := c.initializeWithTrustOptions(trustOptions); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) initializeWithTrustOptions(options TrustOptions) error {
	// 1) Fetch and verify the header.
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

	// 2) Fetch and verify the next vals.
	vals, err := c.primary.ValidatorSet(options.Height + 1)
	if err != nil {
		return err
	}

	// 3) Persist both of them and continue.
	return c.updateTrustedHeaderAndVals(h, vals)
}

// SetLogger sets a logger.
func (c *Client) SetLogger(l log.Logger) {
	c.logger = l
}

// TrustedHeader returns a trusted header at the given height (0 - the latest)
// or nil if no such header exist.
// TODO: mention how many headers will be kept by the light client.
// .
// height must be >= 0.
//
// It returns an error if:
//		- the header expired (ErrOldHeaderExpired). In that case, update your
//		client to more recent height;
//		- there are some issues with the trusted store, although that should not
//		happen normally.
func (c *Client) TrustedHeader(height int64, now time.Time) (*types.SignedHeader, error) {
	if height < 0 {
		return nil, errors.New("negative height")
	}

	if height == 0 {
		var err error
		height, err = c.LastTrustedHeight()
		if err != nil {
			return nil, err
		}
	}

	h, err := c.trustedStore.SignedHeader(height)
	if err != nil {
		return nil, err
	}

	// Ensure header can still be trusted.
	expirationTime := h.Time.Add(c.trustingPeriod)
	if !expirationTime.After(now) {
		return nil, ErrOldHeaderExpired{expirationTime, now}
	}

	return h, nil
}

// LastTrustedHeight returns a last trusted height.
func (c *Client) LastTrustedHeight() (int64, error) {
	return c.trustedStore.LastSignedHeaderHeight()
}

// ChainID returns the chain ID.
func (c *Client) ChainID() string {
	return c.chainID
}

// VerifyHeaderAtHeight fetches the header and validators at the given height
// and calls VerifyHeader.
//
// If the trusted header is more recent than one here, an error is returned.
func (c *Client) VerifyHeaderAtHeight(height int64, now time.Time) error {
	if c.trustedHeader.Height >= height {
		return errors.Errorf("height #%d is already trusted (last: #%d)", height, c.trustedHeader.Height)
	}

	// Request the header and the vals.
	newHeader, newVals, err := c.fetchHeaderAndValsAtHeight(height)
	if err != nil {
		return err
	}

	return c.VerifyHeader(newHeader, newVals, now)
}

// VerifyHeader verifies new header against the trusted state.
//
// SequentialVerification: verifies that 2/3 of the trusted validator set has
// signed the new header. If the headers are not adjacent, **all** intermediate
// headers will be requested.
//
// SkippingVerification(trustLevel): verifies that {trustLevel} of the trusted
// validator set has signed the new header. If it's not the case and the
// headers are not adjacent, bisection is performed and necessary (not all)
// intermediate headers will be requested. See the specification for the
// algorithm.
//
// If the trusted header is more recent than one here, an error is returned.
func (c *Client) VerifyHeader(newHeader *types.SignedHeader, newVals *types.ValidatorSet, now time.Time) error {
	if c.trustedHeader.Height >= newHeader.Height {
		return errors.Errorf("height #%d is already trusted (last: #%d)", newHeader.Height, c.trustedHeader.Height)
	}

	if len(c.alternatives) > 0 {
		if err := c.compareNewHeaderWithRandomAlternative(newHeader); err != nil {
			return err
		}
	}

	var err error
	switch c.verificationMode {
	case sequential:
		err = c.sequence(newHeader, newVals, now)
	case skipping:
		err = c.bisection(c.trustedHeader, c.trustedNextVals, newHeader, newVals, now)
	default:
		panic(fmt.Sprintf("Unknown verification mode: %b", c.verificationMode))
	}
	if err != nil {
		return err
	}

	// Update trusted header and vals.
	nextVals, err := c.primary.ValidatorSet(newHeader.Height + 1)
	if err != nil {
		return err
	}
	return c.updateTrustedHeaderAndVals(newHeader, nextVals)
}

func (c *Client) sequence(newHeader *types.SignedHeader, newVals *types.ValidatorSet, now time.Time) error {
	// 1) Verify any intermediate headers.
	var (
		interimHeader *types.SignedHeader
		nextVals      *types.ValidatorSet
		err           error
	)
	for height := c.trustedHeader.Height + 1; height < newHeader.Height; height++ {
		interimHeader, err = c.primary.SignedHeader(height)
		if err != nil {
			return errors.Wrapf(err, "failed to obtain the header #%d", height)
		}

		err = Verify(c.chainID, c.trustedHeader, c.trustedNextVals, interimHeader, c.trustedNextVals,
			c.trustingPeriod, now, c.trustLevel)
		if err != nil {
			return errors.Wrapf(err, "failed to verify the header #%d", height)
		}

		// Update trusted header and vals.
		if height == newHeader.Height-1 {
			nextVals = newVals
		} else {
			nextVals, err = c.primary.ValidatorSet(height + 1)
			if err != nil {
				return errors.Wrapf(err, "failed to obtain the vals #%d", height+1)
			}
		}
		err = c.updateTrustedHeaderAndVals(interimHeader, nextVals)
		if err != nil {
			return errors.Wrapf(err, "failed to update trusted state #%d", height)
		}
	}

	// 2) Verify the new header.
	return Verify(c.chainID, c.trustedHeader, c.trustedNextVals, newHeader, newVals, c.trustingPeriod, now, c.trustLevel)
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
	case types.ErrTooMuchChange:
		// continue bisection
	default:
		return errors.Wrapf(err, "failed to verify the header #%d ", newHeader.Height)
	}

	if newHeader.Height == c.trustedHeader.Height+1 {
		// TODO: submit evidence here
		return errors.Errorf("adjacent headers (#%d and #%d) that are not matching", lastHeader.Height, newHeader.Height)
	}

	pivot := (c.trustedHeader.Height + newHeader.Header.Height) / 2
	pivotHeader, pivotVals, err := c.fetchHeaderAndValsAtHeight(pivot)
	if err != nil {
		return err
	}

	// left branch
	{
		err := c.bisection(lastHeader, lastVals, pivotHeader, pivotVals, now)
		if err != nil {
			return errors.Wrapf(err, "bisection of #%d and #%d", lastHeader.Height, pivot)
		}
	}

	// right branch
	{
		nextVals, err := c.primary.ValidatorSet(pivot + 1)
		if err != nil {
			return errors.Wrapf(err, "failed to obtain the vals #%d", pivot+1)
		}
		if !bytes.Equal(pivotHeader.NextValidatorsHash, nextVals.Hash()) {
			return errors.Errorf("expected next validator's hash %X, but got %X (height #%d)",
				pivotHeader.NextValidatorsHash,
				nextVals.Hash(),
				pivot)
		}

		err = c.updateTrustedHeaderAndVals(pivotHeader, nextVals)
		if err != nil {
			return errors.Wrapf(err, "failed to update trusted state #%d", pivot)
		}

		err = c.bisection(pivotHeader, nextVals, newHeader, newVals, now)
		if err != nil {
			return errors.Wrapf(err, "bisection of #%d and #%d", pivot, newHeader.Height)
		}
	}

	return nil
}

func (c *Client) updateTrustedHeaderAndVals(h *types.SignedHeader, vals *types.ValidatorSet) error {
	if !bytes.Equal(h.NextValidatorsHash, vals.Hash()) {
		return errors.Errorf("expected next validator's hash %X, but got %X", h.NextValidatorsHash, vals.Hash())
	}

	if err := c.trustedStore.SaveSignedHeader(h); err != nil {
		return errors.Wrap(err, "failed to save trusted header")
	}
	if err := c.trustedStore.SaveValidatorSet(vals, h.Height+1); err != nil {
		return errors.Wrap(err, "failed to save trusted vals")
	}
	c.trustedHeader = h
	c.trustedNextVals = vals
	return nil
}

func (c *Client) fetchHeaderAndValsAtHeight(height int64) (*types.SignedHeader, *types.ValidatorSet, error) {
	h, err := c.primary.SignedHeader(height)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to obtain the header #%d", height)
	}
	vals, err := c.primary.ValidatorSet(height)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to obtain the vals #%d", height)
	}
	return h, vals, nil
}

func (c *Client) compareNewHeaderWithRandomAlternative(h *types.SignedHeader) error {
	// 1. Pick an alternative provider.
	p := c.alternatives[tmrand.Intn(len(c.alternatives))]

	// 2. Fetch the header.
	altHeader, err := p.SignedHeader(h.Height)
	if err != nil {
		return errors.Wrapf(err,
			"failed to obtain header #%d from alternative provider %v", h.Height, p)
	}

	// 3. Compare hashes.
	if !bytes.Equal(h.Hash(), altHeader.Hash()) {
		// TODO: One of the providers is lying. Send the evidence to fork
		// accountability server.
		return errors.Errorf(
			"new header hash %X does not match one from alternative provider %X",
			h.Hash(), altHeader.Hash())
	}

	return nil
}
