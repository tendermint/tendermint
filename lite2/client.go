package lite

import (
	"bytes"
	"fmt"
	"sync"
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

	defaultUpdatePeriod                       = 5 * time.Second
	defaultRemoveNoLongerTrustedHeadersPeriod = 24 * time.Hour
)

// Option sets a parameter for the light client.
type Option func(*Client)

// SequentialVerification option configures the light client to sequentially
// check the headers (every header, in ascending height order). Note this is
// much slower than SkippingVerification, albeit more secure.
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
// applies to non-adjacent headers. For adjacent headers, sequential
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

// UpdatePeriod option can be used to change default polling period (5s).
func UpdatePeriod(d time.Duration) Option {
	return func(c *Client) {
		c.updatePeriod = d
	}
}

// RemoveNoLongerTrustedHeadersPeriod option can be used to define how often
// the routine, which cleans up no longer trusted headers (outside of trusting
// period), is run. Default: once a day. When set to zero, the routine won't be
// started.
func RemoveNoLongerTrustedHeadersPeriod(d time.Duration) Option {
	return func(c *Client) {
		c.removeNoLongerTrustedHeadersPeriod = d
	}
}

// ConfirmationFunction option can be used to prompt to confirm an action. For
// example, remove newer headers if the light client is being reset with an
// older header. No confirmation is required by default!
func ConfirmationFunction(fn func(action string) bool) Option {
	return func(c *Client) {
		c.confirmationFn = fn
	}
}

// Logger option can be used to set a logger for the client.
func Logger(l log.Logger) Option {
	return func(c *Client) {
		c.logger = l
	}
}

// Client represents a light client, connected to a single chain, which gets
// headers from a primary provider, verifies them either sequentially or by
// skipping some and stores them in a trusted store (usually, a local FS).
//
// By default, the client will poll the primary provider for new headers every
// 5s (UpdatePeriod). If there are any, it will try to advance the state.
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

	updatePeriod                       time.Duration
	removeNoLongerTrustedHeadersPeriod time.Duration
	routinesWaitGroup                  sync.WaitGroup

	confirmationFn func(action string) bool

	quit chan struct{}

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
		chainID:                            chainID,
		trustingPeriod:                     trustOptions.Period,
		verificationMode:                   skipping,
		trustLevel:                         DefaultTrustLevel,
		primary:                            primary,
		trustedStore:                       trustedStore,
		updatePeriod:                       defaultUpdatePeriod,
		removeNoLongerTrustedHeadersPeriod: defaultRemoveNoLongerTrustedHeadersPeriod,
		confirmationFn:                     func(action string) bool { return true },
		quit:                               make(chan struct{}),
		logger:                             log.NewNopLogger(),
	}

	for _, o := range options {
		o(c)
	}

	if err := c.restoreTrustedHeaderAndNextVals(); err != nil {
		return nil, err
	}
	if c.trustedHeader != nil {
		if err := c.checkTrustedHeaderUsingOptions(trustOptions); err != nil {
			return nil, err
		}
	}

	if c.trustedHeader == nil || c.trustedHeader.Height != trustOptions.Height {
		if err := c.initializeWithTrustOptions(trustOptions); err != nil {
			return nil, err
		}
	}

	if c.removeNoLongerTrustedHeadersPeriod > 0 {
		c.routinesWaitGroup.Add(1)
		go c.removeNoLongerTrustedHeadersRoutine()
	}

	if c.updatePeriod > 0 {
		c.routinesWaitGroup.Add(1)
		go c.autoUpdateRoutine()
	}

	return c, nil
}

// Load trustedHeader and trustedNextVals from trustedStore.
func (c *Client) restoreTrustedHeaderAndNextVals() error {
	lastHeight, err := c.trustedStore.LastSignedHeaderHeight()
	if err != nil {
		return errors.Wrap(err, "can't get last trusted header height")
	}

	if lastHeight > 0 {
		trustedHeader, err := c.trustedStore.SignedHeader(lastHeight)
		if err != nil {
			return errors.Wrap(err, "can't get last trusted header")
		}

		trustedNextVals, err := c.trustedStore.ValidatorSet(lastHeight + 1)
		if err != nil {
			return errors.Wrap(err, "can't get last trusted next validators")
		}

		c.trustedHeader = trustedHeader
		c.trustedNextVals = trustedNextVals

		c.logger.Debug("Restored trusted header and next vals", lastHeight)
	}

	return nil
}

