package errors

import (
	"fmt"

	cmn "github.com/tendermint/tmlibs/common"
)

//----------------------------------------
// Error types

type errCommitNotFound struct{}

func (e errCommitNotFound) Error() string {
	return "Commit not found by provider"
}

type errUnexpectedValidators struct {
	got  []byte
	want []byte
}

func (e errUnexpectedValidators) Error() string {
	return fmt.Sprintf("Validator set is different. Got %X want %X",
		e.got, e.want)
}

type errTooMuchChange struct{}

func (e errTooMuchChange) Error() string {
	return "Insufficient signatures to validate due to valset changes"
}

type errMissingValidators struct {
	chainID string
	height  int64
}

func (e errMissingValidators) Error() string {
	return fmt.Sprintf("Validators are unknown or missing for chain %s and height %d",
		e.chainID, e.height)
}

//----------------------------------------
// Methods for above error types

//-----------------
// ErrCommitNotFound

// ErrCommitNotFound indicates that a the requested commit was not found.
func ErrCommitNotFound() error {
	return cmn.ErrorWrap(errCommitNotFound{}, "")
}

func IsErrCommitNotFound(err error) bool {
	if err_, ok := err.(cmn.Error); ok {
		_, ok := err_.Data().(errCommitNotFound)
		return ok
	}
	return false
}

//-----------------
// ErrUnexpectedValidators

// ErrUnexpectedValidators indicates a validator set mismatch.
func ErrUnexpectedValidators(got, want []byte) error {
	return cmn.ErrorWrap(errUnexpectedValidators{
		got:  got,
		want: want,
	}, "")
}

func IsErrUnexpectedValidators(err error) bool {
	if err_, ok := err.(cmn.Error); ok {
		_, ok := err_.Data().(errUnexpectedValidators)
		return ok
	}
	return false
}

//-----------------
// ErrTooMuchChange

// ErrTooMuchChange indicates that the underlying validator set was changed by >1/3.
func ErrTooMuchChange() error {
	return cmn.ErrorWrap(errTooMuchChange{}, "")
}

func IsErrTooMuchChange(err error) bool {
	if err_, ok := err.(cmn.Error); ok {
		_, ok := err_.Data().(errTooMuchChange)
		return ok
	}
	return false
}

//-----------------
// ErrMissingValidators

// ErrMissingValidators indicates that some validator set was missing or unknown.
func ErrMissingValidators(chainID string, height int64) error {
	return cmn.ErrorWrap(errMissingValidators{chainID, height}, "")
}

func IsErrMissingValidators(err error) bool {
	if err_, ok := err.(cmn.Error); ok {
		_, ok := err_.Data().(errMissingValidators)
		return ok
	}
	return false
}
