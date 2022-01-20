package provider

import (
	"errors"
	"fmt"
)

var (
	// ErrHeightTooHigh is returned when the height is higher than the last
	// block that the provider has. The light client will not remove the provider
	ErrHeightTooHigh = errors.New("height requested is too high")
	// ErrLightBlockNotFound is returned when a provider can't find the
	// requested header (i.e. it has been pruned).
	// The light client will not remove the provider
	ErrLightBlockNotFound = errors.New("light block not found")
	// ErrNoResponse is returned if the provider doesn't respond to the
	// request in a given time. The light client will not remove the provider
	ErrNoResponse = errors.New("client failed to respond")
	// ErrConnectionClosed is returned if the provider closes the connection.
	// In this case we remove the provider.
	ErrConnectionClosed = errors.New("client closed connection")
)

// ErrBadLightBlock is returned when a provider returns an invalid
// light block. The light client will remove the provider.
type ErrBadLightBlock struct {
	Reason error
}

func (e ErrBadLightBlock) Error() string {
	return fmt.Sprintf("client provided bad signed header: %v", e.Reason)
}

func (e ErrBadLightBlock) Unwrap() error { return e.Reason }

// ErrUnreliableProvider is a generic error that indicates that the provider isn't
// behaving in a reliable manner to the light client. The light client will
// remove the provider
type ErrUnreliableProvider struct {
	Reason error
}

func (e ErrUnreliableProvider) Error() string {
	return fmt.Sprintf("client deemed unreliable: %v", e.Reason)
}

func (e ErrUnreliableProvider) Unwrap() error { return e.Reason }
