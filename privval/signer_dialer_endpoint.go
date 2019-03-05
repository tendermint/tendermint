package privval

import (
	"fmt"
	"io"
	"net"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// SignerServiceEndpointOption sets an optional parameter on the SignerDialerEndpoint.
type SignerServiceEndpointOption func(*SignerDialerEndpoint)

// SignerServiceEndpointTimeoutReadWrite sets the read and write timeout for connections
// from external signing processes.
func SignerServiceEndpointTimeoutReadWrite(timeout time.Duration) SignerServiceEndpointOption {
	return func(ss *SignerDialerEndpoint) { ss.timeoutReadWrite = timeout }
}

// SignerServiceEndpointConnRetries sets the amount of attempted retries to tryAcceptConnection.
func SignerServiceEndpointConnRetries(retries int) SignerServiceEndpointOption {
	return func(ss *SignerDialerEndpoint) { ss.connRetries = retries }
}

// TODO: Create a type for a signerEndpoint (common for both listener/dialer)

// SignerDialerEndpoint dials using its dialer and responds to any
// signature requests using its privVal.
type SignerDialerEndpoint struct {
	cmn.BaseService

	chainID          string
	timeoutReadWrite time.Duration
	connRetries      int
	privVal          types.PrivValidator

	dialer SocketDialer
	conn   net.Conn
}

// NewSignerDialerEndpoint returns a SignerDialerEndpoint that will dial using the given
// dialer and respond to any signature requests over the connection
// using the given privVal.
func NewSignerDialerEndpoint(
	logger log.Logger,
	chainID string,
	privVal types.PrivValidator,
	dialer SocketDialer,
) *SignerDialerEndpoint {
	se := &SignerDialerEndpoint{
		chainID:          chainID,
		timeoutReadWrite: time.Second * defaultTimeoutReadWriteSeconds,
		connRetries:      defaultMaxDialRetries,
		privVal:          privVal,
		dialer:           dialer,
	}

	se.BaseService = *cmn.NewBaseService(logger, "SignerDialerEndpoint", se)
	return se
}

// OnStart implements cmn.Service.
func (ss *SignerDialerEndpoint) OnStart() error {
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
func (ss *SignerDialerEndpoint) OnStop() {
	if ss.conn == nil {
		return
	}

	if err := ss.conn.Close(); err != nil {
		ss.Logger.Error("OnStop", "err", cmn.ErrorWrap(err, "closing listener failed"))
		ss.conn = nil
	}
}

func (ss *SignerDialerEndpoint) connect() (net.Conn, error) {
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

func (ss *SignerDialerEndpoint) readMessage() (msg RemoteSignerMsg, err error) {
	// TODO: Avoid duplication. Unify endpoints

	if ss.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(ss.timeoutReadWrite)
	ss.Logger.Debug("SignerDialerEndpoint: readMessage", "deadline", deadline)
	err = ss.conn.SetReadDeadline(deadline)
	if err != nil {
		return
	}

	const maxRemoteSignerMsgSize = 1024 * 10
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(ss.conn, &msg, maxRemoteSignerMsgSize)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrDialerTimeout, err.Error())
	}

	return
}

func (ss *SignerDialerEndpoint) writeMessage(msg RemoteSignerMsg) (err error) {
	// TODO: Avoid duplication. Unify endpoints

	if ss.conn == nil {
		return fmt.Errorf("not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(ss.timeoutReadWrite)
	ss.Logger.Debug("SignerDialerEndpoint: readMessage", "deadline", deadline)
	err = ss.conn.SetWriteDeadline(deadline)
	if err != nil {
		return
	}

	_, err = cdc.MarshalBinaryLengthPrefixedWriter(ss.conn, msg)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrDialerTimeout, err.Error())
	}

	return
}

func (ss *SignerDialerEndpoint) handleConnection(conn net.Conn) {
	for {
		if !ss.IsRunning() {
			return // Ignore error from listener closing.
		}

		ss.Logger.Debug("SignerDialerEndpoint: connected", "timeout", ss.timeoutReadWrite)

		req, err := ss.readMessage()
		if err != nil {
			if err != io.EOF {
				ss.Logger.Error("SignerDialerEndpoint handleConnection", "err", err)
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
