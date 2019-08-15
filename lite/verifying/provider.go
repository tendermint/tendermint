package verifying

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	log "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/lite"
	lclient "github.com/tendermint/tendermint/lite/client"
	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

const (
	loggerPath = "lite"
	memDBFile  = "trusted.mem"
	lvlDBFile  = "trusted.lvl"
	dbName     = "trust-base"

	sizeOfPendingMap = 1024
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
	TrustPeriod time.Duration `json:"trust-period"`

	// Option 1: TrustHeight and TrustHash can both be provided
	// to force the trusting of a particular height and hash.
	// If the latest trusted height/hash is more recent, then this option is
	// ignored.
	TrustHeight int64  `json:"trust-height"`
	TrustHash   []byte `json:"trust-hash"`

	// Option 2: Callback can be set to implement a confirmation
	// step if the trust store is uninitialized, or expired.
	Callback func(height int64, hash []byte) error
}

// HeightAndHashPresent returns true if TrustHeight and TrustHash are present.
func (opts TrustOptions) HeightAndHashPresent() bool {
	return opts.TrustHeight > 0 && len(opts.TrustHash) > 0
}

// Provider implements a persistent caching Provider that auto-validates. It
// uses a "source" Provider to obtain the needed FullCommits to securely sync
// with validator set changes. It stores properly validated data on the
// "trusted" local system.
//
// NOTE: This Provider can only work with one chainID, provided upon
// instantiation.
type Provider struct {
	chainID     string
	logger      log.Logger
	trustPeriod time.Duration // e.g. the unbonding period, or something smaller.
	now         nowFn
	height      int64

	// Already validated, stored locally
	trusted lite.PersistentProvider

	// New info, like a node rpc, or other import method.
	source lite.Provider

	// pending map to synchronize concurrent verification requests
	mtx                  sync.Mutex
	pendingVerifications map[int64]chan struct{}
}

var _ lite.UpdatingProvider = (*Provider)(nil)

type nowFn func() time.Time

// NewProvider creates a
// NOTE: If you retain the resulting verifier in memory for a long time, usage
// of the verifier may eventually error, but immediate usage should not error
// like that, so that e.g. cli usage never errors unexpectedly.
func NewProvider(chainID, rootDir string, client lclient.SignStatusClient,
	logger log.Logger, cacheSize int, options TrustOptions) (*Provider, error) {

	vp := initProvider(chainID, rootDir, client, logger, cacheSize, options)

	// Get the latest source commit, or the one provided in options.
	trustCommit, err := getTrustedCommit(vp.logger, client, options)
	if err != nil {
		return nil, err
	}

	err = vp.fillValsetAndSaveFC(trustCommit, nil, nil)
	if err != nil {
		return nil, err
	}

	// sanity check
	// FIXME: Can't it happen that the local clock is a bit off and the
	// trustCommit.Time is a few seconds in the future?
	now := time.Now()
	if now.Sub(trustCommit.Time) <= 0 {
		panic(fmt.Sprintf("impossible time %v vs %v", now, trustCommit.Time))
	}

	// Otherwise we're syncing within the unbonding period.
	// NOTE: There is a duplication of fetching this latest commit (since
	// UpdateToHeight() will fetch it again, and latestCommit isn't used), but
	// it's only once upon initialization so it's not a big deal.
	if options.HeightAndHashPresent() {
		// Fetch latest commit (nil means latest height).
		latestCommit, err := client.Commit(nil)
		if err != nil {
			return nil, err
		}
		err = vp.UpdateToHeight(chainID, latestCommit.SignedHeader.Height)
		if err != nil {
			return nil, err
		}
	}

	return vp, nil
}

// initProvider sets up the databases and loggers and instantiates the Provider
// object.
func initProvider(chainID, rootDir string, client lclient.SignStatusClient,
	logger log.Logger, cacheSize int, options TrustOptions) *Provider {

	logger = logger.With("module", loggerPath)
	logger.Info("lite/verifying/NewProvider", "chainID", chainID, "rootDir", rootDir, "client", client)

	trust := lite.NewMultiProvider(
		lite.NewDBProvider(memDBFile, dbm.NewMemDB()).SetLimit(cacheSize),
		lite.NewDBProvider(lvlDBFile, dbm.NewDB(dbName, dbm.GoLevelDBBackend, rootDir)),
	)

	return makeProvider(chainID, options.TrustPeriod, trust, lclient.NewProvider(chainID, client), logger)
}

