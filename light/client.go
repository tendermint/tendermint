package light

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	tmstrings "github.com/tendermint/tendermint/internal/libs/strings"
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

	defaultPruningSize = 1000

	// For verifySkipping, we need an algorithm to find what height to check
	// next to see if it has sufficient validator set overlap. The most
	// intuitive method is to take the halfway point i.e. if you trusted block
	// 1 and were not able to verify block 128 then your next try would be 64.
	//
	// However, because this implementation caches all the prior results, instead of always taking halfpoints
	// it is more efficient to re-check cached blocks. Take this simple example. Say
	// you failed to verify 64 but were able to verify block 32. Following a strict half-way policy,
	// you would start over again and try verify to block 128. If this failed
	// then the halfway point between 32 and 128 is 80. But you already have
	// block 64. Instead of requesting and waiting for another block it is far
	// better to try again with block 64. This is of course not directly in the
	// middle. In fact, no matter how the algrorithm plays out, the blocks in
	// cache are always going to be a little less than the halfway point (
	// maximum 1/8 less). To account for this we add a heuristic, bumping the
	// next height to 9/16 instead of 1/2
	verifySkippingNumerator   = 9
	verifySkippingDenominator = 16

	// 10s should cover most of the clients.
	// References:
	// - http://vancouver-webpages.com/time/web.html
	// - https://blog.codinghorror.com/keeping-time-on-the-pc/
	defaultMaxClockDrift = 10 * time.Second

	// 10s is sufficient for most networks.
	defaultMaxBlockLag = 10 * time.Second
)

// Option sets a parameter for the light client.
type Option func(*Client)

// SequentialVerification option configures the light client to sequentially
// check the blocks (every block, in ascending height order). Note this is
// much slower than SkippingVerification, albeit more secure.
func SequentialVerification() Option {
	return func(c *Client) { c.verificationMode = sequential }
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
	return func(c *Client) { c.pruningSize = h }
}

// Logger option can be used to set a logger for the client.
func Logger(l log.Logger) Option {
	return func(c *Client) { c.logger = l }
}

// MaxClockDrift defines how much new header's time can drift into
// the future relative to the light clients local time. Default: 10s.
func MaxClockDrift(d time.Duration) Option {
	return func(c *Client) { c.maxClockDrift = d }
}

// MaxBlockLag represents the maximum time difference between the realtime
// that a block is received and the timestamp of that block.
// One can approximate it to the maximum block production time
//
// As an example, say the light client received block B at a time
// 12:05 (this is the real time) and the time on the block
// was 12:00. Then the lag here is 5 minutes.
// Default: 10s
func MaxBlockLag(d time.Duration) Option {
	return func(c *Client) { c.maxBlockLag = d }
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
	maxClockDrift    time.Duration
	maxBlockLag      time.Duration

	// Mutex for locking during changes of the light clients providers
	providerMutex sync.Mutex
	// Primary provider of new headers.
	primary provider.Provider
	// Providers used to "witness" new headers.
	witnesses []provider.Provider

	// Where trusted light blocks are stored.
	trustedStore store.Store
	// Highest trusted light block from the store (height=H).
	latestTrustedBlock *types.LightBlock

	// See PruningSize option
	pruningSize uint16

	logger log.Logger
}

func validatePrimaryAndWitnesses(primary provider.Provider, witnesses []provider.Provider) error {
	witnessMap := make(map[string]struct{})
	for _, w := range witnesses {
		if w.ID() == primary.ID() {
			return fmt.Errorf("primary (%s) cannot be also configured as witness", primary.ID())
		}
		if _, duplicate := witnessMap[w.ID()]; duplicate {
			return fmt.Errorf("witness list must not contain duplicates; duplicate found: %s", w.ID())
		}
		witnessMap[w.ID()] = struct{}{}
	}
	return nil
}

