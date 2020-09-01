package provider

import "errors"

var (
	// ErrLightBlockNotFound is returned when a provider can't find the
	// requested header.
	ErrLightBlockNotFound = errors.New("light block not found")
)
