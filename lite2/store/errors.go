package store

import "errors"

var (
	// ErrSignedHeaderNotFound is returned when a store does not have the
	// requested header.
	ErrSignedHeaderNotFound = errors.New("signed header not found")

	// ErrValidatorSetNotFound is returned when a store does not have the
	// requested validator set.
	ErrValidatorSetNotFound = errors.New("validator set not found")
)