// if options.Height:
//
//     1) ahead of trustedHeader.Height => fetch header (same height as
//     trustedHeader) from primary provider and check it's hash matches the
//     trustedHeader's hash (if not, remove trustedHeader and all the headers
//     before)
//
//     2) equals trustedHeader.Height => check options.Hash matches the
//     trustedHeader's hash (if not, remove trustedHeader and all the headers
//     before)
//
//     3) behind trustedHeader.Height => remove all the headers between
//     options.Height and trustedHeader.Height, update trustedHeader, then
//     check options.Hash matches the trustedHeader's hash (if not, remove
//     trustedHeader and all the headers before)
//
// The intuition here is the user is always right. I.e. if she decides to reset
// the light client with an older header, there must be a reason for it.
func (c *Client) checkTrustedHeaderUsingOptions(options TrustOptions) error {
	var primaryHash []byte
	switch {
	case options.Height > c.trustedHeader.Height:
		h, err := c.primary.SignedHeader(c.trustedHeader.Height)
		if err != nil {
			return err
		}
		primaryHash = h.Hash()
	case options.Height == c.trustedHeader.Height:
		primaryHash = options.Hash
	case options.Height < c.trustedHeader.Height:
		c.logger.Info("Client initialized with old header (trusted is more recent)",
			"old", options.Height,
			"trusted", c.trustedHeader.Height)

		action := fmt.Sprintf(
			"Rollback to %d (%X)? Note this will remove newer headers up to %d (%X)",
			options.Height, options.Hash,
			c.trustedHeader.Height, c.trustedHeader.Hash())
		if c.confirmationFn(action) {
			// remove all the headers ( options.Height, trustedHeader.Height ]
			c.cleanup(options.Height + 1)
			// set c.trustedHeader to one at options.Height
			c.restoreTrustedHeaderAndNextVals()

			c.logger.Info("Rolled back to older header (newer headers were removed)",
				"old", options.Height)
		} else {
			return errors.New("rollback aborted")
		}

		primaryHash = options.Hash
	}

	if !bytes.Equal(primaryHash, c.trustedHeader.Hash()) {
		c.logger.Info("Prev. trusted header's hash (h1) doesn't match hash from primary provider (h2)",
			"h1", c.trustedHeader.Hash(), "h1", primaryHash)

		action := fmt.Sprintf(
			"Prev. trusted header's hash %X doesn't match hash %X from primary provider. Remove all the stored headers?",
			c.trustedHeader.Hash(), primaryHash)
		if c.confirmationFn(action) {
			err := c.Cleanup()
			if err != nil {
				return errors.Wrap(err, "failed to cleanup")
			}
		} else {
			return errors.New("refused to remove the stored headers despite hashes mismatch")
		}
	}

	return nil
}

// Fetch trustedHeader and trustedNextVals from primary provider.
func (c *Client) initializeWithTrustOptions(options TrustOptions) error {
	// 1) Fetch and verify the header.
	h, err := c.primary.SignedHeader(options.Height)
	if err != nil {
		return err
	}

	// NOTE: Verify func will check if it's expired or not.
	if err := h.ValidateBasic(c.chainID); err != nil {
		return err
	}

	if !bytes.Equal(h.Hash(), options.Hash) {
		return errors.Errorf("expected header's hash %X, but got %X", options.Hash, h.Hash())
	}

	// 2) Fetch and verify the vals.
	vals, err := c.primary.ValidatorSet(options.Height)
	if err != nil {
		return err
	}
	if !bytes.Equal(h.ValidatorsHash, vals.Hash()) {
		return errors.Errorf("expected header's validators (%X) to match those that were supplied (%X)",
			h.ValidatorsHash,
			vals.Hash(),
		)
	}
	// Ensure that +2/3 of validators signed correctly.
	err = vals.VerifyCommit(c.chainID, h.Commit.BlockID, h.Height, h.Commit)
	if err != nil {
		return errors.Wrap(err, "invalid commit")
	}

	// 3) Fetch and verify the next vals (verification happens in
	// updateTrustedHeaderAndVals).
	nextVals, err := c.primary.ValidatorSet(options.Height + 1)
	if err != nil {
		return err
	}

	// 4) Persist both of them and continue.
	return c.updateTrustedHeaderAndVals(h, nextVals)
}

// Stop stops all the goroutines. If you wish to remove all the data, call
// Cleanup.
func (c *Client) Stop() {
	close(c.quit)
}

