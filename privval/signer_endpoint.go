package privval

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/libs/service"
)

const (
	defaultTimeoutReadWriteSeconds = 5
)

type signerEndpoint struct {
	service.BaseService

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

// TryGetConnection retrieves a connection if it is already available
func (se *signerEndpoint) GetAvailableConnection(connectionAvailableCh chan net.Conn) bool {
	se.connMtx.Lock()
	defer se.connMtx.Unlock()

	// Is there a connection ready?
	select {
	case se.conn = <-connectionAvailableCh:
		return true
	default:
	}
	return false
}

// TryGetConnection retrieves a connection if it is already available
func (se *signerEndpoint) WaitConnection(connectionAvailableCh chan net.Conn, maxWait time.Duration) error {
	se.connMtx.Lock()
	defer se.connMtx.Unlock()

	select {
	case se.conn = <-connectionAvailableCh:
	case <-time.After(maxWait):
		return ErrConnectionTimeout
	}

	return nil
}

// SetConnection replaces the current connection object
func (se *signerEndpoint) SetConnection(newConnection net.Conn) {
	se.connMtx.Lock()
	defer se.connMtx.Unlock()
	se.conn = newConnection
}

// IsConnected indicates if there is an active connection
func (se *signerEndpoint) DropConnection() {
	se.connMtx.Lock()
	defer se.connMtx.Unlock()
	se.dropConnection()
}

// ReadMessage reads a message from the endpoint
func (se *signerEndpoint) ReadMessage() (msg SignerMessage, err error) {
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
			err = errors.Wrap(ErrReadTimeout, err.Error())
		} else {
			err = errors.Wrap(ErrReadTimeout, "Empty error")
		}
		se.Logger.Debug("Dropping [read]", "obj", se)
		se.dropConnection()
	}

	return
}

// WriteMessage writes a message from the endpoint
func (se *signerEndpoint) WriteMessage(msg SignerMessage) (err error) {
	se.connMtx.Lock()
	defer se.connMtx.Unlock()

	if !se.isConnected() {
		return errors.Wrap(ErrNoConnection, "endpoint is not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(se.timeoutReadWrite)
	err = se.conn.SetWriteDeadline(deadline)
	if err != nil {
		return
	}

	_, err = cdc.MarshalBinaryLengthPrefixedWriter(se.conn, msg)
	if _, ok := err.(timeoutError); ok {
		if err != nil {
			err = errors.Wrap(ErrWriteTimeout, err.Error())
		} else {
			err = errors.Wrap(ErrWriteTimeout, "Empty error")
		}
		se.dropConnection()
	}

	return
}

func (se *signerEndpoint) isConnected() bool {
	return se.conn != nil
}

func (se *signerEndpoint) dropConnection() {
	if se.conn != nil {
		if err := se.conn.Close(); err != nil {
			se.Logger.Error("signerEndpoint::dropConnection", "err", err)
		}
		se.conn = nil
	}
}
