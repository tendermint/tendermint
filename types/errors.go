package types

import (
	"fmt"

	"github.com/pkg/errors"
)

type (
	// ErrInvalidCommitHeight is returned when we encounter a commit with an
	// unexpected height.
	ErrInvalidCommitHeight struct {
		Expected int64
		Actual   int64
	}

	// ErrInvalidCommitSignatures is returned when we encounter a commit where
	// the number of signatures doesn't match the number of validators.
	ErrInvalidCommitSignatures struct {
		Expected int
		Actual   int
	}
)

func NewErrInvalidCommitHeight(expected, actual int64) ErrInvalidCommitHeight {
	return ErrInvalidCommitHeight{
		Expected: expected,
		Actual:   actual,
	}
}

func (e ErrInvalidCommitHeight) Error() string {
	return fmt.Sprintf("Invalid commit -- wrong height: %v vs %v", e.Expected, e.Actual)
}

func NewErrInvalidCommitSignatures(expected, actual int) ErrInvalidCommitSignatures {
	return ErrInvalidCommitSignatures{
		Expected: expected,
		Actual:   actual,
	}
}

func (e ErrInvalidCommitSignatures) Error() string {
	return fmt.Sprintf("Invalid commit -- wrong set size: %v vs %v", e.Expected, e.Actual)
}

// ErrNotEnoughVotingPowerSigned is returned when not enough validators signed
// a commit.
type ErrNotEnoughVotingPowerSigned struct {
	Got    int64
	Needed int64
}

// IsErrNotEnoughVotingPowerSigned returns true if err is
// ErrNotEnoughVotingPowerSigned.
func IsErrNotEnoughVotingPowerSigned(err error) bool {
	_, ok := errors.Cause(err).(ErrNotEnoughVotingPowerSigned)
	return ok
}

func (e ErrNotEnoughVotingPowerSigned) Error() string {
	return fmt.Sprintf("invalid commit -- insufficient voting power: got %d, needed more than %d", e.Got, e.Needed)
}
