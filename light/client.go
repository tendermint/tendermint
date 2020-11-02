package light

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmsync "github.com/tendermint/tendermint/libs/sync"
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
	// For verifySkipping, when using the cache of headers from the previous batch,
	// they will always be at a height greater than 1/2 (normal verifySkipping) so to
	// find something in between the range, 9/16 is used.
	verifySkippingNumerator   = 9
	verifySkippingDenominator = 16

	// 10s should cover most of the clients.
	// References:
	// - http://vancouver-webpages.com/time/web.html
	// - https://blog.codinghorror.com/keeping-time-on-the-pc/
	defaultMaxClockDrift = 10 * time.Second
)

// Option sets a parameter for the light client.
type Option func(*Client)

// SequentialVerification option configures the light client to sequentially
// check the blocks (every block, in ascending height order). Note this is
// much slower than SkippingVerification, albeit more secure.
func SequentialVerification() Option {
	return func(c *Client) {
		c.verificationMode = sequential
	}
}

// SkippingVerification option configures the light client to skip blocks as
// long as {trustLevel} of the old validator set signed the new header. The
// verifySkipping algorithm from the specification is used for finding the minimal
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

// PruningSize option sets the maximum amount of light blocks that the light
// client stores. When Prune() is run, all light blocks that are earlier than
// the h amount of light blocks will be removed from the store.
// Default: 1000. A pruning size of 0 will not prune the light client at all.
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

// MaxClockDrift defines how much new header's time can drift into
// the future. Default: 10s.
func MaxClockDrift(d time.Duration) Option {
	return func(c *Client) {
		c.maxClockDrift = d
	}
}

// Client represents a light client, connected to a single chain, which gets
// light blocks from a primary provider, verifies them either sequentially or by
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
	providerMutex tmsync.Mutex
	// Primary provider of new headers.
	primary provider.Provider
	// Providers used to "witness" new headers.
	witnesses []provider.Provider

	// Where trusted light blocks are stored.
	trustedStore store.Store
	// Highest trusted light block from the store (height=H).
	latestTrustedBlock *types.LightBlock

	// See RemoveNoLongerTrustedHeadersPeriod option
	pruningSize uint16
	// See ConfirmationFunction option
	confirmationFn func(action string) bool

	quit chan struct{}

	logger log.Logger
}

