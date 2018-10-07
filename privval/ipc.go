package privval

import (
	"io"
	"net"
	"sync"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// IPCPV implements PrivValidator, it uses a unix socket to request signatures
// from an external process.
type IPCPV struct {
	cmn.BaseService
	*RemoteSignerClient

	addr string

	connTimeout time.Duration

	lock       sync.Mutex
	cancelPing chan bool
}

// Check that IPCPV implements PrivValidator.
var _ types.PrivValidator = (*IPCPV)(nil)

// NewIPCPV returns an instance of IPCPV.
func NewIPCPV(
	logger log.Logger,
	socketAddr string,
) *IPCPV {
	sc := &IPCPV{
		addr:        socketAddr,
		connTimeout: connTimeout,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "IPCPV", sc)
	sc.RemoteSignerClient = NewRemoteSignerClient(logger, nil)

	return sc
}

// OnStart implements cmn.Service.
func (sc *IPCPV) OnStart() error {
	err := sc.connect()
	if err != nil {
		err = cmn.ErrorWrap(err, "failed to connect")
		sc.Logger.Error(
			"OnStart",
			"err", err,
		)

		return err
	}

	err = sc.RemoteSignerClient.Start()
	if err != nil {
		err = cmn.ErrorWrap(err, "failed to start RemoteSignerClient")
		sc.Logger.Error(
			"OnStart",
			"err", err,
		)

		return err
	}

	return nil
}

// OnStop implements cmn.Service.
func (sc *IPCPV) OnStop() {
	if err := sc.RemoteSignerClient.Stop(); err != nil {
		err = cmn.ErrorWrap(err, "failed to stop RemoteSignerClient")
		sc.Logger.Error(
			"OnStop",
			"err", err,
		)
	}

	if sc.conn != nil {
		if err := sc.conn.Close(); err != nil {
			err = cmn.ErrorWrap(err, "failed to close connection")
			sc.Logger.Error(
				"OnStop",
				"err", err,
			)
		}
	}
}

func (sc *IPCPV) connect() error {
	la, err := net.ResolveUnixAddr("unix", sc.addr)
	if err != nil {
		return err
	}

	conn, err := net.DialUnix("unix", nil, la)
	if err != nil {
		return err
	}

	// Wrap in a timeoutConn
	sc.conn = newTimeoutConn(conn, sc.connTimeout)

	return nil
}

//---------------------------------------------------------

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
