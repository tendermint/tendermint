package privval

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	p2pconn "github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/types"
)

const (
	defaultAcceptDeadlineSeconds = 3
	defaultConnDeadlineSeconds   = 3
	defaultConnHeartBeatSeconds  = 2
	defaultDialRetries           = 10
)

// Socket errors.
var (
	ErrDialRetryMax       = errors.New("dialed maximum retries")
	ErrConnTimeout        = errors.New("remote signer timed out")
	ErrUnexpectedResponse = errors.New("received unexpected response")
)

var (
	acceptDeadline = time.Second * defaultAcceptDeadlineSeconds
	connTimeout    = time.Second * defaultConnDeadlineSeconds
	connHeartbeat  = time.Second * defaultConnHeartBeatSeconds
)

// TCPValOption sets an optional parameter on the SocketPV.
type TCPValOption func(*TCPVal)

// TCPValAcceptDeadline sets the deadline for the TCPVal listener.
// A zero time value disables the deadline.
func TCPValAcceptDeadline(deadline time.Duration) TCPValOption {
	return func(sc *TCPVal) { sc.acceptDeadline = deadline }
}

// TCPValConnTimeout sets the read and write timeout for connections
// from external signing processes.
func TCPValConnTimeout(timeout time.Duration) TCPValOption {
	return func(sc *TCPVal) { sc.connTimeout = timeout }
}

// TCPValHeartbeat sets the period on which to check the liveness of the
// connected Signer connections.
func TCPValHeartbeat(period time.Duration) TCPValOption {
	return func(sc *TCPVal) { sc.connHeartbeat = period }
}

// TCPVal implements PrivValidator.
// It listens for an external process to dial in and uses
// the (encrypted) socket to request signatures.
type TCPVal struct {
	cmn.BaseService

	addr string

	acceptDeadline time.Duration
	connTimeout    time.Duration
	connHeartbeat  time.Duration

	secretConnKey ed25519.PrivKeyEd25519
	listener      net.Listener

	// signer is mutable since it can be
	// reset if the connection fails.
	// failures are detected by a background
	// ping routine.
	// Methods on the underlying net.Conn itself
	// are already gorountine safe.
	mtx    sync.RWMutex
	signer *RemoteSignerClient

	cancelPing chan struct{}
	pingTicker *time.Ticker
}

// Check that TCPVal implements PrivValidator.
var _ types.PrivValidator = (*TCPVal)(nil)

// NewTCPVal returns an instance of TCPVal.
func NewTCPVal(
	logger log.Logger,
	socketAddr string,
	privKey ed25519.PrivKeyEd25519,
) *TCPVal {
	sc := &TCPVal{
		addr:           socketAddr,
		acceptDeadline: acceptDeadline,
		connTimeout:    connTimeout,
		connHeartbeat:  connHeartbeat,
		secretConnKey:  privKey,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "TCPVal", sc)

	return sc
}

// GetPubKey implements PrivValidator.
func (sc *TCPVal) GetPubKey() crypto.PubKey {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return sc.signer.GetPubKey()
}

// SignVote implements PrivValidator.
func (sc *TCPVal) SignVote(chainID string, vote *types.Vote) error {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return sc.signer.SignVote(chainID, vote)
}

// SignProposal implements PrivValidator.
func (sc *TCPVal) SignProposal(chainID string, proposal *types.Proposal) error {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return sc.signer.SignProposal(chainID, proposal)
}

// Ping is used to check connection health.
func (sc *TCPVal) Ping() error {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return sc.signer.Ping()
}

// Close closes the underlying net.Conn.
func (sc *TCPVal) Close() {
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

// waits to accept and sets a new connection.
// connection is closed in OnStop.
// returns true if the listener is closed
// (ie. it returns a nil conn)
func (sc *TCPVal) reset() (bool, error) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	// first check if the conn already exists and close it.
	if sc.signer != nil {
		if err := sc.signer.Close(); err != nil {
			sc.Logger.Error("error closing connection", "err", err)
		}
	}

	// dial the addr
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

// OnStart implements cmn.Service.
func (sc *TCPVal) OnStart() error {
	if err := sc.listen(); err != nil {
		sc.Logger.Error("OnStart", "err", err)
		return err
	}

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
func (sc *TCPVal) OnStop() {
	if sc.cancelPing != nil {
		close(sc.cancelPing)
	}
	sc.Close()
}

func (sc *TCPVal) acceptConnection() (net.Conn, error) {
	conn, err := sc.listener.Accept()
	if err != nil {
		if !sc.IsRunning() {
			return nil, nil // Ignore error from listener closing.
		}
		return nil, err

	}

	conn, err = p2pconn.MakeSecretConnection(conn, sc.secretConnKey)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (sc *TCPVal) listen() error {
	ln, err := net.Listen(cmn.ProtocolAndAddress(sc.addr))
	if err != nil {
		return err
	}

	sc.listener = newTCPTimeoutListener(
		ln,
		sc.acceptDeadline,
		sc.connTimeout,
		sc.connHeartbeat,
	)

	return nil
}

// waitConnection uses the configured wait timeout to error if no external
// process connects in the time period.
func (sc *TCPVal) waitConnection() (net.Conn, error) {
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
