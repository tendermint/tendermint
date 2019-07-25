package privval

import (
	"fmt"
)

type ListenerTimeoutError struct{}

// Implement the net.Error interface.
func (e ListenerTimeoutError) Error() string   { return "listening endpoint timed out" }
func (e ListenerTimeoutError) Timeout() bool   { return true }
func (e ListenerTimeoutError) Temporary() bool { return true }

// Socket errors.
var (
	ErrUnexpectedResponse   = fmt.Errorf("received unexpected response")
	ErrListenerTimeout      = ListenerTimeoutError{}
	ErrListenerNoConnection = fmt.Errorf("signer listening endpoint is not connected")
	ErrDialerTimeout        = fmt.Errorf("signer dialer endpoint timed out")

	ErrDialerReadTimeout  = fmt.Errorf("signer dialer endpoint read timed out")
	ErrDialerWriteTimeout = fmt.Errorf("signer dialer endpoint write timed out")
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