// makeProvider returns a new verifying Provider. It uses the trusted Provider
// to store validated data and the source Provider to obtain missing data (e.g.
// FullCommits).
//
// The trusted Provider should be a DBProvider.
// The source Provider should be a client.HTTPProvider.
func makeProvider(chainID string, trustPeriod time.Duration,
	trusted lite.PersistentProvider, source lite.Provider, logger log.Logger) *Provider {

	if trustPeriod == 0 {
		panic("Provider must have non-zero trust period")
	}
	logger = logger.With("module", loggerPath)
	trusted.SetLogger(logger)
	source.SetLogger(logger)
	return &Provider{
		logger:               logger,
		chainID:              chainID,
		trustPeriod:          trustPeriod,
		trusted:              trusted,
		source:               source,
		pendingVerifications: make(map[int64]chan struct{}, sizeOfPendingMap),
	}
}

// getTrustedCommit returns a commit trusted with weak subjectivity. It either:
// 1. Fetches a commit at height provided in options and ensures the specified
// commit is within the trust period of latest block
// 2. Trusts the remote node and gets the latest commit
// 3. Returns an error if the height provided in trust option is too old to
// sync to latest.
func getTrustedCommit(logger log.Logger, client lclient.SignStatusClient, options TrustOptions) (types.SignedHeader, error) {
	// Get the latest commit always.
	latestCommit, err := client.Commit(nil)
	if err != nil {
		return types.SignedHeader{}, err
	}

	// If the user has set a root of trust, confirm it then update to newest.
	if options.HeightAndHashPresent() {
		trustCommit, err := client.Commit(&options.TrustHeight)
		if err != nil {
			return types.SignedHeader{}, err
		}

		if latestCommit.Time.Sub(trustCommit.Time) > options.TrustPeriod {
			return types.SignedHeader{},
				errors.New("your trusted block height is older than the trust period from latest block")
		}

		signedHeader := trustCommit.SignedHeader
		if !bytes.Equal(signedHeader.Hash(), options.TrustHash) {
			return types.SignedHeader{},
				fmt.Errorf("WARNING: expected hash %X but got %X", options.TrustHash, signedHeader.Hash())
		}
		return signedHeader, nil
	}

	signedHeader := latestCommit.SignedHeader

	// NOTE: This should really belong in the callback.
	// WARN THE USER IN ALL CAPS THAT THE LITE CLIENT IS NEW, AND THAT WE WILL
	// SYNC TO AND VERIFY LATEST COMMIT.
	logger.Info("WARNING: trusting source at height %d and hash %X...\n", signedHeader.Height, signedHeader.Hash())
	if options.Callback != nil {
		err := options.Callback(signedHeader.Height, signedHeader.Hash())
		if err != nil {
			return types.SignedHeader{}, err
		}
	}
	return signedHeader, nil
}

func (vp *Provider) Verify(signedHeader types.SignedHeader) error {
	if signedHeader.ChainID != vp.chainID {
		return fmt.Errorf("expected chainID %s, got %s", vp.chainID, signedHeader.ChainID)
	}

	valSet, err := vp.ValidatorSet(signedHeader.ChainID, signedHeader.Height)
	if err != nil {
		return err
	}

	if signedHeader.Height < vp.height {
		return fmt.Errorf("expected height %d, got %d", vp.height, signedHeader.Height)
	}

	if !bytes.Equal(signedHeader.ValidatorsHash, valSet.Hash()) {
		return lerr.ErrUnexpectedValidators(signedHeader.ValidatorsHash, valSet.Hash())
	}

	err = signedHeader.ValidateBasic(vp.chainID)
	if err != nil {
		return err
	}

	// Check commit signatures.
	err = valSet.VerifyCommit(vp.chainID, signedHeader.Commit.BlockID, signedHeader.Height, signedHeader.Commit)
	if err != nil {
		return err
	}

	return nil
}

// SetLogger implements lite.Provider.
func (vp *Provider) SetLogger(logger log.Logger) {
	vp.logger = logger
	vp.trusted.SetLogger(logger)
	vp.source.SetLogger(logger)
}

func (vp *Provider) ChainID() string { return vp.chainID }

// UpdateToHeight implements lite.UpdatingProvider
//
// On success, it will store the full commit (SignedHeader + Validators) in
// vp.trusted.
// NOTE: For concurrent usage, use ConcurrentProvider.
func (vp *Provider) UpdateToHeight(chainID string, height int64) error {
	// If we alreedy have the commit, just return nil
	_, err := vp.trusted.LatestFullCommit(vp.chainID, height, height)
	if err == nil {
		return nil
	} else if !lerr.IsErrCommitNotFound(err) {
		// Return error if it is not CommitNotFound error
		vp.logger.Error("Encountered unknown error while loading full commit", "height", height)
		return err
	}

	// Fetch trusted FC at exactly height, while updating trust when possible.
	_, err = vp.fetchAndVerifyToHeightBisecting(height)
	if err != nil {
		return err
	}

	vp.height = height

	// Good!
	return nil
}