// TrustedHeader returns a trusted header at the given height (0 - the latest)
// or nil if no such header exist.
//
// Headers, which can't be trusted anymore, are removed once a day (can be
// changed with RemoveNoLongerTrustedHeadersPeriod option).
// .
// height must be >= 0.
//
// It returns an error if:
//  - header expired, therefore can't be trusted (ErrOldHeaderExpired);
//  - there are some issues with the trusted store, although that should not
//  happen normally;
//  - negative height is passed;
//  - header is not found.
//
// Safe for concurrent use by multiple goroutines.
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
	if HeaderExpired(h, c.trustingPeriod, now) {
		return nil, ErrOldHeaderExpired{h.Time.Add(c.trustingPeriod), now}
	}

	return h, nil
}

// LastTrustedHeight returns a last trusted height. -1 and nil are returned if
// there are no trusted headers.
//
// Safe for concurrent use by multiple goroutines.
func (c *Client) LastTrustedHeight() (int64, error) {
	return c.trustedStore.LastSignedHeaderHeight()
}

// FirstTrustedHeight returns a first trusted height. -1 and nil are returned if
// there are no trusted headers.
//
// Safe for concurrent use by multiple goroutines.
func (c *Client) FirstTrustedHeight() (int64, error) {
	return c.trustedStore.FirstSignedHeaderHeight()
}

// ChainID returns the chain ID the light client was configured with.
//
// Safe for concurrent use by multiple goroutines.
func (c *Client) ChainID() string {
	return c.chainID
}

