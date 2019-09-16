package lite

import (
	"bytes"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
)

// TrustedState stores the latest state trusted by a lite client, including the
// last header and the validator set to use to verify the next header.
type TrustedState struct {
	LastHeader *types.Header       // height H-1
	Validators *types.ValidatorSet // height H
}

// Verifier is the core of the light client performing validation of the
// headers.
type Verifier struct {
	chainID        string
	trustingPeriod time.Duration
	trustLevel     float

	state TrustedState
}

func NewVerifier(chainID string,
	trustingPeriod time.Duration,
	trustLevel float,
	trustedState *TrustedState) *Verifier {

	return &Verifier{
		chainID:        chainID,
		trustingPeriod: trustingPeriod,
		trustLevel:     trustLevel,
		TrustedState:   trustedState,
	}
}

func (v *Verifier) Verify(newHeader *types.SignedHeader, vals *types.ValidatorSet, now time.Time) error {
	if err := v.expired(); err != nil {
		return err
	}

	if err := v.verifyNewHeaderAndVals(newHeader, vals); err != nil {
		return err
	}

	if newHeader.Height == v.state.LastHeader.Height+1 {
		if !bytes.Equal(newHeader.ValidatorsHash, v.state.Validators.Hash()) {
			return fmt.Errorf("expected our validators (%X) to match those from new header (%X)",
				v.state.Validators.Hash(),
				newHeader.ValidatorsHash,
			)
		}

		// Ensure that +2/3 of current validators signed correctly.
		err = vals.VerifyCommit(v.chainID, newHeader.Commit.BlockID, newHeader.Height, newHeader.Commit)
		if err != nil {
			return err
		}
	} else {
		// Ensure that +1/3 of last trusted validators signed correctly.
		err = v.state.Validators.VerifyCommitTrusting(v.chainID, newHeader.Commit.BlockID,
			newHeader.Height, newHeader.Commit, v.trustLevel)
		if err != nil {
			return err
		}

		// Ensure that +2/3 of current validators signed correctly.
		err = vals.VerifyCommit(v.chainID, newHeader.Commit.BlockID, newHeader.Height,
			newHeader.Commit)
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *Verifier) expired() error {
	expired := v.state.LastHeader.Time.Add(v.trustingPeriod)
	if expired.Before(now) {
		return errors.Errorf("last header expired at %v and too old to be trusted now %v. Verifier must be reset subjectively", expired, now)
	}
	return nil
}

func verifyNewHeaderAndVals(newHeader *types.SignedHeader, vals *types.ValidatorSet) error {
	if err := newHeader.ValidateBasic(v.chainID); err != nil {
		return errors.Wrap(err, "newHeader.ValidateBasic failed")
	}

	if !bytes.Equal(newHeader.ValidatorsHash, vals.Hash()) {
		return fmt.Errorf("expected validators (%X) to match those from new header (%X)",
			vals.Hash(),
			newHeader.NextValidatorsHash,
		)
	}
}
