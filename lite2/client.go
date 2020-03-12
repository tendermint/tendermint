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

	defaultPruningSize      = 1000
	defaultMaxRetryAttempts = 10
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

// PruningSize option sets the maximum amount of headers & validator set pairs
// that the light client stores. When Prune() is run, all headers (along with
// the associated validator sets) that are earlier than the h amount of headers
// will be removed from the store. Default: 1000. A pruning size of 0 will not
// prune the lite client at all.
func PruningSize(h uint16) Option {
	return func(c *Client) {
		c.pruningSize = h
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
	// Highest validator set from the store (height=H).
	latestTrustedVals *types.ValidatorSet

	// See RemoveNoLongerTrustedHeadersPeriod option
	pruningSize uint16
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
		c.logger.Info("Checking trusted header using options")
		if err := c.checkTrustedHeaderUsingOptions(trustOptions); err != nil {
			return nil, err
		}
	}

	if c.latestTrustedHeader == nil || c.latestTrustedHeader.Height < trustOptions.Height {
		c.logger.Info("Downloading trusted header using options")
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
		chainID:          chainID,
		trustingPeriod:   trustingPeriod,
		verificationMode: skipping,
		trustLevel:       DefaultTrustLevel,
		maxRetryAttempts: defaultMaxRetryAttempts,
		primary:          primary,
		witnesses:        witnesses,
		trustedStore:     trustedStore,
		pruningSize:      defaultPruningSize,
		confirmationFn:   func(action string) bool { return true },
		quit:             make(chan struct{}),
		logger:           log.NewNopLogger(),
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

	if err := c.restoreTrustedHeaderAndVals(); err != nil {
		return nil, err
	}

	return c, nil
}

// restoreTrustedHeaderAndVals loads trustedHeader and trustedVals from
// trustedStore.
func (c *Client) restoreTrustedHeaderAndVals() error {
	lastHeight, err := c.trustedStore.LastSignedHeaderHeight()
	if err != nil {
		return errors.Wrap(err, "can't get last trusted header height")
	}

	if lastHeight > 0 {
		trustedHeader, err := c.trustedStore.SignedHeader(lastHeight)
		if err != nil {
			return errors.Wrap(err, "can't get last trusted header")
		}

		trustedVals, err := c.trustedStore.ValidatorSet(lastHeight)
		if err != nil {
			return errors.Wrap(err, "can't get last trusted validators")
		}

		c.latestTrustedHeader = trustedHeader
		c.latestTrustedVals = trustedVals

		c.logger.Info("Restored trusted header and vals", "height", lastHeight)
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
			err := c.cleanupAfter(options.Height)
			if err != nil {
				return errors.Wrapf(err, "cleanupAfter(%d)", options.Height)
			}

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

// initializeWithTrustOptions fetches the weakly-trusted header and vals from
// primary provider. The header is cross-checked with witnesses for additional
// security.
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

	err = c.compareNewHeaderWithWitnesses(h)
	if err != nil {
		return err
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

	// 3) Persist both of them and continue.
	return c.updateTrustedHeaderAndVals(h, vals)
}

// TrustedHeader returns a trusted header at the given height (0 - the latest).
//
// Headers along with validator sets, which can't be trusted anymore, are
// removed once a day (can be changed with RemoveNoLongerTrustedHeadersPeriod
// option).
// .
// height must be >= 0.
//
// It returns an error if:
//  - there are some issues with the trusted store, although that should not
//  happen normally;
//  - negative height is passed;
//  - header has not been verified yet and is therefore not in the store
//
// Safe for concurrent use by multiple goroutines.
func (c *Client) TrustedHeader(height int64) (*types.SignedHeader, error) {
	height, err := c.compareWithLatestHeight(height)
	if err != nil {
		return nil, err
	}
	return c.trustedStore.SignedHeader(height)
}

// TrustedValidatorSet returns a trusted validator set at the given height (0 -
// latest). The second return parameter is the height used (useful if 0 was
// passed; otherwise can be ignored).
//
// height must be >= 0.
//
// Headers along with validator sets are
// removed once a day (can be changed with RemoveNoLongerTrustedHeadersPeriod
// option).
//
// Function returns an error if:
//  - there are some issues with the trusted store, although that should not
//  happen normally;
//  - negative height is passed;
//  - header signed by that validator set has not been verified yet
//
// Safe for concurrent use by multiple goroutines.
func (c *Client) TrustedValidatorSet(height int64) (valSet *types.ValidatorSet, heightUsed int64, err error) {
	heightUsed, err = c.compareWithLatestHeight(height)
	if err != nil {
		return nil, heightUsed, err
	}
	valSet, err = c.trustedStore.ValidatorSet(heightUsed)
	if err != nil {
		return nil, heightUsed, err
	}
	return valSet, heightUsed, err
}

func (c *Client) compareWithLatestHeight(height int64) (int64, error) {
	latestHeight, err := c.LastTrustedHeight()
	if err != nil {
		return 0, errors.Wrap(err, "can't get last trusted height")
	}
	if latestHeight == -1 {
		return 0, errors.New("no headers exist")
	}

	switch {
	case height > latestHeight:
		return 0, errors.Errorf("unverified header/valset requested (latest: %d)", latestHeight)
	case height == 0:
		return latestHeight, nil
	case height < 0:
		return 0, errors.New("negative height")
	}

	return height, nil
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
// height must be > 0.
//
// It returns provider.ErrSignedHeaderNotFound if header is not found by
// primary.
func (c *Client) VerifyHeaderAtHeight(height int64, now time.Time) (*types.SignedHeader, error) {
	if height <= 0 {
		return nil, errors.New("negative or zero height")
	}

	// Check if header already verified.
	h, err := c.TrustedHeader(height)
	if err == nil {
		c.logger.Info("Header has already been verified", "height", height, "hash", hash2str(h.Hash()))
		// Return already trusted header
		return h, nil
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
// It returns ErrOldHeaderExpired if the latest trusted header expired.
//
// If the primary provides an invalid header (ErrInvalidHeader), it is rejected
// and replaced by another provider until all are exhausted.
//
// If, at any moment, SignedHeader or ValidatorSet are not found by the primary
// provider, provider.ErrSignedHeaderNotFound /
// provider.ErrValidatorSetNotFound error is returned.
func (c *Client) VerifyHeader(newHeader *types.SignedHeader, newVals *types.ValidatorSet, now time.Time) error {
	if newHeader.Height <= 0 {
		return errors.New("negative or zero height")
	}

	// Check if newHeader already verified.
	h, err := c.TrustedHeader(newHeader.Height)
	if err == nil {
		// Make sure it's the same header.
		if !bytes.Equal(h.Hash(), newHeader.Hash()) {
			return errors.Errorf("existing trusted header %X does not match newHeader %X", h.Hash(), newHeader.Hash())
		}
		c.logger.Info("Header has already been verified",
			"height", newHeader.Height, "hash", hash2str(newHeader.Hash()))
		return nil
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
			err = c.sequence(c.latestTrustedHeader, newHeader, newVals, now)
		case skipping:
			err = c.bisection(c.latestTrustedHeader, c.latestTrustedVals, newHeader, newVals, now)
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

	return c.updateTrustedHeaderAndVals(newHeader, newVals)
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
	c.latestTrustedHeader = nil
	c.latestTrustedVals = nil
	return c.trustedStore.Prune(0)
}

// cleanupAfter deletes all headers & validator sets after +height+. It also
// resets latestTrustedHeader to the latest header.
func (c *Client) cleanupAfter(height int64) error {
	nextHeight := height

	for {
		h, err := c.trustedStore.SignedHeaderAfter(nextHeight)
		if err == store.ErrSignedHeaderNotFound {
			break
		} else if err != nil {
			return errors.Wrapf(err, "failed to get header after %d", nextHeight)
		}

		err = c.trustedStore.DeleteSignedHeaderAndValidatorSet(h.Height)
		if err != nil {
			c.logger.Error("can't remove a trusted header & validator set", "err", err,
				"height", h.Height)
		}

		nextHeight = h.Height
	}

	c.latestTrustedHeader = nil
	c.latestTrustedVals = nil
	err := c.restoreTrustedHeaderAndVals()
	if err != nil {
		return err
	}

	return nil
}

// see VerifyHeader
func (c *Client) sequence(
	initiallyTrustedHeader *types.SignedHeader,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	var (
		trustedHeader = initiallyTrustedHeader

		interimHeader *types.SignedHeader
		interimVals   *types.ValidatorSet

		err error
	)

	for height := initiallyTrustedHeader.Height + 1; height <= newHeader.Height; height++ {
		// 1) Fetch interim headers and vals if needed.
		if height == newHeader.Height { // last header
			interimHeader, interimVals = newHeader, newVals
		} else { // intermediate headers
			interimHeader, interimVals, err = c.fetchHeaderAndValsAtHeight(height)
			if err != nil {
				return errors.Wrapf(err, "failed to obtain the header #%d", height)
			}
		}

		// 2) Verify them
		c.logger.Debug("Verify newHeader against trustedHeader",
			"trustedHeight", trustedHeader.Height,
			"trustedHash", hash2str(trustedHeader.Hash()),
			"newHeight", interimHeader.Height,
			"newHash", hash2str(interimHeader.Hash()))

		err = VerifyAdjacent(c.chainID, trustedHeader, interimHeader, interimVals,
			c.trustingPeriod, now)
		if err != nil {
			err = errors.Wrapf(err, "verify adjacent from #%d to #%d failed",
				trustedHeader.Height, interimHeader.Height)

			switch errors.Cause(err).(type) {
			case ErrInvalidHeader:
				c.logger.Error("primary sent invalid header -> replacing", "err", err)
				replaceErr := c.replacePrimaryProvider()
				if replaceErr != nil {
					c.logger.Error("Can't replace primary", "err", replaceErr)
					return err // return original error
				}
				// attempt to verify header again
				height--
				continue
			default:
				return err
			}
		}

		// 3) Update trustedHeader
		trustedHeader = interimHeader
	}

	return nil
}

// see VerifyHeader
func (c *Client) bisection(
	initiallyTrustedHeader *types.SignedHeader,
	initiallyTrustedVals *types.ValidatorSet,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	type HeaderSet struct {
		sh  *types.SignedHeader
		val *types.ValidatorSet
	}

	var (
		headerCache = []HeaderSet{{newHeader, newVals}}
		depth       = 0

		trustedHeader = initiallyTrustedHeader
		trustedVals   = initiallyTrustedVals
	)

	for {
		c.logger.Debug("Verify newHeader against trustedHeader",
			"trustedHeight", trustedHeader.Height,
			"trustedHash", hash2str(trustedHeader.Hash()),
			"newHeight", headerCache[depth].sh.Height,
			"newHash", hash2str(headerCache[depth].sh.Hash()))

		err := Verify(c.chainID, trustedHeader, trustedVals, headerCache[depth].sh, headerCache[depth].val,
			c.trustingPeriod, now, c.trustLevel)
		switch err.(type) {
		case nil:
			if depth == 0 {
				return nil
			}

			// Update the lower bound to the previous upper bound
			trustedHeader, trustedVals = headerCache[depth].sh, headerCache[depth].val
			// Remove the untrusted header at the lower bound in the header cache
			headerCache = headerCache[:depth]
			// Update the upper bound to the untrustedHeader by setting depth to 0
			depth = 0

		case ErrNewValSetCantBeTrusted:
			if depth == len(headerCache)-1 {
				pivotHeight := (headerCache[depth].sh.Height + trustedHeader.Height) / 2
				interimHeader, interimVals, err := c.fetchHeaderAndValsAtHeight(pivotHeight)
				if err != nil {
					return err
				}
				depth++
				headerCache = append(headerCache, HeaderSet{interimHeader, interimVals})
			} else { // depth is less than length - 1
				depth++
			}

		case ErrInvalidHeader:
			c.logger.Error("primary sent invalid header -> replacing", "err", err)
			replaceErr := c.replacePrimaryProvider()
			if replaceErr != nil {
				c.logger.Error("Can't replace primary", "err", replaceErr)
				// return original error
				return errors.Wrapf(err, "verify from #%d to #%d failed",
					trustedHeader.Height, headerCache[depth].sh.Height)
			}
			// attempt to verify the header again
			continue

		default:
			return errors.Wrapf(err, "verify from #%d to #%d failed",
				trustedHeader.Height, headerCache[depth].sh.Height)
		}
	}
}

func (c *Client) updateTrustedHeaderAndVals(h *types.SignedHeader, vals *types.ValidatorSet) error {
	if !bytes.Equal(h.ValidatorsHash, vals.Hash()) {
		return errors.Errorf("expected validator's hash %X, but got %X", h.ValidatorsHash, vals.Hash())
	}

	if err := c.trustedStore.SaveSignedHeaderAndValidatorSet(h, vals); err != nil {
		return errors.Wrap(err, "failed to save trusted header")
	}

	if c.pruningSize > 0 {
		if err := c.trustedStore.Prune(c.pruningSize); err != nil {
			return errors.Wrap(err, "prune")
		}
	}

	if c.latestTrustedHeader == nil || h.Height > c.latestTrustedHeader.Height {
		c.latestTrustedHeader = h
		c.latestTrustedVals = vals
	}

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

// backwards verification (see VerifyHeaderBackwards func in the spec) verifies
// headers before a trusted header. If a sent header is invalid the primary is
// replaced with another provider and the operation is repeated.
func (c *Client) backwards(
	initiallyTrustedHeader *types.SignedHeader,
	newHeader *types.SignedHeader,
	now time.Time) error {

	if HeaderExpired(initiallyTrustedHeader, c.trustingPeriod, now) {
		return ErrOldHeaderExpired{initiallyTrustedHeader.Time.Add(c.trustingPeriod), now}
	}

	var (
		trustedHeader = initiallyTrustedHeader
		interimHeader *types.SignedHeader
		err           error
	)

	for trustedHeader.Height > newHeader.Height {
		interimHeader, err = c.signedHeaderFromPrimary(trustedHeader.Height - 1)
		if err != nil {
			return errors.Wrapf(err, "failed to obtain the header at height #%d", trustedHeader.Height-1)
		}

		if err := VerifyBackwards(c.chainID, interimHeader, trustedHeader); err != nil {
			c.logger.Error("primary sent invalid header -> replacing", "err", err)
			if replaceErr := c.replacePrimaryProvider(); replaceErr != nil {
				c.logger.Error("Can't replace primary", "err", replaceErr)
				// return original error
				return errors.Wrapf(err, "verify backwards from %d to %d failed",
					trustedHeader.Height, interimHeader.Height)
			}
		}

		trustedHeader = interimHeader
	}

	// Initially trusted header might have expired at this point.
	if HeaderExpired(initiallyTrustedHeader, c.trustingPeriod, now) {
		return ErrOldHeaderExpired{initiallyTrustedHeader.Time.Add(c.trustingPeriod), now}
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
				if err = c.latestTrustedVals.VerifyCommitTrusting(c.chainID, altH.Commit.BlockID,
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

// Update attempts to advance the state by downloading the latest header and
// comparing it with the existing one. It returns a new header on a successful
// update. Otherwise, it returns nil (plus an error, if any).
func (c *Client) Update(now time.Time) (*types.SignedHeader, error) {
	lastTrustedHeight, err := c.LastTrustedHeight()
	if err != nil {
		return nil, errors.Wrap(err, "can't get last trusted height")
	}

	if lastTrustedHeight == -1 {
		// no headers yet => wait
		return nil, nil
	}

	latestHeader, latestVals, err := c.fetchHeaderAndValsAtHeight(0)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get latest header and vals")
	}

	if latestHeader.Height > lastTrustedHeight {
		err = c.VerifyHeader(latestHeader, latestVals, now)
		if err != nil {
			return nil, err
		}
		c.logger.Info("Advanced to new state", "height", latestHeader.Height, "hash", hash2str(latestHeader.Hash()))
		return latestHeader, nil
	}

	return nil, nil
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
		c.logger.Error("Failed to get signed header from primary", "attempt", attempt, "err", err)
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
		c.logger.Error("Failed to get validator set from primary", "attempt", attempt, "err", err)
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