// NewClient returns a new light client. It returns an error if it fails to
// obtain the light block from the primary, or they are invalid (e.g. trust
// hash does not match with the one from the headers).
//
// Witnesses are providers, which will be used for cross-checking the primary
// provider. At least one witness should be given when skipping verification is
// used (default). A verified header is compared with the headers at same height
// obtained from the specified witnesses. A witness can become a primary iff the
// current primary is unavailable.
//
// See all Option(s) for the additional configuration.
func NewClient(
	ctx context.Context,
	chainID string,
	trustOptions TrustOptions,
	primary provider.Provider,
	witnesses []provider.Provider,
	trustedStore store.Store,
	options ...Option,
) (*Client, error) {

	// Check whether the trusted store already has a trusted block. If so, then create
	// a new client from the trusted store instead of the trust options.
	lastHeight, err := trustedStore.LastLightBlockHeight()
	if err != nil {
		return nil, err
	}
	if lastHeight > 0 {
		return NewClientFromTrustedStore(
			chainID, trustOptions.Period, primary, witnesses, trustedStore, options...,
		)
	}

	// Check that the witness list does not include duplicates or the primary
	if err := validatePrimaryAndWitnesses(primary, witnesses); err != nil {
		return nil, err
	}

	// Validate trust options
	if err := trustOptions.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid TrustOptions: %w", err)
	}

	c := &Client{
		chainID:          chainID,
		trustingPeriod:   trustOptions.Period,
		verificationMode: skipping,
		primary:          primary,
		witnesses:        witnesses,
		trustedStore:     trustedStore,
		trustLevel:       DefaultTrustLevel,
		maxClockDrift:    defaultMaxClockDrift,
		maxBlockLag:      defaultMaxBlockLag,
		pruningSize:      defaultPruningSize,
		logger:           log.NewNopLogger(),
	}

	for _, o := range options {
		o(c)
	}

	// Validate trust level.
	if err := ValidateTrustLevel(c.trustLevel); err != nil {
		return nil, err
	}

	// Use the trusted hash and height to fetch the first weakly-trusted block
	// from the primary provider. Assert that all the witnesses have the same block
	if err := c.initializeWithTrustOptions(ctx, trustOptions); err != nil {
		return nil, err
	}

	return c, nil
}

// NewClientFromTrustedStore initializes an existing client from the trusted store.
// It does not check that the providers have the same trusted block.
func NewClientFromTrustedStore(
	chainID string,
	trustingPeriod time.Duration,
	primary provider.Provider,
	witnesses []provider.Provider,
	trustedStore store.Store,
	options ...Option) (*Client, error) {

	// Check that the witness list does not include duplicates or the primary
	if err := validatePrimaryAndWitnesses(primary, witnesses); err != nil {
		return nil, err
	}

	c := &Client{
		chainID:          chainID,
		trustingPeriod:   trustingPeriod,
		verificationMode: skipping,
		trustLevel:       DefaultTrustLevel,
		maxClockDrift:    defaultMaxClockDrift,
		maxBlockLag:      defaultMaxBlockLag,
		primary:          primary,
		witnesses:        witnesses,
		trustedStore:     trustedStore,
		pruningSize:      defaultPruningSize,
		logger:           log.NewNopLogger(),
	}

	for _, o := range options {
		o(c)
	}

	// Validate trust level.
	if err := ValidateTrustLevel(c.trustLevel); err != nil {
		return nil, err
	}

	// Check that the trusted store has at least one block and
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
	if lastHeight <= 0 {
		return errors.New("trusted store is empty")
	}

	trustedBlock, err := c.trustedStore.LightBlock(lastHeight)
	if err != nil {
		return fmt.Errorf("can't get last trusted light block: %w", err)
	}
	c.latestTrustedBlock = trustedBlock
	c.logger.Info("restored trusted light block", "height", lastHeight)

	return nil
}

