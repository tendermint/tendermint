package light

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/tendermint/tendermint/crypto"
	dashcore "github.com/tendermint/tendermint/dash/core"
	tmstrings "github.com/tendermint/tendermint/internal/libs/strings"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light/provider"
	"github.com/tendermint/tendermint/light/store"
	"github.com/tendermint/tendermint/types"
)

type mode byte

const (
	dashCoreVerification mode = iota + 1

	defaultPruningSize      = 1000
	defaultMaxRetryAttempts = 10
	// For verifySkipping, when using the cache of headers from the previous batch,
	// they will always be at a height greater than 1/2 (normal verifySkipping) so to
	// find something in between the range, 9/16 is used.
	// verifySkippingNumerator   = 9
	// verifySkippingDenominator = 16

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

// DashCoreVerification option configures the light client to go to the last block and verify it was signed by the
// Quorum that is has in quorum hash.
func DashCoreVerification() Option {
	return func(c *Client) {
		c.verificationMode = dashCoreVerification
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
	verificationMode mode
	maxRetryAttempts uint16 // see MaxRetryAttempts option
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

	// Rpc client connected to dashd
	dashCoreRPCClient dashcore.QuorumVerifier

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
	primary provider.Provider,
	witnesses []provider.Provider,
	trustedStore store.Store,
	dashCoreRPCClient dashcore.QuorumVerifier,
	options ...Option) (*Client, error) {

	return NewClientAtHeight(ctx, 0, chainID, primary, witnesses, trustedStore, dashCoreRPCClient, options...)
}

func NewClientAtHeight(
	ctx context.Context,
	height int64,
	chainID string,
	primary provider.Provider,
	witnesses []provider.Provider,
	trustedStore store.Store,
	dashCoreRPCClient dashcore.QuorumVerifier,
	options ...Option,
) (*Client, error) {
	// Check that the witness list does not include duplicates or the primary
	if err := validatePrimaryAndWitnesses(primary, witnesses); err != nil {
		return nil, err
	}

	c, err := NewClientFromTrustedStore(chainID, primary, witnesses, trustedStore, dashCoreRPCClient, options...)
	if err != nil {
		return nil, err
	}

	if c.latestTrustedBlock == nil && height > 0 {
		c.logger.Info("Downloading trusted light block")
		if err := c.initializeAtHeight(ctx, height); err != nil {
			return nil, err
		}
	}

	return c, err
}

// NewClientFromTrustedStore initializes an existing client from the trusted store.
// It does not check that the providers have the same trusted block.
func NewClientFromTrustedStore(
	chainID string,
	primary provider.Provider,
	witnesses []provider.Provider,
	trustedStore store.Store,
	dashCoreRPCClient dashcore.QuorumVerifier,
	options ...Option) (*Client, error) {

	if dashCoreRPCClient == nil {
		return nil, ErrNoDashCoreClient
	}

	// Check that the witness list does not include duplicates or the primary
	if err := validatePrimaryAndWitnesses(primary, witnesses); err != nil {
		return nil, err
	}

	c := &Client{
		chainID:           chainID,
		verificationMode:  dashCoreVerification,
		maxRetryAttempts:  defaultMaxRetryAttempts,
		maxClockDrift:     defaultMaxClockDrift,
		maxBlockLag:       defaultMaxBlockLag,
		primary:           primary,
		witnesses:         witnesses,
		trustedStore:      trustedStore,
		pruningSize:       defaultPruningSize,
		logger:            log.NewNopLogger(),
		dashCoreRPCClient: dashCoreRPCClient,
	}

	for _, o := range options {
		o(c)
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

// initialize fetches the last light block from
// primary provider.
/*
func (c *Client) initialize(ctx context.Context) error {
	return c.initializeAtHeight(ctx, 0) //
}
*/

// initializeAtHeight fetches a light block at given height from
// primary provider.
func (c *Client) initializeAtHeight(ctx context.Context, height int64) error {
	// 1) Fetch and verify the light block.
	l, err := c.lightBlockFromPrimaryAtHeight(ctx, height)
	if err != nil {
		return err
	}

	// NOTE: - Verify func will check if it's expired or not.
	//       - h.Time is not being checked against time.Now() because we don't
	//         want to add yet another argument to NewClient* functions.
	if err := l.ValidateBasic(c.chainID); err != nil {
		return err
	}

	// 2) Ensure the commit height is correct
	if l.Height != l.Commit.Height {
		return fmt.Errorf("invalid commit: height %d does not match commit height %d", l.Height, l.Commit.Height)
	}

	// 3) Ensure that the commit is valid based on validator set we got back.
	// Todo: we will want to remove validator sets entirely from light blocks and just have quorum hashes
	err = l.ValidatorSet.VerifyCommit(c.chainID, l.Commit.BlockID, l.Commit.StateID, l.Height, l.Commit)
	if err != nil {
		return fmt.Errorf("invalid commit: %w", err)
	}

	// 4) Ensure that the commit is valid based on local dash core verification.
	err = c.verifyBlockWithDashCore(ctx, l)
	if err != nil {
		return fmt.Errorf("invalid light block: %w", err)
	}

	// 5) Cross-verify with witnesses to ensure everybody has the same state.
	if err := c.compareFirstHeaderWithWitnesses(ctx, l.SignedHeader); err != nil {
		return err
	}

	// 6) Persist both of them and continue.
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

	latestBlock, err := c.lightBlockFromPrimary(ctx)
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
	l, err := c.lightBlockFromPrimaryAtHeight(ctx, height)
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
	l, err = c.lightBlockFromPrimary(ctx)
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

	if err := newLightBlock.ValidateBasic(c.ChainID()); err != nil {
		return err
	}

	var (
		verifyFunc func(ctx context.Context, new *types.LightBlock) error
		err        error
	)

	switch c.verificationMode {
	case dashCoreVerification:
		verifyFunc = c.verifyBlockWithDashCore
	default:
		panic(fmt.Sprintf("Unknown verification mode: %b", c.verificationMode))
	}

	err = verifyFunc(ctx, newLightBlock)

	if err != nil {
		c.logger.Error("Can't verify", "err", err)
		return err
	}

	err = c.compareFirstHeaderWithWitnesses(ctx, newLightBlock.SignedHeader)

	if err != nil {
		c.logger.Error("failed to verify", "err", err)
		return err
	}

	// Once verified, save and return
	return c.updateTrustedLightBlock(newLightBlock)
}

// verifyBlockWithDashCore verifies block and StateID signature using Dash Core RPC service
// This method is called from verifyLightBlock if verification mode is dashcore,
// verifyLightBlock in its turn is called by VerifyHeader.
func (c *Client) verifyBlockWithDashCore(ctx context.Context, newLightBlock *types.LightBlock) error {
	if err := c.verifyBlockSignatureWithDashCore(ctx, newLightBlock); err != nil {
		return err
	}
	return c.verifyStateIDSignatureWithDashCore(ctx, newLightBlock)
}

func (c *Client) verifyBlockSignatureWithDashCore(ctx context.Context, newLightBlock *types.LightBlock) error {

	quorumHash := newLightBlock.ValidatorSet.QuorumHash
	quorumType := newLightBlock.ValidatorSet.QuorumType

	protoVote := newLightBlock.Commit.GetCanonicalVote().ToProto()

	blockSignBytes := types.VoteBlockSignBytes(c.chainID, protoVote)

	blockMessageHash := crypto.Checksum(blockSignBytes)
	blockRequestID := types.VoteBlockRequestIDProto(protoVote)

	blockSignatureIsValid, err := c.dashCoreRPCClient.QuorumVerify(
		quorumType,
		blockRequestID,
		blockMessageHash,
		newLightBlock.Commit.ThresholdBlockSignature,
		quorumHash,
	)

	if err != nil {
		return err
	}

	if !blockSignatureIsValid {
		return fmt.Errorf("block signature is invalid")
	}

	return nil
}

// This method is called from verifyLightBlock if verification mode is dashcore,
// verifyLightBlock in its turn is called by VerifyHeader.
func (c *Client) verifyStateIDSignatureWithDashCore(ctx context.Context, newLightBlock *types.LightBlock) error {
	quorumHash := newLightBlock.ValidatorSet.QuorumHash
	quorumType := newLightBlock.ValidatorSet.QuorumType

	stateID := newLightBlock.StateID()

	stateSignBytes := stateID.SignBytes(c.chainID)

	stateMessageHash := crypto.Checksum(stateSignBytes)
	stateRequestID := stateID.SignRequestID()
	stateSignature := newLightBlock.Commit.ThresholdStateSignature

	stateSignatureIsValid, err := c.dashCoreRPCClient.QuorumVerify(
		quorumType,
		stateRequestID,
		stateMessageHash,
		stateSignature,
		quorumHash,
	)

	if err != nil {
		return err
	}

	if !stateSignatureIsValid {
		return fmt.Errorf("state signature is invalid")
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

// lightBlockFromPrimary retrieves the latest lightBlock from the primary provider.
// This method also handles provider behavior as follows:
//
// 1. If the provider does not respond, it tries again
//    with a different provider
// 2. If all providers return the same error, the light client forwards the error to
//    where the initial request came from
// 3. If the provider provides an invalid light block, is deemed unreliable or returns
//    any other error, the primary is permanently dropped and is replaced by a witness.
func (c *Client) lightBlockFromPrimary(ctx context.Context) (*types.LightBlock, error) {
	return c.lightBlockFromPrimaryAtHeight(ctx, 0)
}

// lightBlockFromPrimaryAtHeight retrieves a light block from the primary provider
func (c *Client) lightBlockFromPrimaryAtHeight(ctx context.Context, height int64) (*types.LightBlock, error) {
	c.providerMutex.Lock()
	l, err := c.getLightBlock(ctx, c.primary, height)
	c.providerMutex.Unlock()

	switch err {
	case nil:

	// catch canceled contexts or deadlines
	case context.Canceled, context.DeadlineExceeded:
		return l, err

	case provider.ErrLightBlockTooOld:
		// If the block is too old check to see if it the same as our current block
		// If that's the case the chain has most likely stalled and it is most likely the current block
		// If the block is higher than our current block height then return it.
		// Otherwise we need to find a new primary
		if c.latestTrustedBlock != nil && l.Height < c.latestTrustedBlock.Height {
			l, err = c.findNewPrimary(ctx, height, false)
		} else {
			err = nil
		}

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

	// err was re-evaluated inside switch statement above
	if err != nil {
		return l, err
	}

	if height != 0 && l.Height != height {
		return l, fmt.Errorf("invalid height received from primary: expected %d, got %d", height, l.Height)
	}

	return l, nil
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

// findNewPrimary concurrently sends a light block request at some height, promoting the first witness to return
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
			resp := witnessResponse{witnessIndex: witnessIndex}
			resp.lb, resp.err = c.witnesses[witnessIndex].LightBlock(ctx, height)
			witnessResponsesC <- resp
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
		case provider.ErrNoResponse, provider.ErrLightBlockNotFound, provider.ErrHeightTooHigh, provider.ErrLightBlockTooOld:
			lastError = response.err
			c.logger.Debug("error on light block request from witness",
				"error", response.err, "primary", c.witnesses[response.witnessIndex])
			continue

		// catch canceled contexts or deadlines
		case context.Canceled, context.DeadlineExceeded:
			return nil, response.err

		// process malevolent errors like ErrUnreliableProvider and ErrBadLightBlock by removing the witness
		default:
			lastError = response.err
			c.logger.Error("error on light block request from witness, removing...",
				"error", response.err, "primary", c.witnesses[response.witnessIndex])
			witnessesToRemove = append(witnessesToRemove, response.witnessIndex)
		}
	}

	// remove witnesses marked as bad. Removal is done in descending order
	if err := c.removeWitnesses(witnessesToRemove); err != nil {
		c.logger.Error("failed to remove witnesses", "err", err, "witnessesToRemove", witnessesToRemove)
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

	if len(c.witnesses) == 0 {
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
			c.logger.Error(fmt.Sprintf("witness #%d has a different header. Please check primary is correct and"+
				" remove witness. Otherwise, use the different primary", e.WitnessIndex), "witness",
				c.witnesses[e.WitnessIndex])
			// if attempt to generate conflicting headers failed then remove witness
			witnessesToRemove = append(witnessesToRemove, e.WitnessIndex)
		case errBadWitness:
			// If witness sent us an invalid header, then remove it
			c.logger.Info("witness sent an invalid light block, removing...",
				"witness", c.witnesses[e.WitnessIndex],
				"err", err)
			witnessesToRemove = append(witnessesToRemove, e.WitnessIndex)
		default: // benign errors can be ignored with the exception of context errors
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

	info := &types.LightClientInfo{
		PrimaryID:   c.primary.ID(),
		WitnessesID: chunks,
		NumPeers:    len(chunks) + primaryNotInWitnessList,
	}

	if c.latestTrustedBlock != nil {
		info.LastTrustedHeight = c.latestTrustedBlock.Height
		info.LastTrustedHash = c.latestTrustedBlock.Hash()
		info.LatestBlockTime = c.latestTrustedBlock.Time
	}

	return info
}
