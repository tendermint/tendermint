package errors

import (
	"fmt"

	cmn "github.com/tendermint/tendermint/libs/common"
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
// ErrUnknownValidators

// ErrUnknownValidators indicates that some validator set was missing or unknown.
func ErrUnknownValidators(chainID string, height int64) error {
	return cmn.ErrorWrap(errUnknownValidators{chainID, height}, "")
}

func IsErrUnknownValidators(err error) bool {
	if err_, ok := err.(cmn.Error); ok {
		_, ok := err_.Data().(errUnknownValidators)
		return ok
	}
	return false
}

//-----------------
// ErrEmptyTree

func ErrEmptyTree() error {
	return cmn.ErrorWrap(errEmptyTree{}, "")
}

func IsErrEmptyTree(err error) bool {
	if err_, ok := err.(cmn.Error); ok {
		_, ok := err_.Data().(errEmptyTree)
		return ok
	}
	return false
}
