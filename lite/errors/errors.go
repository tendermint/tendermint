package errors

import (
	"fmt"

	"github.com/pkg/errors"
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

type errUnknownValidators struct {
	chainID string
	height  int64
}

func (e errUnknownValidators) Error() string {
	return fmt.Sprintf("Validators are unknown or missing for chain %s and height %d",
		e.chainID, e.height)
}

type errEmptyTree struct{}

func (e errEmptyTree) Error() string {
	return "Tree is empty"
}

//----------------------------------------
// Methods for above error types

//-----------------
// ErrCommitNotFound

// ErrCommitNotFound indicates that a the requested commit was not found.
func ErrCommitNotFound() error {
	return errors.Wrap(errCommitNotFound{}, "")
}

func IsErrCommitNotFound(err error) bool {
	_, ok := errors.Cause(err).(errCommitNotFound)
	return ok
}

//-----------------
// ErrUnexpectedValidators

// ErrUnexpectedValidators indicates a validator set mismatch.
func ErrUnexpectedValidators(got, want []byte) error {
	return errors.Wrap(errUnexpectedValidators{
		got:  got,
		want: want,
	}, "")
}

func IsErrUnexpectedValidators(err error) bool {
	_, ok := errors.Cause(err).(errUnexpectedValidators)
	return ok
}

//-----------------
// ErrUnknownValidators

// ErrUnknownValidators indicates that some validator set was missing or unknown.
func ErrUnknownValidators(chainID string, height int64) error {
	return errors.Wrap(errUnknownValidators{chainID, height}, "")
}

func IsErrUnknownValidators(err error) bool {
	_, ok := errors.Cause(err).(errUnknownValidators)
	return ok
}

//-----------------
// ErrEmptyTree

func ErrEmptyTree() error {
	return errors.Wrap(errEmptyTree{}, "")
}

func IsErrEmptyTree(err error) bool {
	_, ok := errors.Cause(err).(errEmptyTree)
	return ok
}
