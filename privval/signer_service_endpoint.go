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
func (ss *SignerServiceEndpoint) OnStart() error {
	conn, err := ss.connect()
	if err != nil {
		ss.Logger.Error("OnStart", "err", err)
		return err
	}

	ss.conn = conn
	go ss.handleConnection(conn)

	return nil
}

// OnStop implements cmn.Service.
func (ss *SignerServiceEndpoint) OnStop() {
	if ss.conn == nil {
		return
	}

	if err := ss.conn.Close(); err != nil {
		ss.Logger.Error("OnStop", "err", cmn.ErrorWrap(err, "closing listener failed"))
	}
}

func (ss *SignerServiceEndpoint) connect() (net.Conn, error) {
	for retries := 0; retries < ss.connRetries; retries++ {
		// Don't sleep if it is the first retry.
		if retries > 0 {
			time.Sleep(ss.timeoutReadWrite)
		}

		conn, err := ss.dialer()
		if err == nil {
			return conn, nil
		}

		ss.Logger.Error("dialing", "err", err)
	}

	return nil, ErrDialRetryMax
}

func (ss *SignerServiceEndpoint) readMessage() (msg RemoteSignerMsg, err error) {
	// TODO: Avoid duplication
	// TODO: Check connection status

	const maxRemoteSignerMsgSize = 1024 * 10
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(ss.conn, &msg, maxRemoteSignerMsgSize)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrConnTimeout, err.Error())
	}

	return
}

func (ss *SignerServiceEndpoint) writeMessage(msg RemoteSignerMsg) (err error) {
	// TODO: Avoid duplication
	// TODO: Check connection status

	_, err = cdc.MarshalBinaryLengthPrefixedWriter(ss.conn, msg)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrConnTimeout, err.Error())
	}

	// TODO: Probably can assert that is a response type and check for error here
	// Check the impact of KMS/Rust

	return
}

func (ss *SignerServiceEndpoint) handleConnection(conn net.Conn) {
	for {
		if !ss.IsRunning() {
			return // Ignore error from listener closing.
		}

		// Reset the connection deadline
		deadline := time.Now().Add(ss.timeoutReadWrite)
		err := conn.SetDeadline(deadline)
		if err != nil {
			return
		}

		req, err := ss.readMessage()
		if err != nil {
			if err != io.EOF {
				ss.Logger.Error("handleConnection readMessage", "err", err)
			}
			return
		}

		res, err := handleRequest(req, ss.chainID, ss.privVal)

		if err != nil {
			// only log the error; we'll reply with an error in res
			ss.Logger.Error("handleConnection handleRequest", "err", err)
		}

		err = ss.writeMessage(res)
		if err != nil {
			ss.Logger.Error("handleConnection writeMessage", "err", err)
			return
		}
	}
}
