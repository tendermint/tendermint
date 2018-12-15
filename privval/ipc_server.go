package privval

import (
	"io"
	"net"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// IPCRemoteSignerOption sets an optional parameter on the IPCRemoteSigner.
type IPCRemoteSignerOption func(*IPCRemoteSigner)

// IPCRemoteSignerConnDeadline sets the read and write deadline for connections
// from external signing processes.
func IPCRemoteSignerConnDeadline(deadline time.Duration) IPCRemoteSignerOption {
	return func(ss *IPCRemoteSigner) { ss.connDeadline = deadline }
}

// IPCRemoteSignerConnRetries sets the amount of attempted retries to connect.
func IPCRemoteSignerConnRetries(retries int) IPCRemoteSignerOption {
	return func(ss *IPCRemoteSigner) { ss.connRetries = retries }
}

// IPCRemoteSigner is a RPC implementation of PrivValidator that listens on a unix socket.
type IPCRemoteSigner struct {
	cmn.BaseService

	addr         string
	chainID      string
	connDeadline time.Duration
	connRetries  int
	privVal      types.PrivValidator

	listener *net.UnixListener
}

// NewIPCRemoteSigner returns an instance of IPCRemoteSigner.
func NewIPCRemoteSigner(
	logger log.Logger,
	chainID, socketAddr string,
	privVal types.PrivValidator,
) *IPCRemoteSigner {
	rs := &IPCRemoteSigner{
		addr:         socketAddr,
		chainID:      chainID,
		connDeadline: time.Second * defaultConnDeadlineSeconds,
		connRetries:  defaultDialRetries,
		privVal:      privVal,
	}

	rs.BaseService = *cmn.NewBaseService(logger, "IPCRemoteSigner", rs)

	return rs
}

// OnStart implements cmn.Service.
func (rs *IPCRemoteSigner) OnStart() error {
	err := rs.listen()
	if err != nil {
		err = cmn.ErrorWrap(err, "listen")
		rs.Logger.Error("OnStart", "err", err)
		return err
	}

	go func() {
		for {
			conn, err := rs.listener.AcceptUnix()
			if err != nil {
				rs.Logger.Error("AcceptUnix", "err", err)
				return
			}
			go rs.handleConnection(conn)
		}
	}()

	return nil
}

// OnStop implements cmn.Service.
func (rs *IPCRemoteSigner) OnStop() {
	if rs.listener != nil {
		if err := rs.listener.Close(); err != nil {
			rs.Logger.Error("OnStop", "err", cmn.ErrorWrap(err, "closing listener failed"))
		}
	}
}

func (rs *IPCRemoteSigner) listen() error {
	la, err := net.ResolveUnixAddr("unix", rs.addr)
	if err != nil {
		return err
	}

	rs.listener, err = net.ListenUnix("unix", la)

	return err
}

func (rs *IPCRemoteSigner) handleConnection(conn net.Conn) {
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