// If valset or nextValset are nil, fetches them.
// Then validates full commit, then saves it.
func (vp *Provider) fillValsetAndSaveFC(signedHeader types.SignedHeader,
	valset, nextValset *types.ValidatorSet) (err error) {

	// If there is no valset passed, fetch it
	if valset == nil {
		valset, err = vp.source.ValidatorSet(vp.chainID, signedHeader.Height)
		if err != nil {
			return cmn.ErrorWrap(err, "fetching the valset")
		}
	}

	// If there is no nextvalset passed, fetch it
	if nextValset == nil {
		// TODO: Don't loop forever, just do it 10 times
		for {
			// fetch block at signedHeader.Height+1
			nextValset, err = vp.source.ValidatorSet(vp.chainID, signedHeader.Height+1)
			if lerr.IsErrUnknownValidators(err) {
				// try again until we get it.
				fmt.Printf("fetching validatorset for height %v...\n", signedHeader.Height+1)
				continue
			} else if err != nil {
				return cmn.ErrorWrap(err, "fetching the next valset")
			} else if nextValset != nil {
				break
			}
		}
	}

	// Create filled FullCommit.
	fc := lite.FullCommit{
		SignedHeader:   signedHeader,
		Validators:     valset,
		NextValidators: nextValset,
	}

	// Validate the full commit.  This checks the cryptographic
	// signatures of Commit against Validators.
	if err := fc.ValidateFull(vp.chainID); err != nil {
		return cmn.ErrorWrap(err, "verifying validators from source")
	}

	// Trust it.
	err = vp.trusted.SaveFullCommit(fc)
	if err != nil {
		return cmn.ErrorWrap(err, "saving full commit")
	}

	return nil
}

// verifyAndSave will verify if this is a valid source full commit given the
// best match trusted full commit, and persist to vp.trusted.
//
// Returns ErrTooMuchChange when >2/3 of trustedFC did not sign newFC.
// Returns ErrCommitExpired when trustedFC is too old.
// Panics if trustedFC.Height() >= newFC.Height().
func (vp *Provider) verifyAndSave(trustedFC, newFC lite.FullCommit) error {

	// Shouldn't have trusted commits before the new commit height
	if trustedFC.Height() >= newFC.Height() {
		panic("should not happen")
	}

	// Check that the latest commit isn't beyond the vp.trustPeriod
	if vp.now().Sub(trustedFC.SignedHeader.Time) > vp.trustPeriod {
		return lerr.ErrCommitExpired()
	}

	// Validate the new commit in terms of validator set of last trusted commit.
	if err := trustedFC.NextValidators.VerifyCommit(vp.chainID, newFC.SignedHeader.Commit.BlockID, newFC.SignedHeader.Height, newFC.SignedHeader.Commit); err != nil {
		return err
	}

	//Locally validate the full commit before we can trust it.
	if newFC.Height() >= trustedFC.Height()+1 {
		err := newFC.ValidateFull(vp.chainID)

		if err != nil {
			return err
		}
	}
	change := CompareVotingPowers(trustedFC, newFC)

	if change > float64(1/3) {
		return lerr.ErrValidatorChange(change)
	}

	return vp.trusted.SaveFullCommit(newFC)
}

func CompareVotingPowers(trustedFC, newFC lite.FullCommit) float64 {

	var diffAccumulator float64

	for _, val := range newFC.Validators.Validators {

		newPowerRatio := float64(val.VotingPower) / float64(newFC.Validators.TotalVotingPower())

		_, tval := trustedFC.NextValidators.GetByAddress(val.Address)

		oldPowerRatio := float64(tval.VotingPower) / float64(trustedFC.NextValidators.TotalVotingPower())

		diffAccumulator += math.Abs(newPowerRatio - oldPowerRatio)

	}

	return diffAccumulator

}

