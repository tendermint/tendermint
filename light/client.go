package light

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/light/provider"
	"github.com/tendermint/tendermint/light/store"
	"github.com/tendermint/tendermint/types"
)

type mode byte

const (
	sequential mode = iota + 1
	skipping

	defaultPruningSize      = 1000
	defaultMaxRetryAttempts = 10
	// For bisection, when using the cache of headers from the previous batch,
	// they will always be at a height greater than 1/2 (normal bisection) so to
	// find something in between the range, 9/16 is used.
	bisectionNumerator   = 9
	bisectionDenominator = 16

	// 10s should cover most of the clients.
	// References:
	// - http://vancouver-webpages.com/time/web.html
	// - https://blog.codinghorror.com/keeping-time-on-the-pc/
	defaultMaxClockDrift = 10 * time.Second
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
// prune the light client at all.
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

// MaxClockDrift defines how much new (untrusted) header's Time can drift into
// the future. Default: 10s.
func MaxClockDrift(d time.Duration) Option {
	return func(c *Client) {
		c.maxClockDrift = d
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
	maxClockDrift    time.Duration

	// Mutex for locking during changes of the light clients providers
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

	quit chan struct{}

	logger log.Logger
}

// NewClient returns a new light client. It returns an error if it fails to
// obtain the header & vals from the primary or they are invalid (e.g. trust
// hash does not match with the one from the header).
//
// Witnesses are providers, which will be used for cross-checking the primary
// provider. At least one witness must be given when skipping verification is
// used (default). A witness can become a primary iff the current primary is
// unavailable.
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
		return nil, fmt.Errorf("invalid TrustOptions: %w", err)
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
		maxClockDrift:    defaultMaxClockDrift,
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
	if len(c.witnesses) < 1 && c.verificationMode == skipping {
		return nil, errNoWitnesses{}
	}

	// Verify witnesses are all on the same chain.
	for i, w := range witnesses {
		if w.ChainID() != chainID {
			return nil, fmt.Errorf("witness #%d: %v is on another chain %s, expected %s",
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
		return fmt.Errorf("can't get last trusted header height: %w", err)
	}

	if lastHeight > 0 {
		trustedHeader, err := c.trustedStore.SignedHeader(lastHeight)
		if err != nil {
			return fmt.Errorf("can't get last trusted header: %w", err)
		}

		trustedVals, err := c.trustedStore.ValidatorSet(lastHeight)
		if err != nil {
			return fmt.Errorf("can't get last trusted validators: %w", err)
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
				return fmt.Errorf("cleanupAfter(%d): %w", options.Height, err)
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
				return fmt.Errorf("failed to cleanup: %w", err)
			}
		} else {
			return errors.New("refused to remove the stored headers despite hashes mismatch")
		}
	}

	return nil
}

// initializeWithTrustOptions fetches the weakly-trusted header and vals from
// primary provider.
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
		return fmt.Errorf("expected header's hash %X, but got %X", options.Hash, h.Hash())
	}

	// 2) Fetch and verify the vals.
	vals, err := c.validatorSetFromPrimary(options.Height)
	if err != nil {
		return err
	}

	if !bytes.Equal(h.ValidatorsHash, vals.Hash()) {
		return fmt.Errorf("expected header's validators (%X) to match those that were supplied (%X)",
			h.ValidatorsHash,
			vals.Hash(),
		)
	}

	// Ensure that +2/3 of validators signed correctly.
	err = vals.VerifyCommitLight(c.chainID, h.Commit.BlockID, h.Height, h.Commit)
	if err != nil {
		return fmt.Errorf("invalid commit: %w", err)
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
		return 0, fmt.Errorf("can't get last trusted height: %w", err)
	}
	if latestHeight == -1 {
		return 0, errors.New("no headers exist")
	}

	switch {
	case height > latestHeight:
		return 0, fmt.Errorf("unverified header/valset requested (latest: %d)", latestHeight)
	case height == 0:
		return latestHeight, nil
	case height < 0:
		return 0, errors.New("negative height")
	}

	return height, nil
}

// VerifyHeaderAtHeight fetches header and validators at the given height
// and calls VerifyHeader. It returns header immediately if such exists in
// trustedStore (no verification is needed).
//
// height must be > 0.
//
// It returns provider.ErrSignedHeaderNotFound if header is not found by
// primary.
//
// It will replace the primary provider if an error from a request to the provider occurs
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
	newHeader, newVals, err := c.signedHeaderAndValSetFromPrimary(height)
	if err != nil {
		return nil, err
	}

	return newHeader, c.verifyHeader(newHeader, newVals, now)
}

