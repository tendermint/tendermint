package lite

import (
	"bytes"
	"fmt"
	"sync"

	log "github.com/tendermint/tendermint/libs/log"
	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
)

const sizeOfPendingMap = 1024

var _ Verifier = (*DynamicVerifier)(nil)

// DynamicVerifier implements an auto-updating Verifier.  It uses a
// "source" provider to obtain the needed FullCommits to securely sync with
// validator set changes.  It stores properly validated data on the
// "trusted" local system.
// TODO: make this single threaded and create a new
// ConcurrentDynamicVerifier that wraps it with concurrency.
// see https://github.com/tendermint/tendermint/issues/3170
type DynamicVerifier struct {
	chainID string
	logger  log.Logger

	// Already validated, stored locally
	trusted PersistentProvider

	// New info, like a node rpc, or other import method.
	source Provider

	// pending map to synchronize concurrent verification requests
	mtx                  sync.Mutex
	pendingVerifications map[int64]chan struct{}
}

// NewDynamicVerifier returns a new DynamicVerifier. It uses the
// trusted provider to store validated data and the source provider to
// obtain missing data (e.g. FullCommits).
//
// The trusted provider should be a DBProvider.
// The source provider should be a client.HTTPProvider.
func NewDynamicVerifier(chainID string, trusted PersistentProvider, source Provider) *DynamicVerifier {
	return &DynamicVerifier{
		logger:               log.NewNopLogger(),
		chainID:              chainID,
		trusted:              trusted,
		source:               source,
		pendingVerifications: make(map[int64]chan struct{}, sizeOfPendingMap),
	}
}

func (dv *DynamicVerifier) SetLogger(logger log.Logger) {
	logger = logger.With("module", "lite")
	dv.logger = logger
	dv.trusted.SetLogger(logger)
	dv.source.SetLogger(logger)
}

// Implements Verifier.
func (dv *DynamicVerifier) ChainID() string {
	return dv.chainID
}

// Implements Verifier.
//
// If the validators have changed since the last known time, it looks to
// dv.trusted and dv.source to prove the new validators.  On success, it will
// try to store the SignedHeader in dv.trusted if the next
// validator can be sourced.
func (dv *DynamicVerifier) Verify(shdr types.SignedHeader) error {

	// Performs synchronization for multi-threads verification at the same height.
	dv.mtx.Lock()
	if pending := dv.pendingVerifications[shdr.Height]; pending != nil {
		dv.mtx.Unlock()
		<-pending // pending is chan struct{}
	} else {
		pending := make(chan struct{})
		dv.pendingVerifications[shdr.Height] = pending
		defer func() {
			close(pending)
			dv.mtx.Lock()
			delete(dv.pendingVerifications, shdr.Height)
			dv.mtx.Unlock()
		}()
		dv.mtx.Unlock()
	}

	//Get the exact trusted commit for h, and if it is
	// equal to shdr, then it's already trusted, so
	// just return nil.
	trustedFCSameHeight, err := dv.trusted.LatestFullCommit(dv.chainID, shdr.Height, shdr.Height)
	if err == nil {
		// If loading trust commit successfully, and trust commit equal to shdr, then don't verify it,
		// just return nil.
		if bytes.Equal(trustedFCSameHeight.SignedHeader.Hash(), shdr.Hash()) {
			dv.logger.Info(fmt.Sprintf("Load full commit at height %d from cache, there is not need to verify.", shdr.Height))
			return nil
		}
	} else if !lerr.IsErrCommitNotFound(err) {
		// Return error if it is not CommitNotFound error
		dv.logger.Info(fmt.Sprintf("Encountered unknown error in loading full commit at height %d.", shdr.Height))
		return err
	}

	// Get the latest known full commit <= h-1 from our trusted providers.
	// The full commit at h-1 contains the valset to sign for h.
	prevHeight := shdr.Height - 1
	trustedFC, err := dv.trusted.LatestFullCommit(dv.chainID, 1, prevHeight)
	if err != nil {
		return err
	}

	// sync up to the prevHeight and assert our latest NextValidatorSet
	// is the ValidatorSet for the SignedHeader
	if trustedFC.Height() == prevHeight {
		// Return error if valset doesn't match.
		if !bytes.Equal(
			trustedFC.NextValidators.Hash(),
			shdr.Header.ValidatorsHash) {
			return lerr.ErrUnexpectedValidators(
				trustedFC.NextValidators.Hash(),
				shdr.Header.ValidatorsHash)
		}
	} else {
		// If valset doesn't match, try to update
		if !bytes.Equal(
			trustedFC.NextValidators.Hash(),
			shdr.Header.ValidatorsHash) {
			// ... update.
			trustedFC, err = dv.updateToHeight(prevHeight)
			if err != nil {
				return err
			}
			// Return error if valset _still_ doesn't match.
			if !bytes.Equal(trustedFC.NextValidators.Hash(),
				shdr.Header.ValidatorsHash) {
				return lerr.ErrUnexpectedValidators(
					trustedFC.NextValidators.Hash(),
					shdr.Header.ValidatorsHash)
			}
		}
	}

	// Verify the signed header using the matching valset.
	cert := NewBaseVerifier(dv.chainID, trustedFC.Height()+1, trustedFC.NextValidators)
	err = cert.Verify(shdr)
	if err != nil {
		return err
	}

	// By now, the SignedHeader is fully validated and we're synced up to
	// SignedHeader.Height - 1. To sync to SignedHeader.Height, we need
	// the validator set at SignedHeader.Height + 1 so we can verify the
	// SignedHeader.NextValidatorSet.
	// TODO: is the ValidateFull below mostly redundant with the BaseVerifier.Verify above?
	// See https://github.com/tendermint/tendermint/issues/3174.

	// Get the next validator set.
	nextValset, err := dv.source.ValidatorSet(dv.chainID, shdr.Height+1)
	if lerr.IsErrUnknownValidators(err) {
		// Ignore this error.
		return nil
	} else if err != nil {
		return err
	}

	// Create filled FullCommit.
	nfc := FullCommit{
		SignedHeader:   shdr,
		Validators:     trustedFC.NextValidators,
		NextValidators: nextValset,
	}
	// Validate the full commit.  This checks the cryptographic
	// signatures of Commit against Validators.
	if err := nfc.ValidateFull(dv.chainID); err != nil {
		return err
	}
	// Trust it.
	return dv.trusted.SaveFullCommit(nfc)
}

