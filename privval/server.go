package privval

import (
	"io"
	"net"
	"time"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	p2pconn "github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/types"
)

// Socket errors.
var (
	ErrDialRetryMax = errors.New("dialed maximum retries")
)

// RemoteSignerOption sets an optional parameter on the RemoteSigner.
type RemoteSignerOption func(*RemoteSigner)

// RemoteSignerConnDeadline sets the read and write deadline for connections
// from external signing processes.
func RemoteSignerConnDeadline(deadline time.Duration) RemoteSignerOption {
	return func(ss *RemoteSigner) { ss.connDeadline = deadline }
}

// RemoteSignerConnRetries sets the amount of attempted retries to connect.
func RemoteSignerConnRetries(retries int) RemoteSignerOption {
	return func(ss *RemoteSigner) { ss.connRetries = retries }
}

// RemoteSigner dials using its dialer and responds to any
// signature requests using its privVal.
type RemoteSigner struct {
	cmn.BaseService

	chainID      string
	connDeadline time.Duration
	connRetries  int
	privVal      types.PrivValidator

	dialer Dialer
	conn   net.Conn
}

// Dialer dials a remote address and returns a net.Conn or an error.
type Dialer func() (net.Conn, error)

// DialTCPFn dials the given tcp addr, using the given connTimeout and privKey for the
// authenticated encryption handshake.
func DialTCPFn(addr string, connTimeout time.Duration, privKey ed25519.PrivKeyEd25519) Dialer {
	return func() (net.Conn, error) {
		conn, err := cmn.Connect(addr)
		if err == nil {
			err = conn.SetDeadline(time.Now().Add(connTimeout))
		}
		if err == nil {
			conn, err = p2pconn.MakeSecretConnection(conn, privKey)
		}
		return conn, err
	}
}

// DialUnixFn dials the given unix socket.
func DialUnixFn(addr string) Dialer {
	return func() (net.Conn, error) {
		unixAddr := &net.UnixAddr{Name: addr, Net: "unix"}
		return net.DialUnix("unix", nil, unixAddr)
	}
}

// NewRemoteSigner return a RemoteSigner that will dial using the given
// dialer and respond to any signature requests over the connection
// using the given privVal.
func NewRemoteSigner(
	logger log.Logger,
	chainID string,
	privVal types.PrivValidator,
	dialer Dialer,
) *RemoteSigner {
	rs := &RemoteSigner{
		chainID:      chainID,
		connDeadline: time.Second * defaultConnDeadlineSeconds,
		connRetries:  defaultDialRetries,
		privVal:      privVal,
		dialer:       dialer,
	}

	rs.BaseService = *cmn.NewBaseService(logger, "RemoteSigner", rs)
	return rs
}

// OnStart implements cmn.Service.
func (rs *RemoteSigner) OnStart() error {
	conn, err := rs.connect()
	if err != nil {
		rs.Logger.Error("OnStart", "err", err)
		return err
	}
	rs.conn = conn

	go rs.handleConnection(conn)

	return nil
}

// OnStop implements cmn.Service.
func (rs *RemoteSigner) OnStop() {
	if rs.conn == nil {
		return
	}

	if err := rs.conn.Close(); err != nil {
		rs.Logger.Error("OnStop", "err", cmn.ErrorWrap(err, "closing listener failed"))
	}
}

func (rs *RemoteSigner) connect() (net.Conn, error) {
	for retries := rs.connRetries; retries > 0; retries-- {
		// Don't sleep if it is the first retry.
		if retries != rs.connRetries {
			time.Sleep(rs.connDeadline)
		}
		conn, err := rs.dialer()
		if err != nil {
			rs.Logger.Error("dialing", "err", err)
			continue
		}
		return conn, nil
	}

	return nil, ErrDialRetryMax
}

func (rs *RemoteSigner) handleConnection(conn net.Conn) {
	for {
		if !rs.IsRunning() {
			return // Ignore error from listener closing.
		}

		// Reset the connection deadline
		conn.SetDeadline(time.Now().Add(rs.connDeadline))

		req, err := readMsg(conn)
		if err != nil {
			if err != io.EOF {
				rs.Logger.Error("handleConnection readMsg", "err", err)
			}
			return
		}

		res, err := handleRequest(req, rs.chainID, rs.privVal)

		if err != nil {
			// only log the error; we'll reply with an error in res
			rs.Logger.Error("handleConnection handleRequest", "err", err)
		}

		err = writeMsg(conn, res)
		if err != nil {
			rs.Logger.Error("handleConnection writeMsg", "err", err)
			return
		}
	}
}