// initializeWithTrustOptions fetches the weakly-trusted light block from
// primary provider, matches it to the trusted hash, and sets it as the
// lastTrustedBlock. It then asserts that all witnesses have the same light block.
func (c *Client) initializeWithTrustOptions(ctx context.Context, options TrustOptions) error {
	// 1) Fetch and verify the light block. Note that we do not verify the time of the first block
	l, err := c.lightBlockFromPrimary(ctx, options.Height)
	if err != nil {
		return err
	}

	// 2) Assert that the hashes match
	if !bytes.Equal(l.Header.Hash(), options.Hash) {
		return fmt.Errorf("expected header's hash %X, but got %X", options.Hash, l.Hash())
	}

	// 3) Ensure that +2/3 of validators signed correctly. This also sanity checks that the
	// chain ID is the same.
	err = l.ValidatorSet.VerifyCommitLight(c.chainID, l.Commit.BlockID, l.Height, l.Commit)
	if err != nil {
		return fmt.Errorf("invalid commit: %w", err)
	}

	// 4) Cross-verify with witnesses to ensure everybody has the same state.
	if err := c.compareFirstHeaderWithWitnesses(ctx, l.SignedHeader); err != nil {
		return err
	}

	// 5) Persist both of them and continue.
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

	// If there is a new light block then verify it
	if latestBlock.Height > lastTrustedHeight {
		err = c.verifyLightBlock(ctx, latestBlock, now)
		if err != nil {
			return nil, err
		}
		c.logger.Info("advanced to new state", "height", latestBlock.Height, "hash", latestBlock.Hash())
		return latestBlock, nil
	}

	// else return the latestTrustedBlock
	return c.latestTrustedBlock, nil
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

	// Check if the light block is already verified.
	h, err := c.TrustedLightBlock(height)
	if err == nil {
		c.logger.Debug("header has already been verified", "height", height, "hash", h.Hash())
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
// https://github.com/tendermint/tendermint/blob/master/spec/light-client/README.md
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
		c.logger.Debug("header has already been verified",
			"height", newHeader.Height,
			"hash", tmstrings.LazyBlockHash(newHeader))
		return nil
	}

	// Request the header and the vals.
	l, err = c.lightBlockFromPrimary(ctx, newHeader.Height)
	if err != nil {
		return fmt.Errorf("failed to retrieve light block from primary to verify against: %w", err)
	}

	if !bytes.Equal(l.Hash(), newHeader.Hash()) {
		return fmt.Errorf("header from primary %X does not match newHeader %X", l.Hash(), newHeader.Hash())
	}

	return c.verifyLightBlock(ctx, l, now)
}

