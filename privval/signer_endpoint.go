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

	mtx  sync.Mutex
	conn net.Conn

	timeoutReadWrite time.Duration
	//retryWait        time.Duration
	//maxConnRetries   int

	stopServiceLoopCh    chan struct{}
	stoppedServiceLoopCh chan struct{}
}

// Close closes the underlying net.Conn.
func (sd *signerEndpoint) Close() error {
	sd.mtx.Lock()
	defer sd.mtx.Unlock()

	sd.dropConnection()
	return nil
}

// IsConnected indicates if there is an active connection
func (sl *signerEndpoint) IsConnected() bool {
	sl.mtx.Lock()
	defer sl.mtx.Unlock()
	return sl.isConnected()
}

func (se *signerEndpoint) isConnected() bool {
	return se.IsRunning() && se.conn != nil
}

func (se *signerEndpoint) readMessage() (msg RemoteSignerMsg, err error) {
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
		err = errors.Wrap(ErrDialerReadTimeout, err.Error())
		se.dropConnection()
	}

	return
}

func (se *signerEndpoint) writeMessage(msg RemoteSignerMsg) (err error) {
	if !se.isConnected() {
		return errors.Wrap(ErrListenerNoConnection, "endpoint is not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(se.timeoutReadWrite)

	err = se.conn.SetWriteDeadline(deadline)
	if err != nil {
		return
	}

	_, err = cdc.MarshalBinaryLengthPrefixedWriter(se.conn, msg)
	if _, ok := err.(timeoutError); ok {
		err = errors.Wrap(ErrDialerWriteTimeout, err.Error())
		se.dropConnection()
	}

	return
}

func (sd *signerEndpoint) dropConnection() {
	if sd.conn != nil {
		if err := sd.conn.Close(); err != nil {
			sd.Logger.Error("signerEndpoint::dropConnection", "err", err)
		}
		sd.conn = nil
	}
}
