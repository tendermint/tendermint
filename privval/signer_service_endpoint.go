package privval

import (
	"io"
	"net"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// SignerServiceEndpointOption sets an optional parameter on the SignerServiceEndpoint.
type SignerServiceEndpointOption func(*SignerServiceEndpoint)

// SignerServiceEndpointTimeoutReadWrite sets the read and write timeout for connections
// from external signing processes.
func SignerServiceEndpointTimeoutReadWrite(timeout time.Duration) SignerServiceEndpointOption {
	return func(ss *SignerServiceEndpoint) { ss.timeoutReadWrite = timeout }
}

// SignerServiceEndpointConnRetries sets the amount of attempted retries to connect.
func SignerServiceEndpointConnRetries(retries int) SignerServiceEndpointOption {
	return func(ss *SignerServiceEndpoint) { ss.connRetries = retries }
}

// SignerServiceEndpoint dials using its dialer and responds to any
// signature requests using its privVal.
type SignerServiceEndpoint struct {
	cmn.BaseService

	chainID          string
	timeoutReadWrite time.Duration
	connRetries      int
	privVal          types.PrivValidator

	dialer SocketDialer
	conn   net.Conn
}

// NewSignerServiceEndpoint returns a SignerServiceEndpoint that will dial using the given
// dialer and respond to any signature requests over the connection
// using the given privVal.
func NewSignerServiceEndpoint(
	logger log.Logger,
	chainID string,
	privVal types.PrivValidator,
	dialer SocketDialer,
) *SignerServiceEndpoint {
	se := &SignerServiceEndpoint{
		chainID:          chainID,
		timeoutReadWrite: time.Second * defaultTimeoutReadWriteSeconds,
		connRetries:      defaultMaxDialRetries,
		privVal:          privVal,
		dialer:           dialer,
	}

	se.BaseService = *cmn.NewBaseService(logger, "SignerServiceEndpoint", se)
	return se
}

// OnStart implements cmn.Service.
func (se *SignerServiceEndpoint) OnStart() error {
	conn, err := se.connect()
	if err != nil {
		se.Logger.Error("OnStart", "err", err)
		return err
	}

	se.conn = conn
	go se.handleConnection(conn)

	return nil
}

// OnStop implements cmn.Service.
func (se *SignerServiceEndpoint) OnStop() {
	if se.conn == nil {
		return
	}

	if err := se.conn.Close(); err != nil {
		se.Logger.Error("OnStop", "err", cmn.ErrorWrap(err, "closing listener failed"))
	}
}

func (se *SignerServiceEndpoint) connect() (net.Conn, error) {
	for retries := 0; retries < se.connRetries; retries++ {
		// Don't sleep if it is the first retry.
		if retries > 0 {
			time.Sleep(se.timeoutReadWrite)
		}

		conn, err := se.dialer()
		if err == nil {
			return conn, nil
		}

		se.Logger.Error("dialing", "err", err)
	}

	return nil, ErrDialRetryMax
}

func (se *SignerServiceEndpoint) handleConnection(conn net.Conn) {
	for {
		if !se.IsRunning() {
			return // Ignore error from listener closing.
		}

		// Reset the connection deadline
		deadline := time.Now().Add(se.timeoutReadWrite)
		err := conn.SetDeadline(deadline)
		if err != nil {
			return
		}

		req, err := readMsg(conn)
		if err != nil {
			if err != io.EOF {
				se.Logger.Error("handleConnection readMsg", "err", err)
			}
			return
		}

		res, err := handleRequest(req, se.chainID, se.privVal)

		if err != nil {
			// only log the error; we'll reply with an error in res
			se.Logger.Error("handleConnection handleRequest", "err", err)
		}

		err = writeMsg(conn, res)
		if err != nil {
			se.Logger.Error("handleConnection writeMsg", "err", err)
			return
		}
	}
}
