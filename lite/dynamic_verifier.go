package lite

import (
	"bytes"

	log "github.com/tendermint/tendermint/libs/log"
	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
)

var _ Verifier = (*DynamicVerifier)(nil)

// DynamicVerifier implements an auto-updating Verifier.  It uses a
// "source" provider to obtain the needed FullCommits to securely sync with
// validator set changes.  It stores properly validated data on the
// "trusted" local system.
type DynamicVerifier struct {
	logger  log.Logger
	chainID string
	// These are only properly validated data, from local system.
	trusted PersistentProvider
	// This is a source of new info, like a node rpc, or other import method.
	source Provider
}

// NewDynamicVerifier returns a new DynamicVerifier. It uses the
// trusted provider to store validated data and the source provider to
// obtain missing data (e.g. FullCommits).
//
// The trusted provider should a CacheProvider, MemProvider or
// files.Provider.  The source provider should be a client.HTTPProvider.
func NewDynamicVerifier(chainID string, trusted PersistentProvider, source Provider) *DynamicVerifier {
	return &DynamicVerifier{
		logger:  log.NewNopLogger(),
		chainID: chainID,
		trusted: trusted,
		source:  source,
	}
}

func (ic *DynamicVerifier) SetLogger(logger log.Logger) {
	logger = logger.With("module", "lite")
	ic.logger = logger
	ic.trusted.SetLogger(logger)
	ic.source.SetLogger(logger)
}

// Implements Verifier.
func (ic *DynamicVerifier) ChainID() string {
	return ic.chainID
}

// Implements Verifier.
//
// If the validators have changed since the last known time, it looks to
// ic.trusted and ic.source to prove the new validators.  On success, it will
// try to store the SignedHeader in ic.trusted if the next
// validator can be sourced.
func (ic *DynamicVerifier) Certify(shdr types.SignedHeader) error {

	// Get the latest known full commit <= h-1 from our trusted providers.
	// The full commit at h-1 contains the valset to sign for h.
	h := shdr.Height - 1
	trustedFC, err := ic.trusted.LatestFullCommit(ic.chainID, 1, h)
	if err != nil {
		return err
	}

	if trustedFC.Height() == h {
		// Return error if valset doesn't match.
		if !bytes.Equal(
			trustedFC.NextValidators.Hash(),
			shdr.Header.ValidatorsHash) {
			return lerr.ErrUnexpectedValidators(
				trustedFC.NextValidators.Hash(),
				shdr.Header.ValidatorsHash)
		}
	} else {
		// If valset doesn't match...
		if !bytes.Equal(trustedFC.NextValidators.Hash(),
			shdr.Header.ValidatorsHash) {
			// ... update.
			trustedFC, err = ic.updateToHeight(h)
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

	// Certify the signed header using the matching valset.
	cert := NewBaseVerifier(ic.chainID, trustedFC.Height()+1, trustedFC.NextValidators)
	err = cert.Certify(shdr)
	if err != nil {
		return err
	}

	// Get the next validator set.
	nextValset, err := ic.source.ValidatorSet(ic.chainID, shdr.Height+1)
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
	if err := nfc.ValidateFull(ic.chainID); err != nil {
		return err
	}
	// Trust it.
	return ic.trusted.SaveFullCommit(nfc)
}

// verifyAndSave will verify if this is a valid source full commit given the
// best match trusted full commit, and if good, persist to ic.trusted.
// Returns ErrTooMuchChange when >2/3 of trustedFC did not sign sourceFC.
// Panics if trustedFC.Height() >= sourceFC.Height().
func (ic *DynamicVerifier) verifyAndSave(trustedFC, sourceFC FullCommit) error {
	if trustedFC.Height() >= sourceFC.Height() {
		panic("should not happen")
	}
	err := trustedFC.NextValidators.VerifyFutureCommit(
		sourceFC.Validators,
		ic.chainID, sourceFC.SignedHeader.Commit.BlockID,
		sourceFC.SignedHeader.Height, sourceFC.SignedHeader.Commit,
	)
	if err != nil {
		return err
	}

	return ic.trusted.SaveFullCommit(sourceFC)
}

// updateToHeight will use divide-and-conquer to find a path to h.
// Returns nil error iff we successfully verify and persist a full commit
// for height h, using repeated applications of bisection if necessary.
//
// Returns ErrCommitNotFound if source provider doesn't have the commit for h.
func (ic *DynamicVerifier) updateToHeight(h int64) (FullCommit, error) {

	// Fetch latest full commit from source.
	sourceFC, err := ic.source.LatestFullCommit(ic.chainID, h, h)
	if err != nil {
		return FullCommit{}, err
	}

	// Validate the full commit.  This checks the cryptographic
	// signatures of Commit against Validators.
	if err := sourceFC.ValidateFull(ic.chainID); err != nil {
		return FullCommit{}, err
	}

	// If sourceFC.Height() != h, we can't do it.
	if sourceFC.Height() != h {
		return FullCommit{}, lerr.ErrCommitNotFound()
	}

FOR_LOOP:
	for {
		// Fetch latest full commit from trusted.
		trustedFC, err := ic.trusted.LatestFullCommit(ic.chainID, 1, h)
		if err != nil {
			return FullCommit{}, err
		}
		// We have nothing to do.
		if trustedFC.Height() == h {
			return trustedFC, nil
		}

		// Try to update to full commit with checks.
		err = ic.verifyAndSave(trustedFC, sourceFC)
		if err == nil {
			// All good!
			return sourceFC, nil
		}

		// Handle special case when err is ErrTooMuchChange.
		if lerr.IsErrTooMuchChange(err) {
			// Divide and conquer.
			start, end := trustedFC.Height(), sourceFC.Height()
			if !(start < end) {
				panic("should not happen")
			}
			mid := (start + end) / 2
			_, err = ic.updateToHeight(mid)
			if err != nil {
				return FullCommit{}, err
			}
			// If we made it to mid, we retry.
			continue FOR_LOOP
		}
		return FullCommit{}, err
	}
}

func (ic *DynamicVerifier) LastTrustedHeight() int64 {
	fc, err := ic.trusted.LatestFullCommit(ic.chainID, 1, 1<<63-1)
	if err != nil {
		panic("should not happen")
	}
	return fc.Height()
}
