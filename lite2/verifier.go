package lite

import (
	"bytes"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
)

const (
	defaultTrustLevel = 1 / 3
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
	chainID string

	trustingPeriod time.Duration
	trustLevel     float
	state          TrustedState
}

// TrustLevel can be used to change the default trust level (1/3) if the user
// believes that relying on one correct validator is not sufficient.
//
// However, in case of (frequent) changes in the validator set, the higher the
// trustlevel is chosen, the more unlikely it becomes that Verify returns true
// for a non-adjacent header.
func TrustLevel(lvl float) func(*Verifier) {
	return func(v *Verifier) {
		v.trustLevel = lvl
	}
}

func NewVerifier(chainID string,
	trustingPeriod time.Duration,
	trustedState *TrustedState,
	options ...func(*Verifier)) *Verifier {

	v := &Verifier{
		chainID: chainID,

		trustingPeriod: trustingPeriod,
		trustLevel:     defaultTrustLevel,
		state:          trustedState,
	}

	for _, o := range options {
		o(v)
	}

	return v
}

func (v *Verifier) Verify(newHeader *types.SignedHeader, vals *types.ValidatorSet, now time.Time) error {
	if err := v.expired(now); err != nil {
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

func (v *Verifier) expired(now time.Time) error {
	expired := v.state.LastHeader.Time.Add(v.trustingPeriod)
	if expired.Before(now) {
		return errors.Errorf("last header expired at %v and too old to be trusted now %v. Verifier must be reset subjectively", expired, now)
	}
	return nil
}

func (v *Verifier) verifyNewHeaderAndVals(newHeader *types.SignedHeader, vals *types.ValidatorSet) error {
	if err := newHeader.ValidateBasic(v.chainID); err != nil {
		return errors.Wrap(err, "newHeader.ValidateBasic failed")
	}

	if newHeader.Height <= v.state.LastHeader.Height {
		return fmt.Errorf("expected new header height %d to be greater than one of last header %d",
			newHeader.Height,
			v.state.LastHeader.Height)
	}

	if newHeader.Time.Before(v.state.LastHeader.Time) || newHeader.Time == v.state.LastHeader.Time {
		return fmt.Errorf("expected new header time %v to be after last header time %v",
			newHeader.Time,
			v.state.LastHeader.Time)
	}

	if !bytes.Equal(newHeader.ValidatorsHash, vals.Hash()) {
		return fmt.Errorf("expected validators (%X) to match those from new header (%X)",
			vals.Hash(),
			newHeader.NextValidatorsHash,
		)
	}
}
