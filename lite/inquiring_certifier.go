package lite

import (
	"bytes"

	"github.com/tendermint/tendermint/types"

	lerr "github.com/tendermint/tendermint/lite/errors"
)

var _ Certifier = (*InquiringCertifier)(nil)

// InquiringCertifier implements an auto-updating certifier.  It uses a
// "source" provider to obtain the needed FullCommits to securely sync with
// validator set changes.  It stores properly validated data on the
// "trusted" local system.
type InquiringCertifier struct {
	chainID string
	// These are only properly validated data, from local system.
	trusted PersistentProvider
	// This is a source of new info, like a node rpc, or other import method.
	source Provider
}

// NewInquiringCertifier returns a new InquiringCertifier. It uses the
// trusted provider to store validated data and the source provider to
// obtain missing data (e.g. FullCommits).
//
// The trusted provider should a CacheProvider, MemProvider or
// files.Provider.  The source provider should be a client.HTTPProvider.
func NewInquiringCertifier(chainID string, trusted PersistentProvider, source Provider) (
	*InquiringCertifier, error) {

	return &InquiringCertifier{
		chainID: chainID,
		trusted: trusted,
		source:  source,
	}, nil
}

// Implements Certifier.
func (ic *InquiringCertifier) ChainID() string {
	return ic.chainID
}

// Implements Certifier.
//
// If the validators have changed since the last know time, it looks to
// ic.trusted and ic.source to prove the new validators.  On success, it will
// try to store the SignedHeader in ic.trusted if the next
// validator can be sourced.
func (ic *InquiringCertifier) Certify(shdr types.SignedHeader) error {

	// Get the latest known full commit <= h-1 from our trusted providers.
	// The full commit at h-1 contains the valset to sign for h.
	h := shdr.Height - 1
	tfc, err := ic.trusted.LatestFullCommit(ic.chainID, 1, h)
	if err != nil {
		return err
	}

	if tfc.Height() == h {
		// Return error if valset doesn't match.
		if !bytes.Equal(
			tfc.NextValidators.Hash(),
			shdr.Header.ValidatorsHash) {
			return lerr.ErrUnexpectedValidators(
				tfc.NextValidators.Hash(),
				shdr.Header.ValidatorsHash)
		}
	} else {
		// If valset doesn't match...
		if !bytes.Equal(tfc.NextValidators.Hash(),
			shdr.Header.ValidatorsHash) {
			// ... update.
			tfc, err = ic.updateToHeight(h)
			if err != nil {
				return err
			}
			// Return error if valset _still_ doesn't match.
			if !bytes.Equal(tfc.NextValidators.Hash(),
				shdr.Header.ValidatorsHash) {
				return lerr.ErrUnexpectedValidators(
					tfc.NextValidators.Hash(),
					shdr.Header.ValidatorsHash)
			}
		}
	}

	// Certify the signed header using the matching valset.
	cert := NewBaseCertifier(ic.chainID, tfc.Height()+1, tfc.NextValidators)
	err = cert.Certify(shdr)
	if err != nil {
		return err
	}

	// Get the next validator set.
	nvalset, err := ic.source.ValidatorSet(ic.chainID, shdr.Height+1)
	if lerr.IsErrMissingValidators(err) {
		// Ignore this error.
		return nil
	} else if err != nil {
		return err
	} else {
		// Create filled FullCommit.
		nfc := FullCommit{
			SignedHeader:   shdr,
			Validators:     tfc.NextValidators,
			NextValidators: nvalset,
		}
		// Validate the full commit.  This checks the cryptographic
		// signatures of Commit against Validators.
		if err := nfc.ValidateBasic(ic.chainID); err != nil {
			return err
		}
		// Trust it.
		return ic.trusted.SaveFullCommit(nfc)
	}
}

// verifyAndSave will verify if this is a valid source full commit given the
// best match trusted full commit, and if good, persist to ic.trusted.
// Returns ErrTooMuchChange when >2/3 of tfc did not sign sfc.
// Panics if tfc.Height() >= sfc.Height().
func (ic *InquiringCertifier) verifyAndSave(tfc, sfc FullCommit) error {
	if tfc.Height() >= sfc.Height() {
		panic("should not happen")
	}
	err := tfc.NextValidators.VerifyFutureCommit(
		sfc.Validators,
		ic.chainID, sfc.SignedHeader.Commit.BlockID,
		sfc.SignedHeader.Height, sfc.SignedHeader.Commit,
	)
	if err != nil {
		return err
	}

	return ic.trusted.SaveFullCommit(sfc)
}

// updateToHeight will use divide-and-conquer to find a path to h.
// Returns nil iff we successfully verify and persist a full commit
// for height h, using repeated applications of bisection if necessary.
//
// Returns ErrCommitNotFound if source provider doesn't have the commit for h.
func (ic *InquiringCertifier) updateToHeight(h int64) (FullCommit, error) {

	// Fetch latest full commit from source.
	sfc, err := ic.source.LatestFullCommit(ic.chainID, h, h)
	if err != nil {
		return FullCommit{}, err
	}

	// Validate the full commit.  This checks the cryptographic
	// signatures of Commit against Validators.
	if err := sfc.ValidateBasic(ic.chainID); err != nil {
		return FullCommit{}, err
	}

	// If sfc.Height() != h, we can't do it.
	if sfc.Height() != h {
		return FullCommit{}, lerr.ErrCommitNotFound()
	}

FOR_LOOP:
	for {
		// Fetch latest full commit from trusted.
		tfc, err := ic.trusted.LatestFullCommit(ic.chainID, 1, h)
		if err != nil {
			return FullCommit{}, err
		}
		// Maybe we have nothing to do.
		if tfc.Height() == h {
			return FullCommit{}, nil
		}

		// Try to update to full commit with checks.
		err = ic.verifyAndSave(tfc, sfc)
		if err == nil {
			// All good!
			return sfc, nil
		} else {
			// Handle special case when err is ErrTooMuchChange.
			if lerr.IsErrTooMuchChange(err) {
				// Divide and conquer.
				start, end := tfc.Height(), sfc.Height()
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
}

func (ic *InquiringCertifier) LastTrustedHeight() int64 {
	fc, err := ic.trusted.LatestFullCommit(ic.chainID, 1, 1<<63-1)
	if err != nil {
		panic("should not happen")
	}
	return fc.Height()
}
