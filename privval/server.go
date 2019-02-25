package privval

import (
	"io"
	"net"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// RemoteSignerOption sets an optional parameter on the RemoteSigner.
type RemoteSignerOption func(*RemoteSigner)

// RemoteSignerTimeoutReadWrite sets the read and write timeout for connections
// from external signing processes.
func RemoteSignerTimeoutReadWrite(timeout time.Duration) RemoteSignerOption {
	return func(ss *RemoteSigner) { ss.timeoutReadWrite = timeout }
}

// RemoteSignerConnRetries sets the amount of attempted retries to connect.
func RemoteSignerConnRetries(retries int) RemoteSignerOption {
	return func(ss *RemoteSigner) { ss.connRetries = retries }
}

// RemoteSigner dials using its dialer and responds to any
// signature requests using its privVal.
type RemoteSigner struct {
	cmn.BaseService

	chainID          string
	timeoutReadWrite time.Duration
	connRetries      int
	privVal          types.PrivValidator

	dialer Dialer
	conn   net.Conn
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
		chainID:          chainID,
		timeoutReadWrite: time.Second * defaultTimeoutReadWriteSeconds,
		connRetries:      defaultDialRetries,
		privVal:          privVal,
		dialer:           dialer,
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
			time.Sleep(rs.timeoutReadWrite)
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
		deadline := time.Now().Add(rs.timeoutReadWrite)
		err := conn.SetDeadline(deadline)
		if err != nil {
			return
		}

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
