package provider

import "errors"

var (
	// ErrSignedHeaderNotFound is returned when a provider can't find the
	// requested header.
	ErrSignedHeaderNotFound = errors.New("signed header not found")
	// ErrValidatorSetNotFound is returned when a provider can't find the
	// requested validator set.
	ErrValidatorSetNotFound = errors.New("validator set not found")
	// ErrNoResponse is returned if the provider doesn't respond to the
	// request in a gieven time
	ErrNoResponse = errors.New("client failed to respond")
)

// ErrBadSignedHeader is returned when a provider returns an invalid
// signed header.
type ErrBadSignedHeader struct {
	Reason error
}

func (e ErrBadSignedHeader) Error() string {
	return fmt.Sprintf("client provided bad signed header: %w", e.Reason)
}

// ErrBadValidatorSet is returned when a provider returns an invalid
// validator set
type ErrBadValidatorSet struct {
	Reason error
}

func (e ErrBadValidatorSet) Error() string {
	return fmt.Sprintf("client provided bad validator set: %w", e.Reason)
}