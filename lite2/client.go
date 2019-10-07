package lite

import (
	"errors"
	"fmt"
	"log"
	"time"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/lite2/store"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	"github.com/tendermint/tendermint/types"
)

const (
	loggerPath = "lite"
	lvlDBFile  = "trusted.lvl"
	dbName     = "trust-base"
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
	// Required: only trust commits up to this old.
	// Should be equal to the unbonding period minus a configurable evidence
	// submission synchrony bound.
	TrustPeriod time.Duration

	// Option 1: TrustHeight and TrustHash can both be provided
	// to force the trusting of a particular height and hash.
	// If the latest trusted height/hash is more recent, then this option is
	// ignored.
	TrustHeight int64
	TrustHash   []byte

	// Option 2: Callback can be set to implement a confirmation
	// step if the trust store is uninitialized, or expired.
	Callback func(height int64, hash []byte) error
}

// Option1 returns true if Option1 is selected.
func (opts TrustOptions) Option1() bool {
	return opts.TrustHeight > 0 && len(opts.TrustHash) > 0
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
	chainID      string
	trustOptions TrustOptions

	mode       mode
	trustLevel float32

	lastVerifiedHeight int64

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

// NewClient returns a new Client.
//
// If no trusted store is configured using TrustedStore option, goleveldb
// database will be used.
func NewClient(chainID string, trustOptions TrustOptions, primary provider.Provider,
	options ...Option) *Client {

	c := &Client{
		chainID:      chainID,
		trustOptions: trustOptions,
		primary:      primary,
	}

	for _, o := range options {
		o(c)
	}

	// Better to execute after to avoid unnecessary initialization.
	if c.trustedStore == nil {
		rootDir := "."
		c.trustedStore = dbs.New(
			dbm.NewDB(lvlDBFile, "goleveldb", rootDir), "",
		)
	}

	return c
}

func (c *Client) Verify(
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	return c.bisection(c.trustedHeader, c.trustedVals, newHeader, newVals, now)
}

func (c *Client) bisection(lastHeader *types.SignedHeader,
	lastVals *types.ValidatorSet,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	err := Verify(c.chainID, lastHeader, lastVals, newHeader, newVals, c.trustOptions.TrustPeriod, now, c.trustLevel)
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
	pivotHeader := c.signedHeader(pivot)
	c.storeSignedHeader(pivotHeader)

	pivotVals := c.validators(pivot)
	c.storeValidators(pivotVals)

	if err := c.bisection(lastHeader, lastVals, pivotHeader, pivotVals, now); err != nil {
		return c.bisection(pivotHeader, pivotVals, newHeader, newVals, now)
	}

	return errors.New("bisection failed. restart with different full-node?")
}

func (c *Client) storeSignedHeader(h *types.SignedHeader) {
	// TODO: save to DB
}

func (c *Client) storeValidators(vals *types.ValidatorSet) {
	// TODO: save to DB
}

func (c *Client) signedHeader(height int64) *types.SignedHeader {
	// TODO: use provider
	return nil
}

func (c *Client) validators(height int64) *types.ValidatorSet {
	// TODO: use provider
	return nil
}
