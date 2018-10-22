package privval

import (
	"io"
	"net"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	p2pconn "github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/types"
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

// RemoteSigner implements PrivValidator by dialing to a socket.
type RemoteSigner struct {
	cmn.BaseService

	addr         string
	chainID      string
	connDeadline time.Duration
	connRetries  int
	privKey      ed25519.PrivKeyEd25519
	privVal      types.PrivValidator

	conn net.Conn
}

// NewRemoteSigner returns an instance of RemoteSigner.
func NewRemoteSigner(
	logger log.Logger,
	chainID, socketAddr string,
	privVal types.PrivValidator,
	privKey ed25519.PrivKeyEd25519,
) *RemoteSigner {
	rs := &RemoteSigner{
		addr:         socketAddr,
		chainID:      chainID,
		connDeadline: time.Second * defaultConnDeadlineSeconds,
		connRetries:  defaultDialRetries,
		privKey:      privKey,
		privVal:      privVal,
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

		conn, err := cmn.Connect(rs.addr)
		if err != nil {
			rs.Logger.Error(
				"connect",
				"addr", rs.addr,
				"err", err,
			)

			continue
		}

		if err := conn.SetDeadline(time.Now().Add(connTimeout)); err != nil {
			rs.Logger.Error(
				"connect",
				"err", err,
			)
			continue
		}

		conn, err = p2pconn.MakeSecretConnection(conn, rs.privKey)
		if err != nil {
			rs.Logger.Error(
				"connect",
				"err", err,
			)

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
				rs.Logger.Error("handleConnection", "err", err)
			}
			return
		}

		res, err := handleRequest(req, rs.chainID, rs.privVal)

		if err != nil {
			// only log the error; we'll reply with an error in res
			rs.Logger.Error("handleConnection", "err", err)
		}

		err = writeMsg(conn, res)
		if err != nil {
			rs.Logger.Error("handleConnection", "err", err)
			return
		}
	}
}
