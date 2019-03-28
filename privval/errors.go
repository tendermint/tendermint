package privval

import (
	"fmt"
)

// Socket errors.
var (
	ErrUnexpectedResponse = fmt.Errorf("received unexpected response")
	ErrConnTimeout        = fmt.Errorf("remote signer timed out")
)

// RemoteSignerError allows (remote) validators to include meaningful error descriptions in their reply.
type RemoteSignerError struct {
	// TODO(ismail): create an enum of known errors
	Code        int
	Description string
}

func (e *RemoteSignerError) Error() string {
	return fmt.Sprintf("signerServiceEndpoint returned error #%d: %s", e.Code, e.Description)
}