func (c *Client) verifyLightBlock(ctx context.Context, newLightBlock *types.LightBlock, now time.Time) error {
	c.logger.Info("verify light block", "height", newLightBlock.Height, "hash", newLightBlock.Hash())

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

	// Verifying between first and last trusted light block. In this situation
	// we find the closest block prior to the target height then perform
	// verification forwards.
	default:
		var closestBlock *types.LightBlock
		closestBlock, err = c.trustedStore.LightBlockBefore(newLightBlock.Height)
		if err != nil {
			return fmt.Errorf("can't get signed header before height %d: %w", newLightBlock.Height, err)
		}
		err = verifyFunc(ctx, closestBlock, newLightBlock, now)
	}
	if err != nil {
		c.logger.Error("failed to verify", "err", err)
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
		c.logger.Debug("verify adjacent newLightBlock against verifiedBlock",
			"trustedHeight", verifiedBlock.Height,
			"trustedHash", tmstrings.LazyBlockHash(verifiedBlock),
			"newHeight", interimBlock.Height,
			"newHash", interimBlock.Hash())

		err = VerifyAdjacent(verifiedBlock.SignedHeader, interimBlock.SignedHeader, interimBlock.ValidatorSet,
			c.trustingPeriod, now, c.maxClockDrift)
		if err != nil {
			err := ErrVerificationFailed{From: verifiedBlock.Height, To: interimBlock.Height, Reason: err}

			switch errors.Unwrap(err).(type) {
			case ErrInvalidHeader:
				// If the target header is invalid, return immediately.
				if err.To == newLightBlock.Height {
					c.logger.Debug("target header is invalid", "err", err)
					return err
				}

				// If some intermediate header is invalid, remove the primary and try again.
				c.logger.Info("primary sent invalid header -> removing", "err", err, "primary", c.primary)

				replacementBlock, removeErr := c.findNewPrimary(ctx, newLightBlock.Height, true)
				if removeErr != nil {
					c.logger.Debug("failed to replace primary. Returning original error", "err", removeErr)
					return err
				}

				if !bytes.Equal(replacementBlock.Hash(), newLightBlock.Hash()) {
					c.logger.Debug("replaced primary but new primary has a different block to the initial one")
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
//
// If this function errors, it should always wrap it in a `ErrVerifcationFailed`
// struct so that the calling function can determine where it failed and handle
// it accordingly.
func (c *Client) verifySkipping(
	ctx context.Context,
	source provider.Provider,
	trustedBlock *types.LightBlock,
	newLightBlock *types.LightBlock,
	now time.Time) ([]*types.LightBlock, error) {

	var (
		// The block cache is ordered in height from highest to lowest. We start
		// with the newLightBlock and for any height requested in between we add
		// it.
		blockCache = []*types.LightBlock{newLightBlock}
		depth      = 0

		verifiedBlock = trustedBlock
		trace         = []*types.LightBlock{trustedBlock}
	)

	for {
		c.logger.Debug("verify non-adjacent newHeader against verifiedBlock",
			"trustedHeight", verifiedBlock.Height,
			"trustedHash", tmstrings.LazyBlockHash(verifiedBlock),
			"newHeight", blockCache[depth].Height,
			"newHash", tmstrings.LazyBlockHash(blockCache[depth]))

		// Verify the untrusted header. This function is equivalent to
		// ValidAndVerified in the spec
		err := Verify(verifiedBlock.SignedHeader, verifiedBlock.ValidatorSet, blockCache[depth].SignedHeader,
			blockCache[depth].ValidatorSet, c.trustingPeriod, now, c.maxClockDrift, c.trustLevel)
		switch err.(type) {
		case nil:
			// If we have verified the last header then depth will be 0 and we
			// can return a success along with the trace of intermediate headers
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
			// the light block current passed validation, but the validator
			// set is too different to verify it. We keep the block because it
			// may become valuable later on.
			//
			// If we have reached the end of the cache we need to request a
			// completely new block else we recycle a previously requested one.
			// In both cases we are taking a block with a closer height to the
			// previously verified one in the hope that it has a better chance
			// of having a similar validator set
			if depth == len(blockCache)-1 {
				// schedule what the next height we need to fetch is
				pivotHeight := c.schedule(verifiedBlock.Height, blockCache[depth].Height)
				interimBlock, providerErr := c.getLightBlock(ctx, source, pivotHeight)
				if providerErr != nil {
					return nil, ErrVerificationFailed{From: verifiedBlock.Height, To: pivotHeight, Reason: providerErr}
				}
				blockCache = append(blockCache, interimBlock)
			}
			depth++

		// for any verification error we abort the operation and return the error
		default:
			return nil, ErrVerificationFailed{From: verifiedBlock.Height, To: blockCache[depth].Height, Reason: err}
		}
	}
}

// schedule works out the next height to attempt sequential verification
func (c *Client) schedule(lastVerifiedHeight, lastFailedHeight int64) int64 {
	return lastVerifiedHeight +
		(lastFailedHeight-lastVerifiedHeight)*verifySkippingNumerator/verifySkippingDenominator
}

// verifySkippingAgainstPrimary does verifySkipping plus it compares new header with
// witnesses and replaces primary if it sends the light client an invalid header
func (c *Client) verifySkippingAgainstPrimary(
	ctx context.Context,
	trustedBlock *types.LightBlock,
	newLightBlock *types.LightBlock,
	now time.Time) error {

	trace, err := c.verifySkipping(ctx, c.primary, trustedBlock, newLightBlock, now)
	if err == nil {
		// Success! Now compare the header with the witnesses to ensure it's not a fork.
		// More witnesses we have, more chance to notice one.
		//
		// CORRECTNESS ASSUMPTION: there's at least 1 correct full node
		// (primary or one of the witnesses).
		if cmpErr := c.detectDivergence(ctx, trace, now); cmpErr != nil {
			return cmpErr
		}
	}

	var e = &ErrVerificationFailed{}
	// all errors from verify skipping should be `ErrVerificationFailed`
	// if it's not we just return the error directly
	if !errors.As(err, e) {
		return err
	}

	replace := true
	switch e.Reason.(type) {
	// Verification returned an invalid header
	case ErrInvalidHeader:
		// If it was the target header, return immediately.
		if e.To == newLightBlock.Height {
			c.logger.Debug("target header is invalid", "err", err)
			return err
		}

		// If some intermediate header is invalid, remove the primary and try
		// again.

	// An intermediate header expired. We can no longer validate it as there is
	// no longer the ability to punish invalid blocks as evidence of misbehavior
	case ErrOldHeaderExpired:
		return err

	// This happens if there was a problem in finding the next block or a
	// context was canceled.
	default:
		if errors.Is(e.Reason, context.Canceled) || errors.Is(e.Reason, context.DeadlineExceeded) {
			return e.Reason
		}

		if !c.providerShouldBeRemoved(e.Reason) {
			replace = false
		}
	}

	// if we've reached here we're attempting to retry verification with a
	// different provider
	c.logger.Info("primary returned error", "err", e, "primary", c.primary, "replace", replace)

	replacementBlock, removeErr := c.findNewPrimary(ctx, newLightBlock.Height, replace)
	if removeErr != nil {
		c.logger.Error("failed to replace primary. Returning original error", "err", removeErr)
		return e.Reason
	}

	if !bytes.Equal(replacementBlock.Hash(), newLightBlock.Hash()) {
		c.logger.Debug("replaced primary but new primary has a different block to the initial one. Returning original error")
		return e.Reason
	}

	// attempt to verify the header again from the trusted block
	return c.verifySkippingAgainstPrimary(ctx, trustedBlock, replacementBlock, now)
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

// AddProvider adds a providers to the light clients set
//
// NOTE: The light client does not check for uniqueness
func (c *Client) AddProvider(p provider.Provider) {
	c.providerMutex.Lock()
	defer c.providerMutex.Unlock()
	c.witnesses = append(c.witnesses, p)
}

// Cleanup removes all the data (headers and validator sets) stored. Note: the
// client must be stopped at this point.
func (c *Client) Cleanup() error {
	c.logger.Info("removing all light blocks")
	c.latestTrustedBlock = nil
	return c.trustedStore.Prune(0)
}

func (c *Client) updateTrustedLightBlock(l *types.LightBlock) error {
	c.logger.Debug("updating trusted light block", "light_block", l)

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
		c.logger.Debug("verify newHeader against verifiedHeader",
			"trustedHeight", verifiedHeader.Height,
			"trustedHash", tmstrings.LazyBlockHash(verifiedHeader),
			"newHeight", interimHeader.Height,
			"newHash", tmstrings.LazyBlockHash(interimHeader))
		if err := VerifyBackwards(interimHeader, verifiedHeader); err != nil {
			// verification has failed
			c.logger.Info("backwards verification failed, replacing primary...", "err", err, "primary", c.primary)

			// the client tries to see if it can get a witness to continue with the request
			newPrimarysBlock, replaceErr := c.findNewPrimary(ctx, newHeader.Height, true)
			if replaceErr != nil {
				c.logger.Debug("failed to replace primary. Returning original error", "err", replaceErr)
				return err
			}

			// before continuing we must check that they have the same target header to validate
			if !bytes.Equal(newPrimarysBlock.Hash(), newHeader.Hash()) {
				c.logger.Debug("replaced primary but new primary has a different block to the initial one")
				// return the original error
				return err
			}

			// try again with the new primary
			return c.backwards(ctx, verifiedHeader, newPrimarysBlock.Header)
		}
		verifiedHeader = interimHeader
	}

	return nil
}

// lightBlockFromPrimary retrieves the lightBlock from the primary provider
// at the specified height. This method also handles provider behavior as follows:
//
// 1. If the provider does not respond or does not have the block, it tries again
//    with a different provider
// 2. If all providers return the same error, the light client forwards the error to
//    where the initial request came from
// 3. If the provider provides an invalid light block, is deemed unreliable or returns
//    any other error, the primary is permanently dropped and is replaced by a witness.
func (c *Client) lightBlockFromPrimary(ctx context.Context, height int64) (*types.LightBlock, error) {
	c.providerMutex.Lock()
	l, err := c.getLightBlock(ctx, c.primary, height)
	c.providerMutex.Unlock()

	switch err {
	case nil:
		// Everything went smoothly. We reset the lightBlockRequests and return the light block
		return l, nil

	// catch canceled contexts or deadlines
	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	case provider.ErrNoResponse, provider.ErrLightBlockNotFound, provider.ErrHeightTooHigh:
		// we find a new witness to replace the primary
		c.logger.Info("error from light block request from primary, replacing...",
			"error", err, "height", height, "primary", c.primary)
		return c.findNewPrimary(ctx, height, false)

	default:
		// The light client has most likely received either provider.ErrUnreliableProvider or provider.ErrBadLightBlock
		// These errors mean that the light client should drop the primary and try with another provider instead
		c.logger.Info("error from light block request from primary, removing...",
			"error", err, "height", height, "primary", c.primary)
		return c.findNewPrimary(ctx, height, true)
	}
}

func (c *Client) getLightBlock(ctx context.Context, p provider.Provider, height int64) (*types.LightBlock, error) {
	l, err := p.LightBlock(ctx, height)
	if ctx.Err() != nil {
		return nil, provider.ErrNoResponse
	}
	return l, err
}

// NOTE: requires a providerMutex lock
func (c *Client) removeWitnesses(indexes []int) error {
	if len(c.witnesses) <= len(indexes) {
		return ErrNoWitnesses
	}

	// we need to make sure that we remove witnesses by index in the reverse
	// order so as to not affect the indexes themselves
	sort.Ints(indexes)
	for i := len(indexes) - 1; i >= 0; i-- {
		c.witnesses[indexes[i]] = c.witnesses[len(c.witnesses)-1]
		c.witnesses = c.witnesses[:len(c.witnesses)-1]
	}

	return nil
}

type witnessResponse struct {
	lb           *types.LightBlock
	witnessIndex int
	err          error
}

// findNewPrimary concurrently sends a light block request, promoting the first witness to return
// a valid light block as the new primary. The remove option indicates whether the primary should be
// entire removed or just appended to the back of the witnesses list. This method also handles witness
// errors. If no witness is available, it returns the last error of the witness.
func (c *Client) findNewPrimary(ctx context.Context, height int64, remove bool) (*types.LightBlock, error) {
	c.providerMutex.Lock()
	defer c.providerMutex.Unlock()

	if len(c.witnesses) < 1 {
		return nil, ErrNoWitnesses
	}

	var (
		witnessResponsesC = make(chan witnessResponse, len(c.witnesses))
		witnessesToRemove []int
		lastError         error
		wg                sync.WaitGroup
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// send out a light block request to all witnesses
	for index := range c.witnesses {
		wg.Add(1)
		go func(witnessIndex int, witnessResponsesC chan witnessResponse) {
			defer wg.Done()

			lb, err := c.witnesses[witnessIndex].LightBlock(ctx, height)
			select {
			case witnessResponsesC <- witnessResponse{lb, witnessIndex, err}:
			case <-ctx.Done():
			}

		}(index, witnessResponsesC)
	}

	// process all the responses as they come in
	for i := 0; i < cap(witnessResponsesC); i++ {
		var response witnessResponse
		select {
		case response = <-witnessResponsesC:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		switch response.err {
		// success! We have found a new primary
		case nil:
			cancel() // cancel all remaining requests to other witnesses

			wg.Wait() // wait for all goroutines to finish

			// if we are not intending on removing the primary then append the old primary to the end of the witness slice
			if !remove {
				c.witnesses = append(c.witnesses, c.primary)
			}

			// promote respondent as the new primary
			c.logger.Debug("found new primary", "primary", c.witnesses[response.witnessIndex])
			c.primary = c.witnesses[response.witnessIndex]

			// add promoted witness to the list of witnesses to be removed
			witnessesToRemove = append(witnessesToRemove, response.witnessIndex)

			// remove witnesses marked as bad (the client must do this before we alter the witness slice and change the indexes
			// of witnesses). Removal is done in descending order
			if err := c.removeWitnesses(witnessesToRemove); err != nil {
				return nil, err
			}

			// return the light block that new primary responded with
			return response.lb, nil

		// process benign errors by logging them only
		case provider.ErrNoResponse, provider.ErrLightBlockNotFound, provider.ErrHeightTooHigh:
			lastError = response.err
			c.logger.Info("error on light block request from witness",
				"error", response.err, "primary", c.witnesses[response.witnessIndex])
			continue

		// process malevolent errors like ErrUnreliableProvider and ErrBadLightBlock by removing the witness
		default:
			lastError = response.err
			c.logger.Error("error on light block request from witness, removing...",
				"error", response.err, "primary", c.witnesses[response.witnessIndex])
			witnessesToRemove = append(witnessesToRemove, response.witnessIndex)
		}
	}

	return nil, lastError
}

// compareFirstHeaderWithWitnesses concurrently compares h with all witnesses. If any
// witness reports a different header than h, the function returns an error.
func (c *Client) compareFirstHeaderWithWitnesses(ctx context.Context, h *types.SignedHeader) error {
	compareCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	c.providerMutex.Lock()
	defer c.providerMutex.Unlock()

	if len(c.witnesses) < 1 {
		return nil
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
			c.logger.Error(`witness has a different header. Please check primary is
correct and remove witness. Otherwise, use a different primary`,
				"Witness", c.witnesses[e.WitnessIndex], "ExpHeader", h.Hash(), "GotHeader", e.Block.Hash())
			return err
		case errBadWitness:
			// If witness sent us an invalid header, then remove it
			c.logger.Info("witness returned an error, removing...",
				"err", err)
			witnessesToRemove = append(witnessesToRemove, e.WitnessIndex)
		default:
			// check for canceled contexts or deadlines
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}

			// the witness either didn't respond or didn't have the block. We ignore it.
			c.logger.Debug("unable to compare first header with witness, ignoring",
				"err", err)
		}

	}

	// remove all witnesses that misbehaved
	return c.removeWitnesses(witnessesToRemove)
}

// providerShouldBeRemoved analyzes the nature of the error and whether the provider
// should be removed from the light clients set
func (c *Client) providerShouldBeRemoved(err error) bool {
	return errors.As(err, &provider.ErrUnreliableProvider{}) ||
		errors.As(err, &provider.ErrBadLightBlock{}) ||
		errors.Is(err, provider.ErrConnectionClosed)
}

func (c *Client) Status(ctx context.Context) *types.LightClientInfo {
	chunks := make([]string, len(c.witnesses))

	// If primary is in witness list we do not want to count it twice in the number of peers
	primaryNotInWitnessList := 1
	for i, val := range c.witnesses {
		chunks[i] = val.ID()
		if chunks[i] == c.primary.ID() {
			primaryNotInWitnessList = 0
		}
	}

	return &types.LightClientInfo{
		PrimaryID:         c.primary.ID(),
		WitnessesID:       chunks,
		NumPeers:          len(chunks) + primaryNotInWitnessList,
		LastTrustedHeight: c.latestTrustedBlock.Height,
		LastTrustedHash:   c.latestTrustedBlock.Hash(),
		LatestBlockTime:   c.latestTrustedBlock.Time,
		TrustingPeriod:    c.trustingPeriod.String(),
		// The caller of /status can deduce this from the two variables above
		// Having a boolean flag improves readbility
		TrustedBlockExpired: HeaderExpired(c.latestTrustedBlock.SignedHeader, c.trustingPeriod, time.Now()),
	}
}
