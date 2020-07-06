package provider

import "errors"

var (
	// ErrSignedHeaderNotFound is returned when a provider can't find the
	// requested header.
	ErrSignedHeaderNotFound = errors.New("signed header not found")
	// ErrValidatorSetNotFound is returned when a provider can't find the
	// requested validator set.
	ErrValidatorSetNotFound = errors.New("validator set not found")
)