// NewClient returns a new light client. It returns an error if it fails to
// obtain the light block from the primary or they are invalid (e.g. trust
// hash does not match with the one from the headers).
//
// Witnesses are providers, which will be used for cross-checking the primary
// provider. At least one witness must be given when skipping verification is
// used (default). A witness can become a primary iff the current primary is
// unavailable.
//
// See all Option(s) for the additional configuration.
func NewClient(
	ctx context.Context,
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

	if c.latestTrustedBlock != nil {
		c.logger.Info("Checking trusted light block using options")
		if err := c.checkTrustedHeaderUsingOptions(ctx, trustOptions); err != nil {
			return nil, err
		}
	}

	if c.latestTrustedBlock == nil || c.latestTrustedBlock.Height < trustOptions.Height {
		c.logger.Info("Downloading trusted light block using options")
		if err := c.initializeWithTrustOptions(ctx, trustOptions); err != nil {
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
	if len(c.witnesses) < 1 {
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

	if err := c.restoreTrustedLightBlock(); err != nil {
		return nil, err
	}

	return c, nil
}

// restoreTrustedLightBlock loads the latest trusted light block from the store
func (c *Client) restoreTrustedLightBlock() error {
	lastHeight, err := c.trustedStore.LastLightBlockHeight()
	if err != nil {
		return fmt.Errorf("can't get last trusted light block height: %w", err)
	}

	if lastHeight > 0 {
		trustedBlock, err := c.trustedStore.LightBlock(lastHeight)
		if err != nil {
			return fmt.Errorf("can't get last trusted light block: %w", err)
		}
		c.latestTrustedBlock = trustedBlock
		c.logger.Info("Restored trusted light block", "height", lastHeight)
	}

	return nil
}

// if options.Height:
//
//     1) ahead of trustedLightBlock.Height => fetch light blocks (same height as
//     trustedLightBlock) from primary provider and check it's hash matches the
//     trustedLightBlock's hash (if not, remove trustedLightBlock and all the light blocks
//     before)
//
//     2) equals trustedLightBlock.Height => check options.Hash matches the
//     trustedLightBlock's hash (if not, remove trustedLightBlock and all the light blocks
//     before)
//
//     3) behind trustedLightBlock.Height => remove all the light blocks between
//     options.Height and trustedLightBlock.Height, update trustedLightBlock, then
//     check options.Hash matches the trustedLightBlock's hash (if not, remove
//     trustedLightBlock and all the light blocks before)
//
// The intuition here is the user is always right. I.e. if she decides to reset
// the light client with an older header, there must be a reason for it.
func (c *Client) checkTrustedHeaderUsingOptions(ctx context.Context, options TrustOptions) error {
	var primaryHash []byte
	switch {
	case options.Height > c.latestTrustedBlock.Height:
		h, err := c.lightBlockFromPrimary(ctx, c.latestTrustedBlock.Height)
		if err != nil {
			return err
		}
		primaryHash = h.Hash()
	case options.Height == c.latestTrustedBlock.Height:
		primaryHash = options.Hash
	case options.Height < c.latestTrustedBlock.Height:
		c.logger.Info("Client initialized with old header (trusted is more recent)",
			"old", options.Height,
			"trustedHeight", c.latestTrustedBlock.Height,
			"trustedHash", hash2str(c.latestTrustedBlock.Hash()))

		action := fmt.Sprintf(
			"Rollback to %d (%X)? Note this will remove newer light blocks up to %d (%X)",
			options.Height, options.Hash,
			c.latestTrustedBlock.Height, c.latestTrustedBlock.Hash())
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

	if !bytes.Equal(primaryHash, c.latestTrustedBlock.Hash()) {
		c.logger.Info("Prev. trusted header's hash (h1) doesn't match hash from primary provider (h2)",
			"h1", hash2str(c.latestTrustedBlock.Hash()), "h2", hash2str(primaryHash))

		action := fmt.Sprintf(
			"Prev. trusted header's hash %X doesn't match hash %X from primary provider. Remove all the stored light blocks?",
			c.latestTrustedBlock.Hash(), primaryHash)
		if c.confirmationFn(action) {
			err := c.Cleanup()
			if err != nil {
				return fmt.Errorf("failed to cleanup: %w", err)
			}
		} else {
			return errors.New("refused to remove the stored light blocks despite hashes mismatch")
		}
	}

	return nil
}

// initializeWithTrustOptions fetches the weakly-trusted light block from
// primary provider.
func (c *Client) initializeWithTrustOptions(ctx context.Context, options TrustOptions) error {
	// 1) Fetch and verify the light block.
	l, err := c.lightBlockFromPrimary(ctx, options.Height)
	if err != nil {
		return err
	}

	// NOTE: - Verify func will check if it's expired or not.
	//       - h.Time is not being checked against time.Now() because we don't
	//         want to add yet another argument to NewClient* functions.
	if err := l.ValidateBasic(c.chainID); err != nil {
		return err
	}

	if !bytes.Equal(l.Hash(), options.Hash) {
		return fmt.Errorf("expected header's hash %X, but got %X", options.Hash, l.Hash())
	}

	// 2) Ensure that +2/3 of validators signed correctly.
	err = l.ValidatorSet.VerifyCommitLight(c.chainID, l.Commit.BlockID, l.Height, l.Commit)
	if err != nil {
		return fmt.Errorf("invalid commit: %w", err)
	}

	// 3) Cross-verify with witnesses to ensure everybody has the same state.
	if err := c.compareFirstHeaderWithWitnesses(ctx, l.SignedHeader); err != nil {
		return err
	}

	// 4) Persist both of them and continue.
	return c.updateTrustedLightBlock(l)
}

// TrustedLightBlock returns a trusted light block at the given height (0 - the latest).
//
// It returns an error if:
//  - there are some issues with the trusted store, although that should not
//  happen normally;
//  - negative height is passed;
//  - header has not been verified yet and is therefore not in the store
//
// Safe for concurrent use by multiple goroutines.
func (c *Client) TrustedLightBlock(height int64) (*types.LightBlock, error) {
	height, err := c.compareWithLatestHeight(height)
	if err != nil {
		return nil, err
	}
	return c.trustedStore.LightBlock(height)
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

// Update attempts to advance the state by downloading the latest light
// block and verifying it. It returns a new light block on a successful
// update. Otherwise, it returns nil (plus an error, if any).
func (c *Client) Update(ctx context.Context, now time.Time) (*types.LightBlock, error) {
	lastTrustedHeight, err := c.LastTrustedHeight()
	if err != nil {
		return nil, fmt.Errorf("can't get last trusted height: %w", err)
	}

	if lastTrustedHeight == -1 {
		// no light blocks yet => wait
		return nil, nil
	}

	latestBlock, err := c.lightBlockFromPrimary(ctx, 0)
	if err != nil {
		return nil, err
	}

	if latestBlock.Height > lastTrustedHeight {
		err = c.verifyLightBlock(ctx, latestBlock, now)
		if err != nil {
			return nil, err
		}
		c.logger.Info("Advanced to new state", "height", latestBlock.Height, "hash", hash2str(latestBlock.Hash()))
		return latestBlock, nil
	}

	return nil, nil
}

// VerifyLightBlockAtHeight fetches the light block at the given height
// and verifies it. It returns the block immediately if it exists in
// the trustedStore (no verification is needed).
//
// height must be > 0.
//
// It returns provider.ErrlightBlockNotFound if light block is not found by
// primary.
//
// It will replace the primary provider if an error from a request to the provider occurs
func (c *Client) VerifyLightBlockAtHeight(ctx context.Context, height int64, now time.Time) (*types.LightBlock, error) {
	if height <= 0 {
		return nil, errors.New("negative or zero height")
	}

	// Check if the light block already verified.
	h, err := c.TrustedLightBlock(height)
	if err == nil {
		c.logger.Info("Header has already been verified", "height", height, "hash", hash2str(h.Hash()))
		// Return already trusted light block
		return h, nil
	}

	// Request the light block from primary
	l, err := c.lightBlockFromPrimary(ctx, height)
	if err != nil {
		return nil, err
	}

	return l, c.verifyLightBlock(ctx, l, now)
}

// VerifyHeader verifies a new header against the trusted state. It returns
// immediately if newHeader exists in trustedStore (no verification is
// needed). Else it performs one of the two types of verification:
//
// SequentialVerification: verifies that 2/3 of the trusted validator set has
// signed the new header. If the headers are not adjacent, **all** intermediate
// headers will be requested. Intermediate headers are not saved to database.
//
// SkippingVerification(trustLevel): verifies that {trustLevel} of the trusted
// validator set has signed the new header. If it's not the case and the
// headers are not adjacent, verifySkipping is performed and necessary (not all)
// intermediate headers will be requested. See the specification for details.
// Intermediate headers are not saved to database.
// https://github.com/tendermint/spec/blob/master/spec/consensus/light-client.md
//
// If the header, which is older than the currently trusted header, is
// requested and the light client does not have it, VerifyHeader will perform:
//		a) verifySkipping verification if nearest trusted header is found & not expired
//		b) backwards verification in all other cases
//
// It returns ErrOldHeaderExpired if the latest trusted header expired.
//
// If the primary provides an invalid header (ErrInvalidHeader), it is rejected
// and replaced by another provider until all are exhausted.
//
// If, at any moment, a LightBlock is not found by the primary provider as part of
// verification then the provider will be replaced by another and the process will
// restart.
func (c *Client) VerifyHeader(ctx context.Context, newHeader *types.Header, now time.Time) error {
	if newHeader == nil {
		return errors.New("nil header")
	}
	if newHeader.Height <= 0 {
		return errors.New("negative or zero height")
	}

	// Check if newHeader already verified.
	l, err := c.TrustedLightBlock(newHeader.Height)
	if err == nil {
		// Make sure it's the same header.
		if !bytes.Equal(l.Hash(), newHeader.Hash()) {
			return fmt.Errorf("existing trusted header %X does not match newHeader %X", l.Hash(), newHeader.Hash())
		}
		c.logger.Info("Header has already been verified",
			"height", newHeader.Height, "hash", hash2str(newHeader.Hash()))
		return nil
	}

	// Request the header and the vals.
	l, err = c.lightBlockFromPrimary(ctx, newHeader.Height)
	if err != nil {
		return fmt.Errorf("failed to retrieve light block from primary to verify against: %w", err)
	}

	if !bytes.Equal(l.Hash(), newHeader.Hash()) {
		return fmt.Errorf("light block header %X does not match newHeader %X", l.Hash(), newHeader.Hash())
	}

	return c.verifyLightBlock(ctx, l, now)
}

func (c *Client) verifyLightBlock(ctx context.Context, newLightBlock *types.LightBlock, now time.Time) error {
	c.logger.Info("VerifyHeader", "height", newLightBlock.Height, "hash", hash2str(newLightBlock.Hash()))

	var (
		verifyFunc func(ctx context.Context, trusted *types.LightBlock, new *types.LightBlock, now time.Time) error
		err        error
	)

	switch c.verificationMode {
	case sequential:
		verifyFunc = c.verifySequential
	case skipping:
		verifyFunc = c.verifySkippingAgainstPrimary
	default:
		panic(fmt.Sprintf("Unknown verification mode: %b", c.verificationMode))
	}

	firstBlockHeight, err := c.FirstTrustedHeight()
	if err != nil {
		return fmt.Errorf("can't get first light block height: %w", err)
	}

	switch {
	// Verifying forwards
	case newLightBlock.Height >= c.latestTrustedBlock.Height:
		err = verifyFunc(ctx, c.latestTrustedBlock, newLightBlock, now)

	// Verifying backwards
	case newLightBlock.Height < firstBlockHeight:
		var firstBlock *types.LightBlock
		firstBlock, err = c.trustedStore.LightBlock(firstBlockHeight)
		if err != nil {
			return fmt.Errorf("can't get first light block: %w", err)
		}
		err = c.backwards(ctx, firstBlock.Header, newLightBlock.Header)

	// Verifying between first and last trusted light block
	default:
		var closestBlock *types.LightBlock
		closestBlock, err = c.trustedStore.LightBlockBefore(newLightBlock.Height)
		if err != nil {
			return fmt.Errorf("can't get signed header before height %d: %w", newLightBlock.Height, err)
		}
		err = verifyFunc(ctx, closestBlock, newLightBlock, now)
	}
	if err != nil {
		c.logger.Error("Can't verify", "err", err)
		return err
	}

	// Once verified, save and return
	return c.updateTrustedLightBlock(newLightBlock)
}

// see VerifyHeader
func (c *Client) verifySequential(
	ctx context.Context,
	trustedBlock *types.LightBlock,
	newLightBlock *types.LightBlock,
	now time.Time) error {

	var (
		verifiedBlock = trustedBlock
		interimBlock  *types.LightBlock
		err           error
		trace         = []*types.LightBlock{trustedBlock}
	)

	for height := trustedBlock.Height + 1; height <= newLightBlock.Height; height++ {
		// 1) Fetch interim light block if needed.
		if height == newLightBlock.Height { // last light block
			interimBlock = newLightBlock
		} else { // intermediate light blocks
			interimBlock, err = c.lightBlockFromPrimary(ctx, height)
			if err != nil {
				return ErrVerificationFailed{From: verifiedBlock.Height, To: height, Reason: err}
			}
		}

		// 2) Verify them
		c.logger.Debug("Verify adjacent newLightBlock against verifiedBlock",
			"trustedHeight", verifiedBlock.Height,
			"trustedHash", hash2str(verifiedBlock.Hash()),
			"newHeight", interimBlock.Height,
			"newHash", hash2str(interimBlock.Hash()))

		err = VerifyAdjacent(verifiedBlock.SignedHeader, interimBlock.SignedHeader, interimBlock.ValidatorSet,
			c.trustingPeriod, now, c.maxClockDrift)
		if err != nil {
			err := ErrVerificationFailed{From: verifiedBlock.Height, To: interimBlock.Height, Reason: err}

			switch errors.Unwrap(err).(type) {
			case ErrInvalidHeader:
				// If the target header is invalid, return immediately.
				if err.To == newLightBlock.Height {
					c.logger.Debug("Target header is invalid", "err", err)
					return err
				}

				// If some intermediate header is invalid, replace the primary and try
				// again.
				c.logger.Error("primary sent invalid header -> replacing", "err", err)
				replaceErr := c.replacePrimaryProvider()
				if replaceErr != nil {
					c.logger.Error("Can't replace primary", "err", replaceErr)
					// return original error
					return err
				}

				replacementBlock, fErr := c.lightBlockFromPrimary(ctx, newLightBlock.Height)
				if fErr != nil {
					c.logger.Error("Can't fetch light block from primary", "err", fErr)
					// return original error
					return err
				}

				if !bytes.Equal(replacementBlock.Hash(), newLightBlock.Hash()) {
					c.logger.Error("Replacement provider has a different light block",
						"newHash", newLightBlock.Hash(),
						"replHash", replacementBlock.Hash())
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

		// 3) Update verifiedBlock
		verifiedBlock = interimBlock

		// 4) Add verifiedBlock to trace
		trace = append(trace, verifiedBlock)
	}

	// Compare header with the witnesses to ensure it's not a fork.
	// More witnesses we have, more chance to notice one.
	//
	// CORRECTNESS ASSUMPTION: there's at least 1 correct full node
	// (primary or one of the witnesses).
	return c.detectDivergence(ctx, trace, now)
}

// see VerifyHeader
//
// verifySkipping finds the middle light block between a trusted and new light block,
// reiterating the action until it verifies a light block. A cache of light blocks
// requested from source is kept such that when a verification is made, and the
// light client tries again to verify the new light block in the middle, the light
// client does not need to ask for all the same light blocks again.
func (c *Client) verifySkipping(
	ctx context.Context,
	source provider.Provider,
	trustedBlock *types.LightBlock,
	newLightBlock *types.LightBlock,
	now time.Time) ([]*types.LightBlock, error) {

	var (
		blockCache = []*types.LightBlock{newLightBlock}
		depth      = 0

		verifiedBlock = trustedBlock
		trace         = []*types.LightBlock{trustedBlock}
	)

	for {
		c.logger.Debug("Verify non-adjacent newHeader against verifiedBlock",
			"trustedHeight", verifiedBlock.Height,
			"trustedHash", hash2str(verifiedBlock.Hash()),
			"newHeight", blockCache[depth].Height,
			"newHash", hash2str(blockCache[depth].Hash()))

		err := Verify(verifiedBlock.SignedHeader, verifiedBlock.ValidatorSet, blockCache[depth].SignedHeader,
			blockCache[depth].ValidatorSet, c.trustingPeriod, now, c.maxClockDrift, c.trustLevel)
		switch err.(type) {
		case nil:
			// Have we verified the last header
			if depth == 0 {
				trace = append(trace, newLightBlock)
				return trace, nil
			}
			// If not, update the lower bound to the previous upper bound
			verifiedBlock = blockCache[depth]
			// Remove the light block at the lower bound in the header cache - it will no longer be needed
			blockCache = blockCache[:depth]
			// Reset the cache depth so that we start from the upper bound again
			depth = 0
			// add verifiedBlock to the trace
			trace = append(trace, verifiedBlock)

		case ErrNewValSetCantBeTrusted:
			// do add another header to the end of the cache
			if depth == len(blockCache)-1 {
				pivotHeight := verifiedBlock.Height + (blockCache[depth].Height-verifiedBlock.
					Height)*verifySkippingNumerator/verifySkippingDenominator
				interimBlock, providerErr := source.LightBlock(ctx, pivotHeight)
				if providerErr != nil {
					return nil, ErrVerificationFailed{From: verifiedBlock.Height, To: pivotHeight, Reason: providerErr}
				}
				blockCache = append(blockCache, interimBlock)
			}
			depth++

		default:
			return nil, ErrVerificationFailed{From: verifiedBlock.Height, To: blockCache[depth].Height, Reason: err}
		}
	}
}

// verifySkippingAgainstPrimary does verifySkipping plus it compares new header with
// witnesses and replaces primary if it sends the light client an invalid header
func (c *Client) verifySkippingAgainstPrimary(
	ctx context.Context,
	trustedBlock *types.LightBlock,
	newLightBlock *types.LightBlock,
	now time.Time) error {

	trace, err := c.verifySkipping(ctx, c.primary, trustedBlock, newLightBlock, now)

	switch errors.Unwrap(err).(type) {
	case ErrInvalidHeader:
		// If the target header is invalid, return immediately.
		invalidHeaderHeight := err.(ErrVerificationFailed).To
		if invalidHeaderHeight == newLightBlock.Height {
			c.logger.Debug("Target header is invalid", "err", err)
			return err
		}

		// If some intermediate header is invalid, replace the primary and try
		// again.
		c.logger.Error("primary sent invalid header -> replacing", "err", err)
		replaceErr := c.replacePrimaryProvider()
		if replaceErr != nil {
			c.logger.Error("Can't replace primary", "err", replaceErr)
			// return original error
			return err
		}

		replacementBlock, fErr := c.lightBlockFromPrimary(ctx, newLightBlock.Height)
		if fErr != nil {
			c.logger.Error("Can't fetch light block from primary", "err", fErr)
			// return original error
			return err
		}

		if !bytes.Equal(replacementBlock.Hash(), newLightBlock.Hash()) {
			c.logger.Error("Replacement provider has a different light block",
				"newHash", newLightBlock.Hash(),
				"replHash", replacementBlock.Hash())
			// return original error
			return err
		}

		// attempt to verify the header again
		return c.verifySkippingAgainstPrimary(ctx, trustedBlock, replacementBlock, now)
	case nil:
		// Compare header with the witnesses to ensure it's not a fork.
		// More witnesses we have, more chance to notice one.
		//
		// CORRECTNESS ASSUMPTION: there's at least 1 correct full node
		// (primary or one of the witnesses).
		if cmpErr := c.detectDivergence(ctx, trace, now); cmpErr != nil {
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
	return c.trustedStore.LastLightBlockHeight()
}

// FirstTrustedHeight returns a first trusted height. -1 and nil are returned if
// there are no trusted headers.
//
// Safe for concurrent use by multiple goroutines.
func (c *Client) FirstTrustedHeight() (int64, error) {
	return c.trustedStore.FirstLightBlockHeight()
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
	c.latestTrustedBlock = nil
	return c.trustedStore.Prune(0)
}

// cleanupAfter deletes all headers & validator sets after +height+. It also
// resets latestTrustedBlock to the latest header.
func (c *Client) cleanupAfter(height int64) error {
	prevHeight := c.latestTrustedBlock.Height

	for {
		h, err := c.trustedStore.LightBlockBefore(prevHeight)
		if err == store.ErrLightBlockNotFound || (h != nil && h.Height <= height) {
			break
		} else if err != nil {
			return fmt.Errorf("failed to get header before %d: %w", prevHeight, err)
		}

		err = c.trustedStore.DeleteLightBlock(h.Height)
		if err != nil {
			c.logger.Error("can't remove a trusted header & validator set", "err", err,
				"height", h.Height)
		}

		prevHeight = h.Height
	}

	c.latestTrustedBlock = nil
	err := c.restoreTrustedLightBlock()
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) updateTrustedLightBlock(l *types.LightBlock) error {
	if err := c.trustedStore.SaveLightBlock(l); err != nil {
		return fmt.Errorf("failed to save trusted header: %w", err)
	}

	if c.pruningSize > 0 {
		if err := c.trustedStore.Prune(c.pruningSize); err != nil {
			return fmt.Errorf("prune: %w", err)
		}
	}

	if c.latestTrustedBlock == nil || l.Height > c.latestTrustedBlock.Height {
		c.latestTrustedBlock = l
	}

	return nil
}

// backwards verification (see VerifyHeaderBackwards func in the spec) verifies
// headers before a trusted header. If a sent header is invalid the primary is
// replaced with another provider and the operation is repeated.
func (c *Client) backwards(
	ctx context.Context,
	trustedHeader *types.Header,
	newHeader *types.Header) error {

	var (
		verifiedHeader = trustedHeader
		interimHeader  *types.Header
	)

	for verifiedHeader.Height > newHeader.Height {
		interimBlock, err := c.lightBlockFromPrimary(ctx, verifiedHeader.Height-1)
		if err != nil {
			return fmt.Errorf("failed to obtain the header at height #%d: %w", verifiedHeader.Height-1, err)
		}
		interimHeader = interimBlock.Header
		c.logger.Debug("Verify newHeader against verifiedHeader",
			"trustedHeight", verifiedHeader.Height,
			"trustedHash", hash2str(verifiedHeader.Hash()),
			"newHeight", interimHeader.Height,
			"newHash", hash2str(interimHeader.Hash()))
		if err := VerifyBackwards(interimHeader, verifiedHeader); err != nil {
			c.logger.Error("primary sent invalid header -> replacing", "err", err)
			if replaceErr := c.replacePrimaryProvider(); replaceErr != nil {
				c.logger.Error("Can't replace primary", "err", replaceErr)
				// return original error
				return fmt.Errorf("verify backwards from %d to %d failed: %w",
					verifiedHeader.Height, interimHeader.Height, err)
			}
			// we need to verify the header at the same height again
			continue
		}
		verifiedHeader = interimHeader
	}

	return nil
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

// replaceProvider takes the first alternative provider and promotes it as the
// primary provider.
func (c *Client) replacePrimaryProvider() error {
	c.providerMutex.Lock()
	defer c.providerMutex.Unlock()

	if len(c.witnesses) <= 1 {
		return errNoWitnesses{}
	}
	c.primary = c.witnesses[0]
	c.witnesses = c.witnesses[1:]
	c.logger.Info("Replacing primary with the first witness", "new_primary", c.primary)

	return nil
}

// lightBlockFromPrimary retrieves the lightBlock from the primary provider
// at the specified height. Handles dropout by the primary provider by swapping
// with an alternative provider.
func (c *Client) lightBlockFromPrimary(ctx context.Context, height int64) (*types.LightBlock, error) {
	c.providerMutex.Lock()
	l, err := c.primary.LightBlock(ctx, height)
	c.providerMutex.Unlock()
	if err != nil {
		c.logger.Debug("Error on light block request from primary", "error", err)
		replaceErr := c.replacePrimaryProvider()
		if replaceErr != nil {
			return nil, fmt.Errorf("%v. Tried to replace primary but: %w", err.Error(), replaceErr)
		}
		// replace primary and request a light block again
		return c.lightBlockFromPrimary(ctx, height)
	}
	return l, err
}

// compareFirstHeaderWithWitnesses compares h with all witnesses. If any
// witness reports a different header than h, the function returns an error.
func (c *Client) compareFirstHeaderWithWitnesses(ctx context.Context, h *types.SignedHeader) error {
	compareCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if len(c.witnesses) < 1 {
		return errNoWitnesses{}
	}

	errc := make(chan error, len(c.witnesses))
	for i, witness := range c.witnesses {
		go c.compareNewHeaderWithWitness(compareCtx, errc, h, witness, i)
	}

	witnessesToRemove := make([]int, 0, len(c.witnesses))

	// handle errors from the header comparisons as they come in
	for i := 0; i < cap(errc); i++ {
		err := <-errc

		switch e := err.(type) {
		case nil:
			continue
		case errConflictingHeaders:
			c.logger.Error(fmt.Sprintf(`Witness #%d has a different header. Please check primary is correct
and remove witness. Otherwise, use the different primary`, e.WitnessIndex), "witness", c.witnesses[e.WitnessIndex])
			return err
		case errBadWitness:
			// If witness sent us an invalid header, then remove it. If it didn't
			// respond or couldn't find the block, then we ignore it and move on to
			// the next witness.
			if _, ok := e.Reason.(provider.ErrBadLightBlock); ok {
				c.logger.Info("Witness sent us invalid header / vals -> removing it", "witness", c.witnesses[e.WitnessIndex])
				witnessesToRemove = append(witnessesToRemove, e.WitnessIndex)
			}
		}
	}

	for _, idx := range witnessesToRemove {
		c.removeWitness(idx)
	}

	return nil
}

func hash2str(hash []byte) string {
	return fmt.Sprintf("%X", hash)
}
