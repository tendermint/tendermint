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

type errValidatorsDifferent struct {
	got  []byte
	want []byte
}

func (e errValidatorsDifferent) Error() string {
	return fmt.Sprintf("Validator set is different. Got %X want %X",
		e.got, e.want)
}

type errTooMuchChange struct{}

func (e errTooMuchChange) Error() string {
	return "Insufficient signatures to validate due to valset changes"
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
// ErrValidatorsDifferent

// ErrValidatorsDifferent indicates a validator set mismatch.
func ErrValidatorsDifferent(got, want []byte) error {
	return cmn.ErrorWrap(errValidatorsDifferent{
		got:  got,
		want: want,
	}, "")
}

func IsErrValidatorsDifferent(err error) bool {
	if err_, ok := err.(cmn.Error); ok {
		_, ok := err_.Data().(errValidatorsDifferent)
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