// verifyAndSave will verify if this is a valid source full commit given the
// best match trusted full commit, and if good, persist to dv.trusted.
// Returns ErrTooMuchChange when >2/3 of trustedFC did not sign sourceFC.
// Panics if trustedFC.Height() >= sourceFC.Height().
func (dv *DynamicVerifier) verifyAndSave(trustedFC, sourceFC FullCommit) error {
	if trustedFC.Height() >= sourceFC.Height() {
		panic("should not happen")
	}
	err := trustedFC.NextValidators.VerifyFutureCommit(
		sourceFC.Validators,
		dv.chainID, sourceFC.SignedHeader.Commit.BlockID,
		sourceFC.SignedHeader.Height, sourceFC.SignedHeader.Commit,
	)
	if err != nil {
		return err
	}

	return dv.trusted.SaveFullCommit(sourceFC)
}

// updateToHeight will use divide-and-conquer to find a path to h.
// Returns nil error iff we successfully verify and persist a full commit
// for height h, using repeated applications of bisection if necessary.
//
// Returns ErrCommitNotFound if source provider doesn't have the commit for h.
func (dv *DynamicVerifier) updateToHeight(h int64) (FullCommit, error) {

	// Fetch latest full commit from source.
	sourceFC, err := dv.source.LatestFullCommit(dv.chainID, h, h)
	if err != nil {
		return FullCommit{}, err
	}

	// If sourceFC.Height() != h, we can't do it.
	if sourceFC.Height() != h {
		return FullCommit{}, lerr.ErrCommitNotFound()
	}

	// Validate the full commit.  This checks the cryptographic
	// signatures of Commit against Validators.
	if err := sourceFC.ValidateFull(dv.chainID); err != nil {
		return FullCommit{}, err
	}

	// Verify latest FullCommit against trusted FullCommits
FOR_LOOP:
	for {
		// Fetch latest full commit from trusted.
		trustedFC, err := dv.trusted.LatestFullCommit(dv.chainID, 1, h)
		if err != nil {
			return FullCommit{}, err
		}
		// We have nothing to do.
		if trustedFC.Height() == h {
			return trustedFC, nil
		}

		// Try to update to full commit with checks.
		err = dv.verifyAndSave(trustedFC, sourceFC)
		if err == nil {
			// All good!
			return sourceFC, nil
		}

		// Handle special case when err is ErrTooMuchChange.
		if types.IsErrTooMuchChange(err) {
			// Divide and conquer.
			start, end := trustedFC.Height(), sourceFC.Height()
			if !(start < end) {
				panic("should not happen")
			}
			mid := (start + end) / 2
			_, err = dv.updateToHeight(mid)
			if err != nil {
				return FullCommit{}, err
			}
			// If we made it to mid, we retry.
			continue FOR_LOOP
		}
		return FullCommit{}, err
	}
}

func (dv *DynamicVerifier) LastTrustedHeight() int64 {
	fc, err := dv.trusted.LatestFullCommit(dv.chainID, 1, 1<<63-1)
	if err != nil {
		panic("should not happen")
	}
	return fc.Height()
}
