package lite

import (
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
	trusted Provider
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
// store the SignedHeader in ic.trusted.
func (ic *InquiringCertifier) Certify(shdr types.SignedHeader) error {

	// Get the latest known full commit <= h-1 from our trusted providers.
	// The full commit at h-1 contains the valset to sign for h.
	h := shdr.Height() - 1
	fc, err := ic.trusted.LatestFullCommit(ic.chainID, 1, h)
	if err != nil {
		return nil, err
	}

	if fc.Height() == h {
		// Return error if valset doesn't match.
		if !bytes.Equal(
			fc.NextValidators.Hash(),
			shdr.Header.ValidatorsHash) {
			return ErrValidatorsDifferent(
				fc.NextValidators.Hash(),
				shdr.Header.ValidatorsHash)
		}
	} else {
		// If valset doesn't match...
		if !bytes.Equal(fc.NextValidators.Hash(),
			shdr.Header.ValidatorsHash) {
			// ... update.
			fc, err = ic.updateToHeight(h)
			if err != nil {
				return err
			}
			// Return error if valset _still_ doesn't match.
			if !bytes.Equal(fc.NextValidators.Hash(),
				shdr.Header.ValidatorsHash) {
				return ErrValidatorsDifferent(
					fc.NextValidators.Hash(),
					shdr.Header.ValidatorsHash)
			}
		}
	}

	// Certify the signed header using the matching valset.
	cert := NewBaseCertifier(ic.chainID, fc.NextValidators, fc.Height())
	err = cert.Certify(shdr)
	if err != nil {
		return err
	}

	// Construct (fill) and save the new full commit.
	fc2, err := ic.source.FillFullCommit(shdr)
	if err != nil {
		return err
	}
	return ic.trusted.SaveFullCommit(fc2)
}

// verifyAndSave will verify if this is a valid source full commit given the
// best match trusted full commit, and if good, persist to ic.trusted.
// Returns ErrTooMuchChange when >2/3 of tfc did not sign sfc.
func (ic *InquiringCertifier) verifyAndSave(tfc, sfc FullCommit) error {
	err = tfc.NextValidators.VerifyCommitAny(
		ic.chainID, sfc.Commit.BlockID,
		sfc.Header.Height, sfc.Header.Commit,
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
		if err != nil {
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

func (ic *InquiringCertifier) LastTrustedHeight() (int64, error) {
	fc, err := ic.trusted.LatestFullCommit(ic.chainID, 1, 1<<63-1)
	if err != nil {
		return 0, err
	}
	return fc.Height(), nil
}