// VerifyHeader verifies new header against the trusted state. It returns
// immediately if newHeader exists in trustedStore (no verification is
// needed). Else it performs one of the two types of verification:
//
// SequentialVerification: verifies that 2/3 of the trusted validator set has
// signed the new header. If the headers are not adjacent, **all** intermediate
// headers will be requested. Intermediate headers are not saved to database.
//
// SkippingVerification(trustLevel): verifies that {trustLevel} of the trusted
// validator set has signed the new header. If it's not the case and the
// headers are not adjacent, bisection is performed and necessary (not all)
// intermediate headers will be requested. See the specification for details.
// Intermediate headers are not saved to database.
// https://github.com/tendermint/spec/blob/master/spec/consensus/light-client.md
//
// If the header, which is older than the currently trusted header, is
// requested and the light client does not have it, VerifyHeader will perform:
//		a) bisection verification if nearest trusted header is found & not expired
//		b) backwards verification in all other cases
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
			return fmt.Errorf("existing trusted header %X does not match newHeader %X", h.Hash(), newHeader.Hash())
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

	// 1) If going forward, perform either bisection or sequential verification.
	if newHeader.Height >= c.latestTrustedHeader.Height {
		switch c.verificationMode {
		case sequential:
			err = c.sequence(c.latestTrustedHeader, newHeader, newVals, now)
		case skipping:
			err = c.bisectionAgainstPrimary(c.latestTrustedHeader, c.latestTrustedVals, newHeader, newVals, now)
		default:
			panic(fmt.Sprintf("Unknown verification mode: %b", c.verificationMode))
		}
	} else {
		// 2) If verifying before the first trusted header, perform backwards
		// verification.
		var (
			closestHeader     *types.SignedHeader
			firstHeaderHeight int64
		)
		firstHeaderHeight, err = c.FirstTrustedHeight()
		if err != nil {
			return fmt.Errorf("can't get first header height: %w", err)
		}
		if newHeader.Height < firstHeaderHeight {
			closestHeader, err = c.TrustedHeader(firstHeaderHeight)
			if err != nil {
				return fmt.Errorf("can't get first signed header: %w", err)
			}
			if HeaderExpired(closestHeader, c.trustingPeriod, now) {
				closestHeader = c.latestTrustedHeader
			}
			err = c.backwards(closestHeader, newHeader, now)
		} else {
			// 3) OR if between trusted headers where the nearest has not expired,
			// perform bisection verification, else backwards.
			closestHeader, err = c.trustedStore.SignedHeaderBefore(newHeader.Height)
			if err != nil {
				return fmt.Errorf("can't get signed header before height %d: %w", newHeader.Height, err)
			}
			var closestValidatorSet *types.ValidatorSet
			if c.verificationMode == sequential || HeaderExpired(closestHeader, c.trustingPeriod, now) {
				err = c.backwards(c.latestTrustedHeader, newHeader, now)
			} else {
				closestValidatorSet, _, err = c.TrustedValidatorSet(closestHeader.Height)
				if err != nil {
					return fmt.Errorf("can't get validator set at height %d: %w", closestHeader.Height, err)
				}
				err = c.bisectionAgainstPrimary(closestHeader, closestValidatorSet, newHeader, newVals, now)
			}
		}
	}
	if err != nil {
		c.logger.Error("Can't verify", "err", err)
		return err
	}

	// 4) Once verified, save and return
	return c.updateTrustedHeaderAndVals(newHeader, newVals)
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
			interimHeader, interimVals, err = c.signedHeaderAndValSetFromPrimary(height)
			if err != nil {
				return err
			}
		}

		// 2) Verify them
		c.logger.Debug("Verify adjacent newHeader against trustedHeader",
			"trustedHeight", trustedHeader.Height,
			"trustedHash", hash2str(trustedHeader.Hash()),
			"newHeight", interimHeader.Height,
			"newHash", hash2str(interimHeader.Hash()))

		err = VerifyAdjacent(c.chainID, trustedHeader, interimHeader, interimVals,
			c.trustingPeriod, now, c.maxClockDrift)
		if err != nil {
			err := fmt.Errorf("verify adjacent from #%d to #%d failed: %w",
				trustedHeader.Height, interimHeader.Height, err)

			switch errors.Unwrap(err).(type) {
			case ErrInvalidHeader:
				c.logger.Error("primary sent invalid header -> replacing", "err", err)
				replaceErr := c.replacePrimaryProvider()
				if replaceErr != nil {
					c.logger.Error("Can't replace primary", "err", replaceErr)
					// return original error
					return err
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
//
// Bisection finds the middle header between a trusted and new header,
// reiterating the action until it verifies a header. A cache of headers
// requested from source is kept such that when a verification is made, and the
// light client tries again to verify the new header in the middle, the light
// client does not need to ask for all the same headers again.
func (c *Client) bisection(
	source provider.Provider,
	initiallyTrustedHeader *types.SignedHeader,
	initiallyTrustedVals *types.ValidatorSet,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	type headerSet struct {
		sh     *types.SignedHeader
		valSet *types.ValidatorSet
	}

	var (
		headerCache = []headerSet{{newHeader, newVals}}
		depth       = 0

		trustedHeader = initiallyTrustedHeader
		trustedVals   = initiallyTrustedVals
	)

	for {
		c.logger.Debug("Verify non-adjacent newHeader against trustedHeader",
			"trustedHeight", trustedHeader.Height,
			"trustedHash", hash2str(trustedHeader.Hash()),
			"newHeight", headerCache[depth].sh.Height,
			"newHash", hash2str(headerCache[depth].sh.Hash()))

		err := Verify(c.chainID, trustedHeader, trustedVals, headerCache[depth].sh, headerCache[depth].valSet,
			c.trustingPeriod, now, c.maxClockDrift, c.trustLevel)
		switch err.(type) {
		case nil:
			// Have we verified the last header
			if depth == 0 {
				return nil
			}
			// If not, update the lower bound to the previous upper bound
			trustedHeader, trustedVals = headerCache[depth].sh, headerCache[depth].valSet
			// Remove the untrusted header at the lower bound in the header cache - it's no longer useful
			headerCache = headerCache[:depth]
			// Reset the cache depth so that we start from the upper bound again
			depth = 0

		case ErrNewValSetCantBeTrusted:
			// do add another header to the end of the cache
			if depth == len(headerCache)-1 {
				pivotHeight := trustedHeader.Height + (headerCache[depth].sh.Height-trustedHeader.
					Height)*bisectionNumerator/bisectionDenominator
				interimHeader, interimVals, err := c.signedHeaderAndValSetFrom(pivotHeight, source)
				if err != nil {
					return err
				}
				headerCache = append(headerCache, headerSet{interimHeader, interimVals})
			}
			depth++

		default:
			return fmt.Errorf("verify non adjacent from #%d to #%d failed: %w",
				trustedHeader.Height, headerCache[depth].sh.Height, err)
		}
	}
}

// bisectionAgainstPrimary does bisection plus it compares new header with
// witnesses and replaces primary if it does not respond after
// MaxRetryAttempts.
func (c *Client) bisectionAgainstPrimary(
	initiallyTrustedHeader *types.SignedHeader,
	initiallyTrustedVals *types.ValidatorSet,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	err := c.bisection(c.primary, initiallyTrustedHeader, initiallyTrustedVals, newHeader, newVals, now)

	switch errors.Unwrap(err).(type) {
	case ErrInvalidHeader:
		c.logger.Error("primary sent invalid header -> replacing", "err", err)
		replaceErr := c.replacePrimaryProvider()
		if replaceErr != nil {
			c.logger.Error("Can't replace primary", "err", replaceErr)
			// return original error
			return err
		}

		replacementHeader, replacementVals, fErr := c.signedHeaderAndValSetFromPrimary(newHeader.Height)
		if fErr != nil {
			c.logger.Error("Can't fetch header/vals from primary", "err", fErr)
			// return original error
			return err
		}

		if !bytes.Equal(replacementHeader.Hash(), newHeader.Hash()) ||
			!bytes.Equal(replacementVals.Hash(), newVals.Hash()) {
			c.logger.Error("Replacement provider has a different header/vals",
				"newHash", newHeader.Hash(),
				"newVals", newVals.Hash(),
				"replHash", replacementHeader.Hash(),
				"replVals", replacementVals.Hash())
			// return original error
			return err
		}

		// attempt to verify the header again
		return c.bisectionAgainstPrimary(
			initiallyTrustedHeader,
			initiallyTrustedVals,
			replacementHeader,
			replacementVals,
			now,
		)
	case nil:
		// Compare header with the witnesses to ensure it's not a fork.
		// More witnesses we have, more chance to notice one.
		//
		// CORRECTNESS ASSUMPTION: there's at least 1 correct full node
		// (primary or one of the witnesses).
		if cmpErr := c.compareNewHeaderWithWitnesses(newHeader, now); cmpErr != nil {
			return cmpErr
		}
	default:
		return err
	}

	return nil
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
	prevHeight := c.latestTrustedHeader.Height

	for {
		h, err := c.trustedStore.SignedHeaderBefore(prevHeight)
		if err == store.ErrSignedHeaderNotFound || (h != nil && h.Height <= height) {
			break
		} else if err != nil {
			return fmt.Errorf("failed to get header before %d: %w", prevHeight, err)
		}

		err = c.trustedStore.DeleteSignedHeaderAndValidatorSet(h.Height)
		if err != nil {
			c.logger.Error("can't remove a trusted header & validator set", "err", err,
				"height", h.Height)
		}

		prevHeight = h.Height
	}

	c.latestTrustedHeader = nil
	c.latestTrustedVals = nil
	err := c.restoreTrustedHeaderAndVals()
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) updateTrustedHeaderAndVals(h *types.SignedHeader, vals *types.ValidatorSet) error {
	if !bytes.Equal(h.ValidatorsHash, vals.Hash()) {
		return fmt.Errorf("expected validator's hash %X, but got %X", h.ValidatorsHash, vals.Hash())
	}

	if err := c.trustedStore.SaveSignedHeaderAndValidatorSet(h, vals); err != nil {
		return fmt.Errorf("failed to save trusted header: %w", err)
	}

	if c.pruningSize > 0 {
		if err := c.trustedStore.Prune(c.pruningSize); err != nil {
			return fmt.Errorf("prune: %w", err)
		}
	}

	if c.latestTrustedHeader == nil || h.Height > c.latestTrustedHeader.Height {
		c.latestTrustedHeader = h
		c.latestTrustedVals = vals
	}

	return nil
}

// 0 - latest header
// Note it swaps the primary with a witness if primary is not responding after
// MaxRetryAttempts.
func (c *Client) signedHeaderAndValSetFromPrimary(height int64) (*types.SignedHeader, *types.ValidatorSet, error) {
	h, err := c.signedHeaderFromPrimary(height)
	if err != nil {
		return nil, nil, fmt.Errorf("can't fetch header: %w", err)
	}
	vals, err := c.validatorSetFromPrimary(height)
	if err != nil {
		return nil, nil, fmt.Errorf("can't fetch vals: %w", err)
	}
	return h, vals, nil
}

// 0 - latest header
// Note it does not do retries nor swapping.
func (c *Client) signedHeaderAndValSetFromWitness(height int64,
	witness provider.Provider) (*types.SignedHeader, *types.ValidatorSet, *errBadWitness) {

	h, err := witness.SignedHeader(height)
	if err != nil {
		return nil, nil, &errBadWitness{err, noResponse, -1}
	}
	err = c.validateHeader(h, height)
	if err != nil {
		return nil, nil, &errBadWitness{err, invalidHeader, -1}
	}

	vals, err := witness.ValidatorSet(height)
	if err != nil {
		return nil, nil, &errBadWitness{err, noResponse, -1}
	}
	err = c.validateValidatorSet(vals)
	if err != nil {
		return nil, nil, &errBadWitness{err, invalidValidatorSet, -1}
	}

	return h, vals, nil
}

func (c *Client) signedHeaderAndValSetFrom(height int64,
	source provider.Provider) (*types.SignedHeader, *types.ValidatorSet, error) {

	c.providerMutex.Lock()
	sourceIsPrimary := (c.primary == source)
	c.providerMutex.Unlock()

	if sourceIsPrimary {
		return c.signedHeaderAndValSetFromPrimary(height)
	}
	return c.signedHeaderAndValSetFromWitness(height, source)
}

// backwards verification (see VerifyHeaderBackwards func in the spec) verifies
// headers before a trusted header. If a sent header is invalid the primary is
// replaced with another provider and the operation is repeated.
func (c *Client) backwards(
	initiallyTrustedHeader *types.SignedHeader,
	newHeader *types.SignedHeader,
	now time.Time) error {

	if HeaderExpired(initiallyTrustedHeader, c.trustingPeriod, now) {
		c.logger.Error("Header Expired")
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
			return fmt.Errorf("failed to obtain the header at height #%d: %w", trustedHeader.Height-1, err)
		}
		c.logger.Debug("Verify newHeader against trustedHeader",
			"trustedHeight", trustedHeader.Height,
			"trustedHash", hash2str(trustedHeader.Hash()),
			"newHeight", interimHeader.Height,
			"newHash", hash2str(interimHeader.Hash()))
		if err := VerifyBackwards(c.chainID, interimHeader, trustedHeader); err != nil {
			c.logger.Error("primary sent invalid header -> replacing", "err", err)
			if replaceErr := c.replacePrimaryProvider(); replaceErr != nil {
				c.logger.Error("Can't replace primary", "err", replaceErr)
				// return original error
				return fmt.Errorf("verify backwards from %d to %d failed: %w",
					trustedHeader.Height, interimHeader.Height, err)
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
func (c *Client) compareNewHeaderWithWitnesses(h *types.SignedHeader, now time.Time) error {
	c.providerMutex.Lock()
	defer c.providerMutex.Unlock()

	// 1. Make sure AT LEAST ONE witness returns the same header.
	var (
		headerMatched      bool
		lastErrConfHeaders error
	)
	for attempt := uint16(1); attempt <= c.maxRetryAttempts; attempt++ {
		if len(c.witnesses) == 0 {
			return errNoWitnesses{}
		}

		// launch one goroutine per witness
		errc := make(chan error, len(c.witnesses))
		for i, witness := range c.witnesses {
			go c.compareNewHeaderWithWitness(errc, h, witness, i, now)
		}

		witnessesToRemove := make([]int, 0)

		// handle errors as they come
		for i := 0; i < cap(errc); i++ {
			err := <-errc

			switch e := err.(type) {
			case nil: // at least one header matched
				headerMatched = true
			case ErrConflictingHeaders: // fork detected
				c.logger.Info("FORK DETECTED", "witness", e.Witness, "err", err)
				c.sendConflictingHeadersEvidence(&types.ConflictingHeadersEvidence{H1: h, H2: e.H2})
				lastErrConfHeaders = e
			case errBadWitness:
				c.logger.Info("Bad witness", "witness", c.witnesses[e.WitnessIndex], "err", err)
				// if witness sent us invalid header / vals, remove it
				if e.Code == invalidHeader || e.Code == invalidValidatorSet {
					c.logger.Info("Witness sent us invalid header / vals -> removing it", "witness", c.witnesses[e.WitnessIndex])
					witnessesToRemove = append(witnessesToRemove, e.WitnessIndex)
				}
			}
		}

		for _, idx := range witnessesToRemove {
			c.removeWitness(idx)
		}

		if lastErrConfHeaders != nil {
			// NOTE: all of the potential forks will be reported, but we only return
			// the last ErrConflictingHeaders error here.
			return lastErrConfHeaders
		} else if headerMatched {
			return nil
		}

		// 2. Otherwise, sleep
		time.Sleep(backoffTimeout(attempt))
	}

	return errors.New("awaiting response from all witnesses exceeded dropout time")
}

func (c *Client) compareNewHeaderWithWitness(errc chan error, h *types.SignedHeader,
	witness provider.Provider, witnessIndex int, now time.Time) {

	altH, altVals, err := c.signedHeaderAndValSetFromWitness(h.Height, witness)
	if err != nil {
		err.WitnessIndex = witnessIndex
		errc <- err
		return
	}

	if !bytes.Equal(h.Hash(), altH.Hash()) {
		if bsErr := c.bisection(witness, c.latestTrustedHeader, c.latestTrustedVals, altH, altVals, now); bsErr != nil {
			errc <- errBadWitness{bsErr, invalidHeader, witnessIndex}
			return
		}
		errc <- ErrConflictingHeaders{H1: h, Primary: c.primary, H2: altH, Witness: witness}
	}

	errc <- nil
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
		return nil, fmt.Errorf("can't get last trusted height: %w", err)
	}

	if lastTrustedHeight == -1 {
		// no headers yet => wait
		return nil, nil
	}

	latestHeader, latestVals, err := c.signedHeaderAndValSetFromPrimary(0)
	if err != nil {
		return nil, err
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

	c.logger.Info("Primary is unavailable. Replacing with the first witness")
	if len(c.witnesses) <= 1 {
		return errNoWitnesses{}
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
		h, providerErr := c.primary.SignedHeader(height)
		c.providerMutex.Unlock()
		if providerErr == nil {
			err := c.validateHeader(h, height)
			if err != nil {
				replaceErr := c.replacePrimaryProvider()
				if replaceErr != nil {
					return nil, fmt.Errorf("%v. Tried to replace primary but: %w", err.Error(), replaceErr)
				}
				// replace primary and request signed header again
				return c.signedHeaderFromPrimary(height)
			}
			// valid header has been received
			return h, nil
		}
		if providerErr == provider.ErrSignedHeaderNotFound {
			return nil, providerErr
		}
		c.logger.Error("Failed to get signed header from primary", "attempt", attempt, "err", providerErr)
		time.Sleep(backoffTimeout(attempt))
	}

	err := c.replacePrimaryProvider()
	if err != nil {
		return nil, fmt.Errorf("primary dropped out. Tried to replace but: %w", err)
	}

	return c.signedHeaderFromPrimary(height)
}

func (c *Client) validateHeader(h *types.SignedHeader, expectedHeight int64) error {
	if h == nil {
		return errors.New("nil header")
	}
	err := h.ValidateBasic(c.chainID)
	if err != nil {
		return err
	}
	if expectedHeight > 0 && h.Height != expectedHeight {
		return errors.New("height mismatch")
	}
	return nil
}

// validatorSetFromPrimary retrieves the ValidatorSet from the primary provider
// at the specified height. Handles dropout by the primary provider after 5
// attempts by replacing it with an alternative provider.
func (c *Client) validatorSetFromPrimary(height int64) (*types.ValidatorSet, error) {
	for attempt := uint16(1); attempt <= c.maxRetryAttempts; attempt++ {
		c.providerMutex.Lock()
		vals, providerErr := c.primary.ValidatorSet(height)
		c.providerMutex.Unlock()
		if providerErr == nil {
			err := c.validateValidatorSet(vals)
			if err != nil {
				replaceErr := c.replacePrimaryProvider()
				if replaceErr != nil {
					return nil, fmt.Errorf("%v. Tried to replace primary but: %w", err.Error(), replaceErr)
				}
				// replace primary and request signed header again
				return c.validatorSetFromPrimary(height)
			}
			return vals, nil
		}

		if providerErr == provider.ErrValidatorSetNotFound {
			return vals, providerErr
		}
		c.logger.Error("Failed to get validator set from primary", "attempt", attempt, "err", providerErr)
		time.Sleep(backoffTimeout(attempt))
	}

	err := c.replacePrimaryProvider()
	if err != nil {
		return nil, fmt.Errorf("primary dropped out. Tried to replace but: %w", err)
	}

	return c.validatorSetFromPrimary(height)
}

func (c *Client) validateValidatorSet(vals *types.ValidatorSet) error {
	if vals == nil {
		return errors.New("validator set is nil")
	}
	return vals.ValidateBasic()
}

// sendConflictingHeadersEvidence sends evidence to all witnesses and primary
// on best effort basis.
//
// Evidence needs to be submitted to all full nodes since there's no way to
// determine which full node is correct (honest).
func (c *Client) sendConflictingHeadersEvidence(ev *types.ConflictingHeadersEvidence) {
	err := c.primary.ReportEvidence(ev)
	if err != nil {
		c.logger.Error("Failed to report evidence to primary", "ev", ev, "primary", c.primary)
	}

	for _, w := range c.witnesses {
		err := w.ReportEvidence(ev)
		if err != nil {
			c.logger.Error("Failed to report evidence to witness", "ev", ev, "witness", w)
		}
	}
}

// exponential backoff (with jitter)
//		0.5s -> 2s -> 4.5s -> 8s -> 12.5 with 1s variation
func backoffTimeout(attempt uint16) time.Duration {
	return time.Duration(500*attempt*attempt)*time.Millisecond + time.Duration(rand.Intn(1000))*time.Millisecond
}

func hash2str(hash []byte) string {
	return fmt.Sprintf("%X", hash)
}
