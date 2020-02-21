package lite

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/lite2/store"
	"github.com/tendermint/tendermint/types"
)

type mode byte

const (
	sequential mode = iota + 1
	skipping

	defaultUpdatePeriod                       = 5 * time.Second
	defaultRemoveNoLongerTrustedHeadersPeriod = 24 * time.Hour
	defaultMaxRetryAttempts                   = 10
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
	return func(c *Client) {
		c.verificationMode = skipping
		c.trustLevel = trustLevel
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

// MaxRetryAttempts option can be used to set max attempts before replacing
// primary with a witness.
func MaxRetryAttempts(max uint16) Option {
	return func(c *Client) {
		c.maxRetryAttempts = max
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
	maxRetryAttempts uint16 // see MaxRetryAttempts option

	// Mutex for locking during changes of the lite clients providers
	providerMutex sync.Mutex
	// Primary provider of new headers.
	primary provider.Provider
	// See Witnesses option
	witnesses []provider.Provider

	// Where trusted headers are stored.
	trustedStore store.Store
	// Highest trusted header from the store (height=H).
	latestTrustedHeader *types.SignedHeader
	// Highest next validator set from the store (height=H+1).
	latestTrustedNextVals *types.ValidatorSet

	// See UpdatePeriod option
	updatePeriod time.Duration
	// See RemoveNoLongerTrustedHeadersPeriod option
	removeNoLongerTrustedHeadersPeriod time.Duration
	// See ConfirmationFunction option
	confirmationFn func(action string) bool

	routinesWaitGroup sync.WaitGroup
	quit              chan struct{}

	logger log.Logger
}

// NewClient returns a new light client. It returns an error if it fails to
// obtain the header & vals from the primary or they are invalid (e.g. trust
// hash does not match with the one from the header).
//
// Witnesses are providers, which will be used for cross-checking the primary
// provider. At least one witness must be given. A witness can become a primary
// iff the current primary is unavailable.
//
// See all Option(s) for the additional configuration.
func NewClient(
	chainID string,
	trustOptions TrustOptions,
	primary provider.Provider,
	witnesses []provider.Provider,
	trustedStore store.Store,
	options ...Option) (*Client, error) {

	if err := trustOptions.ValidateBasic(); err != nil {
		return nil, errors.Wrap(err, "invalid TrustOptions")
	}

	c, err := NewClientFromTrustedStore(chainID, trustOptions.Period, primary, witnesses, trustedStore, options...)
	if err != nil {
		return nil, err
	}

	if c.latestTrustedHeader != nil {
		if err := c.checkTrustedHeaderUsingOptions(trustOptions); err != nil {
			return nil, err
		}
	}

	if c.latestTrustedHeader == nil || c.latestTrustedHeader.Height < trustOptions.Height {
		if err := c.initializeWithTrustOptions(trustOptions); err != nil {
			return nil, err
		}
	}

	return c, err
}

// NewClientFromTrustedStore initializes existing client from the trusted store.
//
// See NewClient
func NewClientFromTrustedStore(
	chainID string,
	trustingPeriod time.Duration,
	primary provider.Provider,
	witnesses []provider.Provider,
	trustedStore store.Store,
	options ...Option) (*Client, error) {

	c := &Client{
		chainID:                            chainID,
		trustingPeriod:                     trustingPeriod,
		verificationMode:                   skipping,
		trustLevel:                         DefaultTrustLevel,
		maxRetryAttempts:                   defaultMaxRetryAttempts,
		primary:                            primary,
		witnesses:                          witnesses,
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

	// Validate the number of witnesses.
	if len(c.witnesses) < 1 {
		return nil, errors.New("expected at least one witness")
	}

	// Verify witnesses are all on the same chain.
	for i, w := range witnesses {
		if w.ChainID() != chainID {
			return nil, errors.Errorf("witness #%d: %v is on another chain %s, expected %s",
				i, w, w.ChainID(), chainID)
		}
	}

	// Validate trust level.
	if err := ValidateTrustLevel(c.trustLevel); err != nil {
		return nil, err
	}

	if err := c.restoreTrustedHeaderAndNextVals(); err != nil {
		return nil, err
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

		c.latestTrustedHeader = trustedHeader
		c.latestTrustedNextVals = trustedNextVals

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
	case options.Height > c.latestTrustedHeader.Height:
		h, err := c.signedHeaderFromPrimary(c.latestTrustedHeader.Height)
		if err != nil {
			return err
		}
		primaryHash = h.Hash()
	case options.Height == c.latestTrustedHeader.Height:
		primaryHash = options.Hash
	case options.Height < c.latestTrustedHeader.Height:
		c.logger.Info("Client initialized with old header (trusted is more recent)",
			"old", options.Height,
			"trustedHeight", c.latestTrustedHeader.Height,
			"trustedHash", hash2str(c.latestTrustedHeader.Hash()))

		action := fmt.Sprintf(
			"Rollback to %d (%X)? Note this will remove newer headers up to %d (%X)",
			options.Height, options.Hash,
			c.latestTrustedHeader.Height, c.latestTrustedHeader.Hash())
		if c.confirmationFn(action) {
			// remove all the headers (options.Height, trustedHeader.Height]
			c.cleanup(options.Height + 1)

			c.logger.Info("Rolled back to older header (newer headers were removed)",
				"old", options.Height)
		} else {
			return nil
		}

		primaryHash = options.Hash
	}

	if !bytes.Equal(primaryHash, c.latestTrustedHeader.Hash()) {
		c.logger.Info("Prev. trusted header's hash (h1) doesn't match hash from primary provider (h2)",
			"h1", hash2str(c.latestTrustedHeader.Hash()), "h2", hash2str(primaryHash))

		action := fmt.Sprintf(
			"Prev. trusted header's hash %X doesn't match hash %X from primary provider. Remove all the stored headers?",
			c.latestTrustedHeader.Hash(), primaryHash)
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
	h, err := c.signedHeaderFromPrimary(options.Height)
	if err != nil {
		return err
	}

	// NOTE: - Verify func will check if it's expired or not.
	//       - h.Time is not being checked against time.Now() because we don't
	//         want to add yet another argument to NewClient* functions.
	if err := h.ValidateBasic(c.chainID); err != nil {
		return err
	}

	if !bytes.Equal(h.Hash(), options.Hash) {
		return errors.Errorf("expected header's hash %X, but got %X", options.Hash, h.Hash())
	}

	// 2) Fetch and verify the vals.
	vals, err := c.validatorSetFromPrimary(options.Height)
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
	// updateTrustedHeaderAndNextVals).
	nextVals, err := c.validatorSetFromPrimary(options.Height + 1)
	if err != nil {
		return err
	}

	// 4) Persist both of them and continue.
	return c.updateTrustedHeaderAndNextVals(h, nextVals)
}

// Start starts two processes: 1) auto updating 2) removing outdated headers.
func (c *Client) Start() error {
	c.logger.Info("Starting light client")

	if c.removeNoLongerTrustedHeadersPeriod > 0 {
		c.routinesWaitGroup.Add(1)
		go c.removeNoLongerTrustedHeadersRoutine()
	}

	if c.updatePeriod > 0 {
		c.routinesWaitGroup.Add(1)
		go c.autoUpdateRoutine()
	}

	return nil
}

// Stop stops two processes: 1) auto updating 2) removing outdated headers.
// Stop only returns after both of them are finished running. If you wish to
// remove all the data, call Cleanup.
func (c *Client) Stop() {
	c.logger.Info("Stopping light client")
	close(c.quit)
	c.routinesWaitGroup.Wait()
}

// TrustedHeader returns a trusted header at the given height (0 - the latest).
// If a header is missing in trustedStore (e.g. it was skipped during
// bisection), it will be downloaded from primary.
//
// Headers along with validator sets, which can't be trusted anymore, are
// removed once a day (can be changed with RemoveNoLongerTrustedHeadersPeriod
// option).
// .
// height must be >= 0.
//
// It returns an error if:
//  - header expired, therefore can't be trusted (ErrOldHeaderExpired);
//  - there are some issues with the trusted store, although that should not
//  happen normally;
//  - negative height is passed;
//  - header has not been verified yet
//
// Safe for concurrent use by multiple goroutines.
func (c *Client) TrustedHeader(height int64, now time.Time) (*types.SignedHeader, error) {
	if height < 0 {
		return nil, errors.New("negative height")
	}

	// 1) Get latest height.
	latestHeight, err := c.LastTrustedHeight()
	if err != nil {
		return nil, err
	}
	if latestHeight == -1 {
		return nil, errors.New("no headers exist")
	}
	if height > latestHeight {
		return nil, errors.Errorf("unverified header requested (latest: %d)", latestHeight)
	}
	if height == 0 {
		height = latestHeight
	}

	// 2) Get header from store.
	h, err := c.trustedStore.SignedHeader(height)
	if err != nil {
		return nil, err
	}

	// 3) Ensure header can still be trusted.
	if HeaderExpired(h, c.trustingPeriod, now) {
		return nil, ErrOldHeaderExpired{h.Time.Add(c.trustingPeriod), now}
	}

	return h, nil
}

// TrustedValidatorSet returns a trusted validator set at the given height. If
// a validator set is missing in trustedStore (e.g. the associated header was
// skipped during bisection), it will be downloaded from primary. The second
// return parameter is height validator set corresponds to (useful when you
// pass 0).
//
// height must be >= 0.
//
// Headers along with validator sets, which can't be trusted anymore, are
// removed once a day (can be changed with RemoveNoLongerTrustedHeadersPeriod
// option).
//
// It returns an error if:
//	- header signed by that validator set expired (ErrOldHeaderExpired)
//  - there are some issues with the trusted store, although that should not
//  happen normally;
//  - negative height is passed;
//  - header signed by that validator set has not been verified yet
//
// Safe for concurrent use by multiple goroutines.
func (c *Client) TrustedValidatorSet(height int64, now time.Time) (*types.ValidatorSet, error) {
	// Checks height is positive and header (note: height - 1) is not expired.
	// Additionally, it fetches validator set from primary if it's missing in
	// store.
	_, err := c.TrustedHeader(height-1, now)
	if err != nil {
		return nil, err
	}

	return c.trustedStore.ValidatorSet(height)
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

// VerifyHeaderAtHeight fetches header and validators at the given height
// and calls VerifyHeader. It returns header immediately if such exists in
// trustedStore (no verification is needed).
//
// It returns provider.ErrSignedHeaderNotFound if header is not found by
// primary.
// It returns ErrOldHeaderExpired if header expired.
func (c *Client) VerifyHeaderAtHeight(height int64, now time.Time) (*types.SignedHeader, error) {
	if height <= 0 {
		return nil, errors.New("negative or zero height")
	}

	h, err := c.TrustedHeader(height, now)
	switch err.(type) {
	case nil: // Return already trusted header
		c.logger.Info("Header has already been verified", "height", height, "hash", hash2str(h.Hash()))
		return h, nil
	case ErrOldHeaderExpired:
		return nil, err
	}

	// Request the header and the vals.
	newHeader, newVals, err := c.fetchHeaderAndValsAtHeight(height)
	if err != nil {
		return nil, err
	}

	return newHeader, c.verifyHeader(newHeader, newVals, now)
}

// VerifyHeader verifies new header against the trusted state. It returns
// immediately if newHeader exists in trustedStore (no verification is
// needed).
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
// It returns ErrOldHeaderExpired if newHeader expired.
//
// If, at any moment, SignedHeader or ValidatorSet are not found by the primary
// provider, provider.ErrSignedHeaderNotFound /
// provider.ErrValidatorSetNotFound error is returned.
//
// NOTE: although newVals is entered as input, trustedStore will only store the
// validator set at height newHeader.Height+1 (i.e.
// newHeader.NextValidatorsHash).
func (c *Client) VerifyHeader(newHeader *types.SignedHeader, newVals *types.ValidatorSet, now time.Time) error {
	h, err := c.TrustedHeader(newHeader.Height, now)
	switch err.(type) {
	case nil: // Return already trusted header
		// Make sure it's the same header.
		if !bytes.Equal(h.Hash(), newHeader.Hash()) {
			return errors.Errorf("existing trusted header %X does not match newHeader %X", h.Hash(), newHeader.Hash())
		}
		c.logger.Info("Header has already been verified",
			"height", newHeader.Height, "hash", hash2str(newHeader.Hash()))
		return nil
	case ErrOldHeaderExpired:
		return err
	}

	return c.verifyHeader(newHeader, newVals, now)
}

func (c *Client) verifyHeader(newHeader *types.SignedHeader, newVals *types.ValidatorSet, now time.Time) error {
	c.logger.Info("VerifyHeader", "height", newHeader.Height, "hash", hash2str(newHeader.Hash()),
		"vals", hash2str(newVals.Hash()))

	var err error

	// 1) If going forward, perform either bisection or sequential verification
	if newHeader.Height >= c.latestTrustedHeader.Height {
		switch c.verificationMode {
		case sequential:
			err = c.sequence(c.latestTrustedHeader, c.latestTrustedNextVals, newHeader, newVals, now)
		case skipping:
			err = c.bisection(c.latestTrustedHeader, c.latestTrustedNextVals, newHeader, newVals, now)
		default:
			panic(fmt.Sprintf("Unknown verification mode: %b", c.verificationMode))
		}
	} else {
		// 2) Otherwise, perform backwards verification
		// Find the closest trusted header after newHeader.Height
		var closestHeader *types.SignedHeader
		closestHeader, err = c.trustedStore.SignedHeaderAfter(newHeader.Height)
		if err != nil {
			return errors.Wrapf(err, "can't get signed header after height %d", newHeader.Height)
		}

		err = c.backwards(closestHeader, newHeader, now)
	}
	if err != nil {
		c.logger.Error("Can't verify", "err", err)
		return err
	}

	if err := c.compareNewHeaderWithWitnesses(newHeader); err != nil {
		c.logger.Error("Error when comparing new header with witnesses", "err", err)
		return err
	}

	// Update trusted header and vals.
	nextVals, err := c.validatorSetFromPrimary(newHeader.Height + 1)
	if err != nil {
		return err
	}

	return c.updateTrustedHeaderAndNextVals(newHeader, nextVals)
}

// Primary returns the primary provider.
//
// NOTE: provider may be not safe for concurrent access.
func (c *Client) Primary() provider.Provider {
	c.providerMutex.Lock()
	defer c.providerMutex.Unlock()
	return c.primary
}

// Witnesses returns the witness providers.
//
// NOTE: providers may be not safe for concurrent access.
func (c *Client) Witnesses() []provider.Provider {
	c.providerMutex.Lock()
	defer c.providerMutex.Unlock()
	return c.witnesses
}

// Cleanup removes all the data (headers and validator sets) stored. Note: the
// client must be stopped at this point.
func (c *Client) Cleanup() error {
	c.logger.Info("Removing all the data")
	return c.cleanup(0)
}

// cleanup deletes all headers & validator sets between +stopHeight+ and latest
// height included. It also sets trustedHeader (vals) to the latest header
// (vals) if such exists.
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
	if stopHeight < oldestHeight {
		stopHeight = oldestHeight
	}
	for height := stopHeight; height <= latestHeight; height++ {
		err = c.trustedStore.DeleteSignedHeaderAndNextValidatorSet(height)
		if err != nil {
			c.logger.Error("can't remove a trusted header & validator set", "err", err, "height", height)
			continue
		}
	}

	c.latestTrustedHeader = nil
	c.latestTrustedNextVals = nil
	err = c.restoreTrustedHeaderAndNextVals()
	if err != nil {
		return err
	}

	return nil
}

// see VerifyHeader
func (c *Client) sequence(
	trustedHeader *types.SignedHeader,
	trustedNextVals *types.ValidatorSet,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {
	// 1) Verify any intermediate headers.
	var (
		interimHeader   *types.SignedHeader
		interimNextVals *types.ValidatorSet
		err             error
	)
	for height := trustedHeader.Height + 1; height < newHeader.Height; height++ {
		interimHeader, err = c.signedHeaderFromPrimary(height)
		if err != nil {
			return errors.Wrapf(err, "failed to obtain the header #%d", height)
		}

		c.logger.Debug("Verify newHeader against trustedHeader",
			"trustedHeight", c.latestTrustedHeader.Height,
			"trustedHash", hash2str(c.latestTrustedHeader.Hash()),
			"newHeight", interimHeader.Height,
			"newHash", hash2str(interimHeader.Hash()))
		err = VerifyAdjacent(c.chainID, trustedHeader, interimHeader, trustedNextVals,
			c.trustingPeriod, now)
		if err != nil {
			return errors.Wrapf(err, "failed to verify the header #%d", height)
		}

		// Update trusted header and vals.
		if height == newHeader.Height-1 {
			interimNextVals = newVals
		} else {
			interimNextVals, err = c.validatorSetFromPrimary(height + 1)
			if err != nil {
				return errors.Wrapf(err, "failed to obtain the vals #%d", height+1)
			}
		}
		err = c.updateTrustedHeaderAndNextVals(interimHeader, interimNextVals)
		if err != nil {
			return errors.Wrapf(err, "failed to update trusted state #%d", height)
		}
		trustedHeader, trustedNextVals = interimHeader, interimNextVals
	}

	// 2) Verify the new header.
	return VerifyAdjacent(c.chainID, c.latestTrustedHeader, newHeader, newVals, c.trustingPeriod, now)
}

// see VerifyHeader
func (c *Client) bisection(
	trustedHeader *types.SignedHeader, // height h
	trustedNextVals *types.ValidatorSet, // height h + 1
	newHeader *types.SignedHeader, // height g
	newVals *types.ValidatorSet, // height g
	now time.Time) error {

	interimVals := newVals
	interimHeader := newHeader

	for trustedHeader.Height < newHeader.Height {
		c.logger.Debug("Verify newHeader against trustedHeader",
			"trustedHeight", trustedHeader.Height,
			"trustedHash", hash2str(trustedHeader.Hash()),
			"newHeight", newHeader.Height,
			"newHash", hash2str(newHeader.Hash()))
		err := Verify(c.chainID, trustedHeader, trustedNextVals, interimHeader, interimVals, c.trustingPeriod, now,
			c.trustLevel)
		switch err.(type) {
		case nil:
			// Update the lower bound to the previous upper bound
			trustedHeader = interimHeader
			trustedNextVals, err = c.validatorSetFromPrimary(interimHeader.Height + 1)
			if err != nil {
				return err
			}
			if !bytes.Equal(trustedHeader.NextValidatorsHash, trustedNextVals.Hash()) {
				return errors.Errorf("expected next validator's hash %X, but got %X (height #%d)",
					trustedHeader.NextValidatorsHash,
					trustedNextVals.Hash(),
					trustedHeader.Height)
			}

			err = c.updateTrustedHeaderAndNextVals(trustedHeader, trustedNextVals)
			if err != nil {
				return err
			}

			// Update the upper bound to the untrustedHeader
			interimHeader, interimVals = newHeader, newVals

		case ErrNewValSetCantBeTrusted:
			pivotHeight := (interimHeader.Height + trustedHeader.Height) / 2
			interimHeader, interimVals, err = c.fetchHeaderAndValsAtHeight(pivotHeight)
			if err != nil {
				return err
			}

		default:
			return errors.Wrapf(err, "failed to verify the header #%d", newHeader.Height)
		}
	}

	return nil
}

// persist header and next validators to trustedStore.
func (c *Client) updateTrustedHeaderAndNextVals(h *types.SignedHeader, nextVals *types.ValidatorSet) error {
	if !bytes.Equal(h.NextValidatorsHash, nextVals.Hash()) {
		return errors.Errorf("expected next validator's hash %X, but got %X", h.NextValidatorsHash, nextVals.Hash())
	}

	if err := c.trustedStore.SaveSignedHeaderAndNextValidatorSet(h, nextVals); err != nil {
		return errors.Wrap(err, "failed to save trusted header")
	}

	c.latestTrustedHeader = h
	c.latestTrustedNextVals = nextVals

	return nil
}

// fetch header and validators for the given height (0 - latest) from primary
// provider.
func (c *Client) fetchHeaderAndValsAtHeight(height int64) (*types.SignedHeader, *types.ValidatorSet, error) {
	h, err := c.signedHeaderFromPrimary(height)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to obtain the header #%d", height)
	}
	vals, err := c.validatorSetFromPrimary(height)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to obtain the vals #%d", height)
	}
	return h, vals, nil
}

// Backwards verification (see VerifyHeaderBackwards func in the spec)
func (c *Client) backwards(trustedHeader *types.SignedHeader, newHeader *types.SignedHeader,
	now time.Time) error {
	var (
		interimHeader *types.SignedHeader
		err           error
	)

	for trustedHeader.Height > newHeader.Height {
		interimHeader, err = c.signedHeaderFromPrimary(trustedHeader.Height - 1)
		if err != nil {
			return errors.Wrapf(err, "failed to obtain the header at height #%d", trustedHeader.Height-1)
		}

		if err := interimHeader.ValidateBasic(c.chainID); err != nil {
			return errors.Wrap(err, "untrustedHeader.ValidateBasic failed")
		}

		if !interimHeader.Time.Before(trustedHeader.Time) {
			return errors.Errorf("expected older header time %v to be before newer header time %v",
				interimHeader.Time,
				trustedHeader.Time)
		}

		if HeaderExpired(interimHeader, c.trustingPeriod, now) {
			return ErrOldHeaderExpired{interimHeader.Time.Add(c.trustingPeriod), now}
		}

		if !bytes.Equal(interimHeader.Hash(), trustedHeader.LastBlockID.Hash) {
			return errors.Errorf("older header hash %X does not match trusted header's last block %X",
				interimHeader.Hash(),
				trustedHeader.LastBlockID.Hash)
		}

		trustedHeader = interimHeader
	}

	return nil
}

// compare header with all witnesses provided.
func (c *Client) compareNewHeaderWithWitnesses(h *types.SignedHeader) error {
	c.providerMutex.Lock()
	defer c.providerMutex.Unlock()

	// 1. Make sure AT LEAST ONE witness returns the same header.
	headerMatched := false
	witnessesToRemove := make([]int, 0)
	for attempt := uint16(1); attempt <= c.maxRetryAttempts; attempt++ {
		if len(c.witnesses) == 0 {
			return errors.New("could not find any witnesses. please reset the light client")
		}

		for i, witness := range c.witnesses {
			altH, err := witness.SignedHeader(h.Height)
			if err != nil {
				c.logger.Error("Failed to get a header from witness", "height", h.Height, "witness", witness)
				continue
			}

			if err = altH.ValidateBasic(c.chainID); err != nil {
				c.logger.Error("Witness sent us incorrect header", "err", err, "witness", witness)
				witnessesToRemove = append(witnessesToRemove, i)
				continue
			}

			if !bytes.Equal(h.Hash(), altH.Hash()) {
				if err = c.latestTrustedNextVals.VerifyCommitTrusting(c.chainID, altH.Commit.BlockID,
					altH.Height, altH.Commit, c.trustLevel); err != nil {
					c.logger.Error("Witness sent us incorrect header", "err", err, "witness", witness)
					witnessesToRemove = append(witnessesToRemove, i)
					continue
				}

				// TODO: send the diverged headers to primary && all witnesses

				return errors.Errorf(
					"header hash %X does not match one %X from the witness %v",
					h.Hash(), altH.Hash(), witness)
			}

			headerMatched = true
		}

		for _, idx := range witnessesToRemove {
			c.removeWitness(idx)
		}
		witnessesToRemove = make([]int, 0)

		if headerMatched {
			return nil
		}

		// 2. Otherwise, sleep
		time.Sleep(backoffTimeout(attempt))
	}

	return errors.New("awaiting response from all witnesses exceeded dropout time")
}

// NOTE: requires a providerMutex locked.
func (c *Client) removeWitness(idx int) {
	switch len(c.witnesses) {
	case 0:
		panic(fmt.Sprintf("wanted to remove %d element from empty witnesses slice", idx))
	case 1:
		c.witnesses = make([]provider.Provider, 0)
	default:
		c.witnesses[idx] = c.witnesses[len(c.witnesses)-1]
		c.witnesses = c.witnesses[:len(c.witnesses)-1]
	}
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
	oldestHeight, err := c.FirstTrustedHeight()
	if err != nil {
		c.logger.Error("can't get first trusted height", "err", err)
		return
	}
	if oldestHeight == -1 { // no headers yet => wait
		return
	}

	// 2) Get the latest height.
	latestHeight, err := c.LastTrustedHeight()
	if err != nil {
		c.logger.Error("can't get last trusted height", "err", err)
		return
	}
	if latestHeight == -1 { // no headers yet => wait
		return
	}

	// 3) Remove all headers that are outside of the trusting period.
	//
	// NOTE: even the latest header can be removed. it's okay because
	// c.latestTrustedHeader will retain it in memory so other funcs like VerifyHeader
	// don't crash.
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

		c.logger.Info("Advanced to new state", "height", latestHeader.Height, "hash", hash2str(latestHeader.Hash()))
	}

	return nil
}

// replaceProvider takes the first alternative provider and promotes it as the
// primary provider.
func (c *Client) replacePrimaryProvider() error {
	c.providerMutex.Lock()
	defer c.providerMutex.Unlock()

	if len(c.witnesses) <= 1 {
		return errors.Errorf("only one witness left. please reset the light client")
	}
	c.primary = c.witnesses[0]
	c.witnesses = c.witnesses[1:]
	c.logger.Info("New primary", "p", c.primary)

	return nil
}

// signedHeaderFromPrimary retrieves the SignedHeader from the primary provider
// at the specified height. Handles dropout by the primary provider by swapping
// with an alternative provider.
func (c *Client) signedHeaderFromPrimary(height int64) (*types.SignedHeader, error) {
	for attempt := uint16(1); attempt <= c.maxRetryAttempts; attempt++ {
		c.providerMutex.Lock()
		h, err := c.primary.SignedHeader(height)
		c.providerMutex.Unlock()
		if err == nil {
			// sanity check
			if height > 0 && h.Height != height {
				return nil, errors.Errorf("expected %d height, got %d", height, h.Height)
			}
			return h, nil
		}
		if err == provider.ErrSignedHeaderNotFound {
			return nil, err
		}
		time.Sleep(backoffTimeout(attempt))
	}

	c.logger.Info("Primary is unavailable. Replacing with the first witness")
	err := c.replacePrimaryProvider()
	if err != nil {
		return nil, err
	}

	return c.signedHeaderFromPrimary(height)
}

// validatorSetFromPrimary retrieves the ValidatorSet from the primary provider
// at the specified height. Handles dropout by the primary provider after 5
// attempts by replacing it with an alternative provider.
func (c *Client) validatorSetFromPrimary(height int64) (*types.ValidatorSet, error) {
	for attempt := uint16(1); attempt <= c.maxRetryAttempts; attempt++ {
		c.providerMutex.Lock()
		vals, err := c.primary.ValidatorSet(height)
		c.providerMutex.Unlock()
		if err == nil || err == provider.ErrValidatorSetNotFound {
			return vals, err
		}
		time.Sleep(backoffTimeout(attempt))
	}

	c.logger.Info("Primary is unavailable. Replacing with the first witness")
	err := c.replacePrimaryProvider()
	if err != nil {
		return nil, err
	}

	return c.validatorSetFromPrimary(height)
}

// exponential backoff (with jitter)
//		0.5s -> 2s -> 4.5s -> 8s -> 12.5 with 1s variation
func backoffTimeout(attempt uint16) time.Duration {
	return time.Duration(500*attempt*attempt)*time.Millisecond + time.Duration(rand.Intn(1000))*time.Millisecond
}

func hash2str(hash []byte) string {
	return fmt.Sprintf("%X", hash)
}
