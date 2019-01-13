package privval

import (
	"net"
	"sync"
	"time"

	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// IPCValOption sets an optional parameter on the SocketPV.
type IPCValOption func(*IPCVal)

// IPCValConnTimeout sets the read and write timeout for connections
// from external signing processes.
func IPCValConnTimeout(timeout time.Duration) IPCValOption {
	return func(sc *IPCVal) { sc.connTimeout = timeout }
}

// IPCValHeartbeat sets the period on which to check the liveness of the
// connected Signer connections.
func IPCValHeartbeat(period time.Duration) IPCValOption {
	return func(sc *IPCVal) { sc.connHeartbeat = period }
}

// IPCVal implements PrivValidator.
// It dials an external process and uses the unencrypted socket
// to request signatures.
type IPCVal struct {
	cmn.BaseService

	addr string

	connTimeout   time.Duration
	connHeartbeat time.Duration

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

// Check that IPCVal implements PrivValidator.
var _ types.PrivValidator = (*IPCVal)(nil)

// NewIPCVal returns an instance of IPCVal.
// It provides thread-safe access to an underlying
// RemoteSignerClient, resetting it as necessary when
// connections fail.
func NewIPCVal(
	logger log.Logger,
	socketAddr string,
) *IPCVal {
	sc := &IPCVal{
		addr:          socketAddr,
		connTimeout:   connTimeout,
		connHeartbeat: connHeartbeat,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "IPCVal", sc)

	return sc
}

// GetPubKey implements PrivValidator.
func (sc *IPCVal) GetPubKey() crypto.PubKey {
	return sc.signer.GetPubKey()
}

// SignVote implements PrivValidator.
func (sc *IPCVal) SignVote(chainID string, vote *types.Vote) error {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return sc.signer.SignVote(chainID, vote)
}

// SignProposal implements PrivValidator.
func (sc *IPCVal) SignProposal(chainID string, proposal *types.Proposal) error {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return sc.signer.SignProposal(chainID, proposal)
}

// Ping is used to check connection health.
func (sc *IPCVal) Ping() error {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return sc.signer.Ping()
}

// Close closes the underlying net.Conn.
func (sc *IPCVal) Close() error {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return sc.signer.Close()
}

// dials and sets a new connection.
// connection is closed in OnStop.
func (sc *IPCVal) reset() error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	// first check if the conn already exists and close it.
	if sc.signer != nil {
		if err := sc.signer.Close(); err != nil {
			sc.Logger.Error("error closing connection", "err", err)
		}
	}

	// dial the addr
	conn, err := sc.connect()
	if err != nil {
		return err
	}

	sc.signer, err = NewRemoteSignerClient(conn)
	if err != nil {
		// failed to fetch the pubkey. close out the connection.
		if err := conn.Close(); err != nil {
			sc.Logger.Error("error closing connection", "err", err)
		}
		return err
	}
	return nil
}

// OnStart implements cmn.Service.
func (sc *IPCVal) OnStart() error {

	if err := sc.reset(); err != nil {
		sc.Logger.Error("OnStart", "err", err)
		return err
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

					err := sc.reset()
					if err != nil {
						sc.Logger.Error("Reconnecting to remote signer failed", "err", err)
						continue
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
// It stops the ping routine and
// closes the underlying net.Conn.
func (sc *IPCVal) OnStop() {
	if sc.cancelPing != nil {
		close(sc.cancelPing)
	}

	sc.Close()
}

// return a new connection using the address and timeout.
func (sc *IPCVal) connect() (net.Conn, error) {
	la, err := net.ResolveUnixAddr("unix", sc.addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUnix("unix", nil, la)
	if err != nil {
		return nil, err
	}

	return newTimeoutConn(conn, sc.connTimeout), nil
}
