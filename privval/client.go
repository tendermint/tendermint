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
var (
	ErrUnexpectedResponse = errors.New("received unexpected response")
)

var (
	connHeartbeat = time.Second * defaultConnHeartBeatSeconds
)

// KMSListenerOption sets an optional parameter on the SocketVal.
type KMSListenerOption func(*KMSListener)

// SocketValHeartbeat sets the period on which to check the liveness of the
// connected Signer connections.
func SocketValHeartbeat(period time.Duration) KMSListenerOption {
	return func(sc *KMSListener) { sc.connHeartbeat = period }
}

// SocketVal implements PrivValidator.
// It listens for an external process to dial in and uses
// the socket to request signatures.
type KMSListener struct {
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
	// All messages are request/response, so we hold the mutex
	// so only one request/response pair can happen at a time.
	// Methods on the underlying net.Conn itself are already goroutine safe.
	mtx    sync.Mutex
	signer *RemoteSignerClient
}

// Check that KMSListener implements PrivValidator.
var _ types.PrivValidator = (*KMSListener)(nil)

// NewKMSListener returns an instance of KMSListener.
func NewKMSListener(
	logger log.Logger,
	listener net.Listener,
) *KMSListener {
	sc := &KMSListener{
		listener:      listener,
		connHeartbeat: connHeartbeat,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "KMSListener", sc)

	return sc
}

//--------------------------------------------------------
// Implement PrivValidator

// GetPubKey implements PrivValidator.
func (sc *KMSListener) GetPubKey() crypto.PubKey {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	return sc.signer.GetPubKey()
}

// SignVote implements PrivValidator.
func (sc *KMSListener) SignVote(chainID string, vote *types.Vote) error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	return sc.signer.SignVote(chainID, vote)
}

// SignProposal implements PrivValidator.
func (sc *KMSListener) SignProposal(chainID string, proposal *types.Proposal) error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	return sc.signer.SignProposal(chainID, proposal)
}

//--------------------------------------------------------
// More thread safe methods proxied to the signer

// Ping is used to check connection health.
func (sc *KMSListener) Ping() error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	return sc.signer.Ping()
}

// Close closes the underlying net.Conn.
func (sc *KMSListener) Close() {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
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
func (sc *KMSListener) OnStart() error {
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
func (sc *KMSListener) OnStop() {
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
// (ie. it returns a nil conn).
func (sc *KMSListener) reset() (closed bool, err error) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	// first check if the conn already exists and close it.
	if sc.signer != nil {
		if err := sc.signer.Close(); err != nil {
			sc.Logger.Error("error closing socket val connection during reset", "err", err)
		}
	}

	// wait for a new conn
	conn, err := sc.acceptConnection()
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

// Attempt to accept a connection.
// Times out after the listener's timeoutAccept
func (sc *KMSListener) acceptConnection() (net.Conn, error) {
	conn, err := sc.listener.Accept()
	if err != nil {
		if !sc.IsRunning() {
			return nil, nil // Ignore error from listener closing.
		}
		return nil, err
	}
	return conn, nil
}