func (vp *Provider) fetchAndVerifyToHeightLinear(h int64) (lite.FullCommit, error) {

	// Fetch latest full commit from source.
	sourceFC, err := vp.source.LatestFullCommit(vp.chainID, h, h)
	if err != nil {
		return lite.FullCommit{}, err
	}

	// If sourceFC.Height() != h, we can't do it.
	if sourceFC.Height() != h {
		return lite.FullCommit{}, lerr.ErrCommitNotFound()
	}

	// Validate the full commit.  This checks the cryptographic
	// signatures of Commit against Validators.
	if err := sourceFC.ValidateFull(vp.chainID); err != nil {
		return lite.FullCommit{}, err
	}

	if h == sourceFC.Height()+1 {
		trustedFC, err := vp.trusted.LatestFullCommit(vp.chainID, 1, h)
		if err != nil {
			return lite.FullCommit{}, err
		}

		err = vp.verifyAndSave(trustedFC, sourceFC)

		if err != nil {
			return lite.FullCommit{}, err
		}
		return sourceFC, nil
	}

	// Verify latest FullCommit against trusted FullCommits
	// Use a loop rather than recursion to avoid stack overflows.
	for {
		// Fetch latest full commit from trusted.
		trustedFC, err := vp.trusted.LatestFullCommit(vp.chainID, 1, h)
		if err != nil {
			return lite.FullCommit{}, err
		}

		// We have nothing to do.
		if trustedFC.Height() == h {
			return trustedFC, nil
		}
		sourceFC, err = vp.source.LatestFullCommit(vp.chainID, trustedFC.Height()+1, trustedFC.Height()+1)

		if err != nil {
			return lite.FullCommit{}, err
		}
		err = vp.verifyAndSave(trustedFC, sourceFC)

		if err != nil {
			return lite.FullCommit{}, err
		}
	}
}

// fetchAndVerifyToHeightBiscecting will use divide-and-conquer to find a path to h.
// Returns nil error iff we successfully verify for height h, using repeated
// applications of bisection if necessary.
// Along the way, if a recent trust is used to verify a more recent header, the
// more recent header becomes trusted.
//
// Returns ErrCommitNotFound if source Provider doesn't have the commit for h.
func (vp *Provider) fetchAndVerifyToHeightBisecting(h int64) (lite.FullCommit, error) {

	// Fetch latest full commit from source.
	sourceFC, err := vp.source.LatestFullCommit(vp.chainID, h, h)
	if err != nil {
		return lite.FullCommit{}, err
	}

	// If sourceFC.Height() != h, we can't do it.
	if sourceFC.Height() != h {
		return lite.FullCommit{}, lerr.ErrCommitNotFound()
	}

	// Validate the full commit.  This checks the cryptographic
	// signatures of Commit against Validators.
	if err := sourceFC.ValidateFull(vp.chainID); err != nil {
		return lite.FullCommit{}, err
	}

	// Verify latest FullCommit against trusted FullCommits
	// Use a loop rather than recursion to avoid stack overflows.
	for {
		// Fetch latest full commit from trusted.
		trustedFC, err := vp.trusted.LatestFullCommit(vp.chainID, 1, h)
		if err != nil {
			return lite.FullCommit{}, err
		}

		// We have nothing to do.
		if trustedFC.Height() == h {
			return trustedFC, nil
		}

		// Update to full commit with checks.
		err = vp.verifyAndSave(trustedFC, sourceFC)

		// Handle special case when err is ErrTooMuchChange.
		if types.IsErrTooMuchChange(err) {
			// Divide and conquer.
			start, end := trustedFC.Height(), sourceFC.Height()
			if !(start < end) {
				panic("should not happen")
			}
			mid := (start + end) / 2

			// Recursive call back into fetchAndVerifyToHeight. Once you get to an inner
			// call that succeeeds, the outer calls will succeed.
			_, err = vp.fetchAndVerifyToHeightBisecting(mid)
			if err != nil {
				return lite.FullCommit{}, err
			}
			// If we made it to mid, we retry.
			continue
		} else if err != nil {
			return lite.FullCommit{}, err
		}

		// All good!
		return sourceFC, nil
	}
}

func (vp *Provider) LastTrustedHeight() int64 {
	fc, err := vp.trusted.LatestFullCommit(vp.chainID, 1, 1<<63-1)
	if err != nil {
		panic("should not happen")
	}
	return fc.Height()
}

func (vp *Provider) LatestFullCommit(chainID string, minHeight, maxHeight int64) (lite.FullCommit, error) {
	return vp.trusted.LatestFullCommit(chainID, minHeight, maxHeight)
}

func (vp *Provider) ValidatorSet(chainID string, height int64) (*types.ValidatorSet, error) {
	// XXX try to sync?
	return vp.trusted.ValidatorSet(chainID, height)
}
