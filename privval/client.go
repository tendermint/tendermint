package privval

import (
	"errors"
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
	defaultConnHeartBeatSeconds = 2
	defaultDialRetries          = 10
)

// Socket errors.
// XXX these arent all used in this file
var (
	ErrDialRetryMax       = errors.New("dialed maximum retries")
	ErrConnTimeout        = errors.New("remote signer timed out")
	ErrUnexpectedResponse = errors.New("received unexpected response")
)

var (
	connHeartbeat = time.Second * defaultConnHeartBeatSeconds
)

// SocketValOption sets an optional parameter on the SocketPV.
type SocketValOption func(*SocketVal)

// SocketValHeartbeat sets the period on which to check the liveness of the
// connected Signer connections.
func SocketValHeartbeat(period time.Duration) SocketValOption {
	return func(sc *SocketVal) { sc.connHeartbeat = period }
}

// SocketVal implements PrivValidator.
// It listens for an external process to dial in and uses
// the (encrypted) socket to request signatures.
type SocketVal struct {
	cmn.BaseService

	listener net.Listener

	// ping
	cancelPing    chan struct{}
	pingTicker    *time.Ticker
	connHeartbeat time.Duration

	// signer is mutable since it can be
	// reset if the connection fails.
	// failures are detected by a background
	// ping routine.
	// Methods on the underlying net.Conn itself
	// are already gorountine safe.
	mtx    sync.RWMutex
	signer *RemoteSignerClient
}

// Check that SocketVal implements PrivValidator.
var _ types.PrivValidator = (*SocketVal)(nil)

// NewSocketVal returns an instance of SocketVal.
func NewSocketVal(
	logger log.Logger,
	listener net.Listener,
) *SocketVal {
	sc := &SocketVal{
		listener:      listener,
		connHeartbeat: connHeartbeat,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "SocketVal", sc)

	return sc
}

//--------------------------------------------------------
// Implement PrivValidator

// GetPubKey implements PrivValidator.
func (sc *SocketVal) GetPubKey() crypto.PubKey {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return sc.signer.GetPubKey()
}

// SignVote implements PrivValidator.
func (sc *SocketVal) SignVote(chainID string, vote *types.Vote) error {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return sc.signer.SignVote(chainID, vote)
}

// SignProposal implements PrivValidator.
func (sc *SocketVal) SignProposal(chainID string, proposal *types.Proposal) error {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return sc.signer.SignProposal(chainID, proposal)
}

//--------------------------------------------------------
// More thread safe methods proxied to the signer

// Ping is used to check connection health.
func (sc *SocketVal) Ping() error {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return sc.signer.Ping()
}

// Close closes the underlying net.Conn.
func (sc *SocketVal) Close() {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	if sc.signer != nil {
		if err := sc.signer.Close(); err != nil {
			sc.Logger.Error("OnStop", "err", err)
		}
	}

	if sc.listener != nil {
		if err := sc.listener.Close(); err != nil {
			sc.Logger.Error("OnStop", "err", err)
		}
	}
}

//--------------------------------------------------------
// Service start and stop

// OnStart implements cmn.Service.
func (sc *SocketVal) OnStart() error {
	if closed, err := sc.reset(); err != nil {
		sc.Logger.Error("OnStart", "err", err)
		return err
	} else if closed {
		return fmt.Errorf("listener is closed")
	}

	// Start a routine to keep the connection alive
	sc.cancelPing = make(chan struct{}, 1)
	sc.pingTicker = time.NewTicker(sc.connHeartbeat)
	go func() {
		for {
			select {
			case <-sc.pingTicker.C:
				err := sc.Ping()
				if err != nil {
					sc.Logger.Error("Ping", "err", err)
					if err == ErrUnexpectedResponse {
						return
					}

					closed, err := sc.reset()
					if err != nil {
						sc.Logger.Error("Reconnecting to remote signer failed", "err", err)
						continue
					}
					if closed {
						sc.Logger.Info("listener is closing")
						return
					}

					sc.Logger.Info("Re-created connection to remote signer", "impl", sc)
				}
			case <-sc.cancelPing:
				sc.pingTicker.Stop()
				return
			}
		}
	}()

	return nil
}

// OnStop implements cmn.Service.
func (sc *SocketVal) OnStop() {
	if sc.cancelPing != nil {
		close(sc.cancelPing)
	}
	sc.Close()
}

//--------------------------------------------------------
// Connection and signer management

// waits to accept and sets a new connection.
// connection is closed in OnStop.
// returns true if the listener is closed
// (ie. it returns a nil conn)
func (sc *SocketVal) reset() (bool, error) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	// first check if the conn already exists and close it.
	if sc.signer != nil {
		if err := sc.signer.Close(); err != nil {
			sc.Logger.Error("error closing connection", "err", err)
		}
	}

	// wait for a new conn
	conn, err := sc.waitConnection()
	if err != nil {
		return false, err
	}

	// listener is closed
	if conn == nil {
		return true, nil
	}

	sc.signer, err = NewRemoteSignerClient(conn)
	if err != nil {
		// failed to fetch the pubkey. close out the connection.
		if err := conn.Close(); err != nil {
			sc.Logger.Error("error closing connection", "err", err)
		}
		return false, err
	}
	return false, nil
}

func (sc *SocketVal) acceptConnection() (net.Conn, error) {
	conn, err := sc.listener.Accept()
	if err != nil {
		if !sc.IsRunning() {
			return nil, nil // Ignore error from listener closing.
		}
		return nil, err

	}
	return conn, nil
}

// waitConnection uses the configured wait timeout to error if no external
// process connects in the time period.
func (sc *SocketVal) waitConnection() (net.Conn, error) {
	var (
		connc = make(chan net.Conn, 1)
		errc  = make(chan error, 1)
	)

	go func(connc chan<- net.Conn, errc chan<- error) {
		conn, err := sc.acceptConnection()
		if err != nil {
			errc <- err
			return
		}

		connc <- conn
	}(connc, errc)

	select {
	case conn := <-connc:
		return conn, nil
	case err := <-errc:
		return nil, err
	}
}
