package privval

import (
	"errors"
	"net"
	"time"

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

// TCPVal implements PrivValidator, it uses a socket to request signatures
// from an external process.
type TCPVal struct {
	cmn.BaseService
	*RemoteSignerClient

	addr           string
	acceptDeadline time.Duration
	connTimeout    time.Duration
	connHeartbeat  time.Duration
	privKey        ed25519.PrivKeyEd25519

	conn       net.Conn
	listener   net.Listener
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
		privKey:        privKey,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "TCPVal", sc)

	return sc
}

// OnStart implements cmn.Service.
func (sc *TCPVal) OnStart() error {
	if err := sc.listen(); err != nil {
		sc.Logger.Error("OnStart", "err", err)
		return err
	}

	conn, err := sc.waitConnection()
	if err != nil {
		sc.Logger.Error("OnStart", "err", err)
		return err
	}

	sc.conn = conn

	sc.RemoteSignerClient = NewRemoteSignerClient(sc.conn)

	// Start a routine to keep the connection alive
	sc.cancelPing = make(chan struct{}, 1)
	sc.pingTicker = time.NewTicker(sc.connHeartbeat)
	go func() {
		for {
			select {
			case <-sc.pingTicker.C:
				err := sc.Ping()
				if err != nil {
					sc.Logger.Error(
						"Ping",
						"err", err,
					)
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

	if sc.conn != nil {
		if err := sc.conn.Close(); err != nil {
			sc.Logger.Error("OnStop", "err", err)
		}
	}

	if sc.listener != nil {
		if err := sc.listener.Close(); err != nil {
			sc.Logger.Error("OnStop", "err", err)
		}
	}
}

func (sc *TCPVal) acceptConnection() (net.Conn, error) {
	conn, err := sc.listener.Accept()
	if err != nil {
		if !sc.IsRunning() {
			return nil, nil // Ignore error from listener closing.
		}
		return nil, err

	}

	conn, err = p2pconn.MakeSecretConnection(conn, sc.privKey)
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
