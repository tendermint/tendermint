package privval

import (
	"net"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// IPCValOption sets an optional parameter on the SocketPV.
type IPCValOption func(*IPCVal)

// IPCValConnTimeout sets the read and write timeout for connections
// from external signing processes.
func IPCValConnTimeout(timeout time.Duration) IPCValOption {
	return func(sc *IPCVal) { sc.connTimeout = timeout }
}

// IPCValHeartbeat sets the period on which to check the liveness of the
// connected Signer connections.
func IPCValHeartbeat(period time.Duration) IPCValOption {
	return func(sc *IPCVal) { sc.connHeartbeat = period }
}

// IPCVal implements PrivValidator, it uses a unix socket to request signatures
// from an external process.
type IPCVal struct {
	cmn.BaseService
	*RemoteSignerClient

	addr string

	connTimeout   time.Duration
	connHeartbeat time.Duration

	conn       net.Conn
	cancelPing chan struct{}
	pingTicker *time.Ticker
}

// Check that IPCVal implements PrivValidator.
var _ types.PrivValidator = (*IPCVal)(nil)

// NewIPCVal returns an instance of IPCVal.
func NewIPCVal(
	logger log.Logger,
	socketAddr string,
) *IPCVal {
	sc := &IPCVal{
		addr:          socketAddr,
		connTimeout:   connTimeout,
		connHeartbeat: connHeartbeat,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "IPCVal", sc)

	return sc
}

// OnStart implements cmn.Service.
func (sc *IPCVal) OnStart() error {
	err := sc.connect()
	if err != nil {
		sc.Logger.Error("OnStart", "err", err)
		return err
	}

	sc.RemoteSignerClient = NewRemoteSignerClient(sc.conn)

	// Start a routine to keep the connection alive
	sc.cancelPing = make(chan struct{}, 1)
	sc.pingTicker = time.NewTicker(sc.connHeartbeat)
	go func() {
		for {
			select {
			case <-sc.pingTicker.C:
				err := sc.Ping()
				if err != nil {
					sc.Logger.Error("Ping", "err", err)
				}
			case <-sc.cancelPing:
				sc.pingTicker.Stop()
				return
			}
		}
	}()

	return nil
}

// OnStop implements cmn.Service.
func (sc *IPCVal) OnStop() {
	if sc.cancelPing != nil {
		close(sc.cancelPing)
	}

	if sc.conn != nil {
		if err := sc.conn.Close(); err != nil {
			sc.Logger.Error("OnStop", "err", err)
		}
	}
}

func (sc *IPCVal) connect() error {
	la, err := net.ResolveUnixAddr("unix", sc.addr)
	if err != nil {
		return err
	}

	conn, err := net.DialUnix("unix", nil, la)
	if err != nil {
		return err
	}

	sc.conn = newTimeoutConn(conn, sc.connTimeout)

	return nil
}
