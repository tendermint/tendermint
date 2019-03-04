package privval

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

const (
	defaultHeartbeatSeconds = 2
	defaultMaxDialRetries   = 10
)

var (
	heartbeatPeriod = time.Second * defaultHeartbeatSeconds
)

// SignerValidatorEndpointOption sets an optional parameter on the SocketVal.
type SignerValidatorEndpointOption func(*SignerValidatorEndpoint)

// SignerValidatorEndpointSetHeartbeat sets the period on which to check the liveness of the
// connected Signer connections.
func SignerValidatorEndpointSetHeartbeat(period time.Duration) SignerValidatorEndpointOption {
	return func(sc *SignerValidatorEndpoint) { sc.heartbeatPeriod = period }
}

// SocketVal implements PrivValidator.
// It listens for an external process to dial in and uses
// the socket to request signatures.
type SignerValidatorEndpoint struct {
	cmn.BaseService

	listener net.Listener

	// ping
	cancelPingCh    chan struct{}
	pingTicker      *time.Ticker
	heartbeatPeriod time.Duration

	// signer is mutable since it can be reset if the connection fails.
	// failures are detected by a background ping routine.
	// All messages are request/response, so we hold the mutex
	// so only one request/response pair can happen at a time.
	// Methods on the underlying net.Conn itself are already goroutine safe.
	mtx sync.Mutex

	// TODO: Signer should encapsulate and hide the endpoint completely. Invert the relation
	signer *SignerRemote
}

// Check that SignerValidatorEndpoint implements PrivValidator.
var _ types.PrivValidator = (*SignerValidatorEndpoint)(nil)

// NewSignerValidatorEndpoint returns an instance of SignerValidatorEndpoint.
func NewSignerValidatorEndpoint(logger log.Logger, listener net.Listener) *SignerValidatorEndpoint {
	sc := &SignerValidatorEndpoint{
		listener:        listener,
		heartbeatPeriod: heartbeatPeriod,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "SignerValidatorEndpoint", sc)

	return sc
}

//--------------------------------------------------------
// Implement PrivValidator

// GetPubKey implements PrivValidator.
func (ve *SignerValidatorEndpoint) GetPubKey() crypto.PubKey {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()
	return ve.signer.GetPubKey()
}

// SignVote implements PrivValidator.
func (ve *SignerValidatorEndpoint) SignVote(chainID string, vote *types.Vote) error {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()
	return ve.signer.SignVote(chainID, vote)
}

// SignProposal implements PrivValidator.
func (ve *SignerValidatorEndpoint) SignProposal(chainID string, proposal *types.Proposal) error {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()
	return ve.signer.SignProposal(chainID, proposal)
}

//--------------------------------------------------------
// More thread safe methods proxied to the signer

// Ping is used to check connection health.
func (ve *SignerValidatorEndpoint) Ping() error {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()
	return ve.signer.Ping()
}

// Close closes the underlying net.Conn.
func (ve *SignerValidatorEndpoint) Close() {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()
	if ve.signer != nil {
		if err := ve.signer.Close(); err != nil {
			ve.Logger.Error("OnStop", "err", err)
		}
	}

	if ve.listener != nil {
		if err := ve.listener.Close(); err != nil {
			ve.Logger.Error("OnStop", "err", err)
		}
	}
}

//--------------------------------------------------------
// Service start and stop

// OnStart implements cmn.Service.
func (ve *SignerValidatorEndpoint) OnStart() error {
	if closed, err := ve.reset(); err != nil {
		ve.Logger.Error("OnStart", "err", err)
		return err
	} else if closed {
		return fmt.Errorf("listener is closed")
	}

	// Start a routine to keep the connection alive
	ve.cancelPingCh = make(chan struct{}, 1)
	ve.pingTicker = time.NewTicker(ve.heartbeatPeriod)
	go func() {
		for {
			select {
			case <-ve.pingTicker.C:
				err := ve.Ping()
				if err != nil {
					ve.Logger.Error("Ping", "err", err)
					if err == ErrUnexpectedResponse {
						return
					}

					closed, err := ve.reset()
					if err != nil {
						ve.Logger.Error("Reconnecting to remote signer failed", "err", err)
						continue
					}
					if closed {
						ve.Logger.Info("listener is closing")
						return
					}

					ve.Logger.Info("Re-created connection to remote signer", "impl", ve)
				}
			case <-ve.cancelPingCh:
				ve.pingTicker.Stop()
				return
			}
		}
	}()

	return nil
}

// OnStop implements cmn.Service.
func (ve *SignerValidatorEndpoint) OnStop() {
	if ve.cancelPingCh != nil {
		close(ve.cancelPingCh)
	}
	ve.Close()
}

//--------------------------------------------------------
// Connection and signer management

// waits to accept and sets a new connection.
// connection is closed in OnStop.
// returns true if the listener is closed
// (ie. it returns a nil conn).
func (ve *SignerValidatorEndpoint) reset() (closed bool, err error) {
	ve.mtx.Lock()
	defer ve.mtx.Unlock()

	// first check if the conn already exists and close it.
	if ve.signer != nil {
		if tmpErr := ve.signer.Close(); tmpErr != nil {
			ve.Logger.Error("error closing socket val connection during reset", "err", tmpErr)
		}
	}

	// wait for a new conn
	conn, err := ve.acceptConnection()
	if err != nil {
		return false, err
	}

	// listener is closed
	if conn == nil {
		return true, nil
	}

	ve.signer, err = NewSignerRemote(conn)
	if err != nil {
		// failed to fetch the pubkey. close out the connection.
		if tmpErr := conn.Close(); tmpErr != nil {
			ve.Logger.Error("error closing connection", "err", tmpErr)
		}
		return false, err
	}
	return false, nil
}

// Attempt to accept a connection.
// Times out after the listener's timeoutAccept
func (ve *SignerValidatorEndpoint) acceptConnection() (net.Conn, error) {
	conn, err := ve.listener.Accept()
	if err != nil {
		if !ve.IsRunning() {
			return nil, nil // Ignore error from listener closing.
		}
		return nil, err
	}
	return conn, nil
}
