package verifying

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	log "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/lite"
	lclient "github.com/tendermint/tendermint/lite/client"
	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
)

type TrustOptions struct {
	// Required: only trust commits up to this old.
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

// NOTE If you retain the resulting verifier in memory for a long time,
// usage of the verifier may eventually error, but immediate usage should
// not error like that, so that e.g. cli usage never errors unexpectedly.
// TODO Move some of this initialization to a separate function.
func NewProvider(chainID, rootDir string, client lclient.SignStatusClient, logger log.Logger, cacheSize int, options TrustOptions) (*provider, error) {

	logger = logger.With("module", "lite/proxy")
	logger.Info("lite/proxy/NewProvider()...", "chainID", chainID, "rootDir", rootDir, "client", client)

	memProvider := lite.NewDBProvider("trusted.mem", dbm.NewMemDB()).SetLimit(cacheSize)
	lvlProvider := lite.NewDBProvider("trusted.lvl", dbm.NewDB("trust-base", dbm.LevelDBBackend, rootDir))
	trust := lite.NewMultiProvider(
		memProvider,
		lvlProvider,
	)
	source := lclient.NewProvider(chainID, client)
	vp := makeProvider(chainID, options.TrustPeriod, trust, source)
	vp.SetLogger(logger)

	// Get the latest source commit, or the one provided in options.
	trustCommit, err := getTrustCommit(client, options)
	if err != nil {
		return nil, err
	}

	err = vp.fillValidateAndSaveToTrust(trustCommit, nil, nil)
	if err != nil {
		return nil, err
	}

	// sanity check
	if time.Now().Sub(trustCommit.Time) <= 0 {
		panic(fmt.Sprintf("impossible time %v vs %v", time.Now(), trustCommit.Time))
	}

	// Otherwise we're syncing within the unbonding period.
	// NOTE: There is a duplication of fetching this latest commit (since
	// UpdateToHeight() will fetch it again, and latestCommit isn't used), but
	// it's only once upon initialization of a validator so it's not a big
	// deal.
	if options.TrustHeight > 0 {
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

// getTragetCommit returns a commit trusted with weak subjectivity. It either:
// 1. Fetches a commit at height provided in options and ensures the specified commit
// 	is within the trust period of latest block
// 2. Trusts the remote node and gets the latest commit
// 3. Returns an error if the height provided in trust option is too old to sync to latest.
func getTrustCommit(client lclient.SignStatusClient, options TrustOptions) (types.SignedHeader, error) {

	// Get the lastest commit always
	latestBlock, err := client.Commit(nil)
	if err != nil {
		return types.SignedHeader{}, err
	}

	if options.TrustHeight != 0 {
		trustBlock, err := client.Commit(&options.TrustHeight)
		if err != nil {
			return types.SignedHeader{}, err
		}

		if latestBlock.Time.Sub(trustBlock.Time) > options.TrustPeriod {
			return types.SignedHeader{}, fmt.Errorf("Your Trusted Block Height is older than the trust period from Latest Block")
		}

		trustCommit := trustBlock.SignedHeader
		if !bytes.Equal(trustCommit.Hash(), options.TrustHash) {
			return types.SignedHeader{}, fmt.Errorf("WARNING!!! Expected height/hash %v/%X but got %X",
				options.TrustHeight, options.TrustHash, trustCommit.Hash())
		}
		return trustCommit, nil
	} else {

		latestCommit := latestBlock.SignedHeader

		// NOTE: This should really belong in the callback.
		// WARN THE USER IN ALL CAPS THAT THE LITE CLIENT IS NEW,
		// AND THAT WE WILL SYNC TO AND VERIFY LATEST COMMIT.
		fmt.Printf("trusting source at height %v and hash %X...\n", latestCommit.Height, latestCommit.Hash())
		if options.Callback != nil {
			err := options.Callback(latestCommit.Height, latestCommit.Hash())
			if err != nil {
				return types.SignedHeader{}, err
			}
		}
		return latestCommit, nil
	}
}

//----------------------------------------

type nowFn func() time.Time

const sizeOfPendingMap = 1024

var _ lite.UpdatingProvider = (*provider)(nil)

// provider implements a persistent caching provider that
// auto-validates.  It uses a "source" provider to obtain the needed
// FullCommits to securely sync with validator set changes.  It stores properly
// validated data on the "trusted" local system.
// NOTE: This provider can only work with one chainID, provided upon
// instantiation.
type provider struct {
	chainID     string
	logger      log.Logger
	trustPeriod time.Duration // e.g. the unbonding period, or something smaller.
	now         nowFn

	// Already validated, stored locally
	trusted lite.PersistentProvider

	// New info, like a node rpc, or other import method.
	source lite.Provider

	// pending map to synchronize concurrent verification requests
	mtx                  sync.Mutex
	pendingVerifications map[int64]chan struct{}
}

// makeProvider returns a new verifying provider. It uses the
// trusted provider to store validated data and the source provider to
// obtain missing data (e.g. FullCommits).
//
// The trusted provider should be a DBProvider.
// The source provider should be a client.HTTPProvider.
// NOTE: The external facing constructor is called NewVerifyingProivider.
func makeProvider(chainID string, trustPeriod time.Duration, trusted lite.PersistentProvider, source lite.Provider) *provider {
	if trustPeriod == 0 {
		panic("VerifyingProvider must have non-zero trust period")
	}
	return &provider{
		logger:               log.NewNopLogger(),
		chainID:              chainID,
		trustPeriod:          trustPeriod,
		trusted:              trusted,
		source:               source,
		pendingVerifications: make(map[int64]chan struct{}, sizeOfPendingMap),
	}
}

func (vp *provider) SetLogger(logger log.Logger) {
	logger = logger.With("module", "lite")
	vp.logger = logger
	vp.trusted.SetLogger(logger)
	vp.source.SetLogger(logger)
}

func (vp *provider) ChainID() string {
	return vp.chainID
}

// Implements UpdatingProvider
//
// On success, it will store the full commit (SignedHeader + Validators) in vp.trusted
// NOTE: For concurreent usage, use concurrentProvider
func (vp *provider) UpdateToHeight(chainID string, height int64) error {

	// If we alreedy have the commit, just return nil
	_, err := vp.trusted.LatestFullCommit(vp.chainID, height, height)
	if err == nil {
		return nil
	} else if !lerr.IsErrCommitNotFound(err) {
		// Return error if it is not CommitNotFound error
		vp.logger.Info(fmt.Sprintf("Encountered unknown error in loading full commit at height %d.", height))
		return err
	}

	// Fetch trusted FC at exactly height, while updating trust when possible.
	_, err = vp.fetchAndVerifyToHeight(height)
	if err != nil {
		return err
	}

	// Good!
	return nil
}

// If valset or nextValset are nil, fetches them.
// Then, validatees the full commit, then saves it.
func (vp *provider) fillValidateAndSaveToTrust(signedHeader types.SignedHeader, valset, nextValset *types.ValidatorSet) (err error) {

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
func (vp *provider) verifyAndSave(trustedFC, newFC lite.FullCommit) error {

	// Shouldn't have trusted commits before the new commit height
	if trustedFC.Height() >= newFC.Height() {
		panic("should not happen")
	}

	// Check that the latest commit isn't beyond the vp.trustPeriod
	if vp.now().Sub(trustedFC.SignedHeader.Time) > vp.trustPeriod {
		return lerr.ErrCommitExpired()
	}

	// If the new full commit is the next block, verify it. Otherwise use the verify future commit function
	if err := trustedFC.NextValidators.VerifyCommit(vp.chainID, newFC.SignedHeader.Commit.BlockID, newFC.SignedHeader.Height, newFC.SignedHeader.Commit); err != nil {
		return err
	}

	return vp.trusted.SaveFullCommit(newFC)
}

// fetchAndVerifyToHeight will use divide-and-conquer to find a path to h.
// Returns nil error iff we successfully verify for height h, using repeated
// applications of bisection if necessary.
// Along the way, if a recent trust is used to verify a more recent header, the
// more recent header becomes trusted.
//
// Returns ErrCommitNotFound if source provider doesn't have the commit for h.
func (vp *provider) fetchAndVerifyToHeight(h int64) (lite.FullCommit, error) {

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
			_, err = vp.fetchAndVerifyToHeight(mid)
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

func (vp *provider) LastTrustedHeight() int64 {
	fc, err := vp.trusted.LatestFullCommit(vp.chainID, 1, 1<<63-1)
	if err != nil {
		panic("should not happen")
	}
	return fc.Height()
}

func (vp *provider) LatestFullCommit(chainID string, minHeight, maxHeight int64) (lite.FullCommit, error) {
	return vp.trusted.LatestFullCommit(chainID, minHeight, maxHeight)
}

func (vp *provider) ValidatorSet(chainID string, height int64) (*types.ValidatorSet, error) {
	// XXX try to sync?
	return vp.trusted.ValidatorSet(chainID, height)
}
