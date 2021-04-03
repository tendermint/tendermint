package provider

import (
	"errors"
	"fmt"
)

var (
	// ErrLightBlockNotFound is returned when a provider can't find the
	// requested header. The light client will not remove the provider
	ErrLightBlockNotFound = errors.New("light block not found")
	// ErrNoResponse is returned if the provider doesn't respond to the
	// request in a given time. The light client will not remove the provider
	ErrNoResponse = errors.New("client failed to respond")
)

// ErrBadLightBlock is returned when a provider returns an invalid
// light block. The light client will remove the provider.
type ErrBadLightBlock struct {
	Reason error
}

func (e ErrBadLightBlock) Error() string {
	return fmt.Sprintf("client provided bad signed header: %s", e.Reason.Error())
}

// ErrUnreliableProvider is a generic error that indicates that the provider isn't
// behaving in a reliable manner to the light client. The light client will
// remove the provider
type ErrUnreliableProvider struct {
	Reason string
}

func (e ErrUnreliableProvider) Error() string {
	return fmt.Sprintf("client deemed unreliable: %s", e.Reason)
}