// VerifyHeaderAtHeight fetches the header and validators at the given height
// and calls VerifyHeader.
//
// If the trusted header is more recent than one here, an error is returned.
// If the header is not found by the primary provider,
// provider.ErrSignedHeaderNotFound error is returned.
func (c *Client) VerifyHeaderAtHeight(height int64, now time.Time) (*types.SignedHeader, error) {
	c.logger.Info("VerifyHeaderAtHeight", "height", height)

	if c.trustedHeader.Height >= height {
		return nil, errors.Errorf("header at more recent height #%d exists", c.trustedHeader.Height)
	}

	// Request the header and the vals.
	newHeader, newVals, err := c.fetchHeaderAndValsAtHeight(height)
	if err != nil {
		return nil, err
	}

	return newHeader, c.VerifyHeader(newHeader, newVals, now)
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
// intermediate headers will be requested. See the specification for details.
// https://github.com/tendermint/spec/blob/master/spec/consensus/light-client.md
//
// If the trusted header is more recent than one here, an error is returned.
//
// If, at any moment, SignedHeader or ValidatorSet are not found by the primary
// provider, provider.ErrSignedHeaderNotFound /
// provider.ErrValidatorSetNotFound error is returned.
func (c *Client) VerifyHeader(newHeader *types.SignedHeader, newVals *types.ValidatorSet, now time.Time) error {
	c.logger.Info("VerifyHeader", "height", newHeader.Hash(), "newVals", fmt.Sprintf("%X", newVals.Hash()))

	if c.trustedHeader.Height >= newHeader.Height {
		return errors.Errorf("header at more recent height #%d exists", c.trustedHeader.Height)
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

// Cleanup removes all the data (headers and validator sets) stored. It blocks
// until internal routines are finished. Note: the client must be stopped at
// this point.
func (c *Client) Cleanup() error {
	c.routinesWaitGroup.Wait()
	c.logger.Info("Cleanup everything")
	return c.cleanup(0)
}

// stopHeight=0 -> remove all data
func (c *Client) cleanup(stopHeight int64) error {
	// 1) Get the oldest height.
	oldestHeight, err := c.trustedStore.FirstSignedHeaderHeight()
	if err != nil {
		return errors.Wrap(err, "can't get first trusted height")
	}

	// 2) Get the latest height.
	latestHeight, err := c.trustedStore.LastSignedHeaderHeight()
	if err != nil {
		return errors.Wrap(err, "can't get last trusted height")
	}

	// 3) Remove all headers and validator sets.
	if stopHeight == 0 {
		stopHeight = oldestHeight
	}
	for height := stopHeight; height <= latestHeight; height++ {
		err = c.trustedStore.DeleteSignedHeaderAndNextValidatorSet(height)
		if err != nil {
			c.logger.Error("can't remove a trusted header & validator set", "err", err, "height", height)
			continue
		}
	}

	c.trustedHeader = nil
	c.trustedNextVals = nil

	return nil
}

// see VerifyHeader
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

		c.logger.Debug("Verify newHeader against lastHeader",
			"lastHeight", c.trustedHeader.Height,
			"lastHash", c.trustedHeader.Hash(),
			"newHeight", interimHeader.Height,
			"newHash", interimHeader.Hash())
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

// see VerifyHeader
func (c *Client) bisection(
	lastHeader *types.SignedHeader,
	lastVals *types.ValidatorSet,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	c.logger.Debug("Verify newHeader against lastHeader",
		"lastHeight", lastHeader.Height,
		"lastHash", lastHeader.Hash(),
		"newHeight", newHeader.Height,
		"newHash", newHeader.Hash())
	err := Verify(c.chainID, lastHeader, lastVals, newHeader, newVals, c.trustingPeriod, now, c.trustLevel)
	switch err.(type) {
	case nil:
		return nil
	case ErrNewValSetCantBeTrusted:
		// continue bisection
	default:
		return errors.Wrapf(err, "failed to verify the header #%d", newHeader.Height)
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

// persist header and next validators to trustedStore.
func (c *Client) updateTrustedHeaderAndVals(h *types.SignedHeader, nextVals *types.ValidatorSet) error {
	if !bytes.Equal(h.NextValidatorsHash, nextVals.Hash()) {
		return errors.Errorf("expected next validator's hash %X, but got %X", h.NextValidatorsHash, nextVals.Hash())
	}

	if err := c.trustedStore.SaveSignedHeaderAndNextValidatorSet(h, nextVals); err != nil {
		return errors.Wrap(err, "failed to save trusted header")
	}

	c.trustedHeader = h
	c.trustedNextVals = nextVals

	return nil
}

// fetch header and validators for the given height from primary provider.
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

// compare header with one from a random alternative provider.
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

func (c *Client) removeNoLongerTrustedHeadersRoutine() {
	defer c.routinesWaitGroup.Done()

	ticker := time.NewTicker(c.removeNoLongerTrustedHeadersPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.RemoveNoLongerTrustedHeaders(time.Now())
		case <-c.quit:
			return
		}
	}
}

// RemoveNoLongerTrustedHeaders removes no longer trusted headers (due to
// expiration).
//
// Exposed for testing.
func (c *Client) RemoveNoLongerTrustedHeaders(now time.Time) {
	// 1) Get the oldest height.
	oldestHeight, err := c.trustedStore.FirstSignedHeaderHeight()
	if err != nil {
		c.logger.Error("can't get first trusted height", "err", err)
		return
	}

	// 2) Get the latest height.
	latestHeight, err := c.LastTrustedHeight()
	if err != nil {
		c.logger.Error("can't get last trusted height", "err", err)
		return
	}

	// 3) Remove all headers that are outside of the trusting period.
	for height := oldestHeight; height <= latestHeight; height++ {
		h, err := c.trustedStore.SignedHeader(height)
		if err != nil {
			c.logger.Error("can't get a trusted header", "err", err, "height", height)
			continue
		}

		// Stop if the header is within the trusting period.
		if !HeaderExpired(h, c.trustingPeriod, now) {
			break
		}

		err = c.trustedStore.DeleteSignedHeaderAndNextValidatorSet(height)
		if err != nil {
			c.logger.Error("can't remove a trusted header & validator set", "err", err, "height", height)
			continue
		}
	}
}

func (c *Client) autoUpdateRoutine() {
	defer c.routinesWaitGroup.Done()

	ticker := time.NewTicker(c.updatePeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := c.Update(time.Now())
			if err != nil {
				c.logger.Error("Error during auto update", "err", err)
			}
		case <-c.quit:
			return
		}
	}
}

// Update attempts to advance the state making exponential steps (note:
// when SequentialVerification is being used, the client will still be
// downloading all intermediate headers).
//
// Exposed for testing.
func (c *Client) Update(now time.Time) error {
	lastTrustedHeight, err := c.LastTrustedHeight()
	if err != nil {
		return errors.Wrap(err, "can't get last trusted height")
	}

	if lastTrustedHeight == -1 {
		// no headers yet => wait
		return nil
	}

	latestHeader, latestVals, err := c.fetchHeaderAndValsAtHeight(0)
	if err != nil {
		return errors.Wrapf(err, "can't get latest header and vals")
	}

	if latestHeader.Height > lastTrustedHeight {
		err = c.VerifyHeader(latestHeader, latestVals, now)
		if err != nil {
			return err
		}

		c.logger.Info("Advanced to new state", "height", latestHeader.Height, "hash", latestHeader.Hash())
	}

	return nil
}
