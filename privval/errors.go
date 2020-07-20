package privval

import (
	"errors"
	"fmt"
)

type EndpointTimeoutError struct{}

// Implement the net.Error interface.
func (e EndpointTimeoutError) Error() string   { return "endpoint connection timed out" }
func (e EndpointTimeoutError) Timeout() bool   { return true }
func (e EndpointTimeoutError) Temporary() bool { return true }

// Socket errors.
var (
	ErrUnexpectedResponse = errors.New("received unexpected response")
	ErrNoConnection       = errors.New("endpoint is not connected")
	ErrConnectionTimeout  = EndpointTimeoutError{}

	ErrReadTimeout  = errors.New("endpoint read timed out")
	ErrWriteTimeout = errors.New("endpoint write timed out")
)

// RemoteSignerError allows (remote) validators to include meaningful error descriptions in their reply.
type RemoteSignerError struct {
	// TODO(ismail): create an enum of known errors
	Code        int
	Description string
}

func (e *RemoteSignerError) Error() string {
	return fmt.Sprintf("signerEndpoint returned error #%d: %s", e.Code, e.Description)
}
