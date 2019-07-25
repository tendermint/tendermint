package privval

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	cmn "github.com/tendermint/tendermint/libs/common"
)

const (
	defaultTimeoutReadWriteSeconds = 3
)

type signerEndpoint struct {
	cmn.BaseService

	connMtx sync.Mutex
	conn    net.Conn

	timeoutReadWrite time.Duration
}

// Close closes the underlying net.Conn.
func (se *signerEndpoint) Close() error {
	se.DropConnection()
	return nil
}

// IsConnected indicates if there is an active connection
func (se *signerEndpoint) IsConnected() bool {
	se.connMtx.Lock()
	defer se.connMtx.Unlock()
	return se.isConnected()
}

// IsConnected indicates if there is an active connection
func (se *signerEndpoint) DropConnection() {
	se.connMtx.Lock()
	defer se.connMtx.Unlock()
	se.dropConnection()
}

func (se *signerEndpoint) ReadMessage() (msg RemoteSignerMsg, err error) {
	se.connMtx.Lock()
	defer se.connMtx.Unlock()

	if !se.isConnected() {
		return nil, fmt.Errorf("endpoint is not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(se.timeoutReadWrite)

	err = se.conn.SetReadDeadline(deadline)
	if err != nil {
		return
	}

	const maxRemoteSignerMsgSize = 1024 * 10
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(se.conn, &msg, maxRemoteSignerMsgSize)
	if _, ok := err.(timeoutError); ok {
		if err != nil {
			err = errors.Wrap(ErrDialerReadTimeout, err.Error())
		} else {
			err = errors.Wrap(ErrDialerReadTimeout, "Empty error")
		}
		se.Logger.Debug("Dropping [read]", "obj", se)
		se.dropConnection()
	}

	return
}

func (se *signerEndpoint) WriteMessage(msg RemoteSignerMsg) (err error) {
	se.connMtx.Lock()
	defer se.connMtx.Unlock()

	if !se.isConnected() {
		return errors.Wrap(ErrListenerNoConnection, "endpoint is not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(se.timeoutReadWrite)
	se.Logger.Debug("Write::Error Resetting deadline", "obj", se)

	err = se.conn.SetWriteDeadline(deadline)
	if err != nil {
		return
	}

	_, err = cdc.MarshalBinaryLengthPrefixedWriter(se.conn, msg)
	if _, ok := err.(timeoutError); ok {
		if err != nil {
			err = errors.Wrap(ErrDialerWriteTimeout, err.Error())
		} else {
			err = errors.Wrap(ErrDialerWriteTimeout, "Empty error")
		}
		se.dropConnection()
	}

	return
}

func (se *signerEndpoint) isConnected() bool {
	return se.IsRunning() && se.conn != nil
}

func (se *signerEndpoint) dropConnection() {
	if se.conn != nil {
		if err := se.conn.Close(); err != nil {
			se.Logger.Error("signerEndpoint::dropConnection", "err", err)
		}
		se.conn = nil
	}
}
