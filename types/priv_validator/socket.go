package types

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/pkg/errors"
	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
	"golang.org/x/net/netutil"

	p2pconn "github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/types"
)

const (
	defaultConnDeadlineSeconds = 3
	defaultConnWaitSeconds     = 60
	defaultDialRetries         = 10
	defaultSignersMax          = 1
)

// Socket errors.
var (
	ErrDialRetryMax    = errors.New("Error max client retries")
	ErrConnWaitTimeout = errors.New("Error waiting for external connection")
	ErrConnTimeout     = errors.New("Error connection timed out")
)

var (
	connDeadline = time.Second * defaultConnDeadlineSeconds
)

// SocketClientOption sets an optional parameter on the SocketClient.
type SocketClientOption func(*SocketClient)

// SocketClientConnDeadline sets the read and write deadline for connections
// from external signing processes.
func SocketClientConnDeadline(deadline time.Duration) SocketClientOption {
	return func(sc *SocketClient) { sc.connDeadline = deadline }
}

// SocketClientConnWait sets the timeout duration before connection of external
// signing processes are considered to be unsuccessful.
func SocketClientConnWait(timeout time.Duration) SocketClientOption {
	return func(sc *SocketClient) { sc.connWaitTimeout = timeout }
}

// SocketClient implements PrivValidator, it uses a socket to request signatures
// from an external process.
type SocketClient struct {
	cmn.BaseService

	addr            string
	connDeadline    time.Duration
	connWaitTimeout time.Duration
	privKey         *crypto.PrivKeyEd25519

	conn     net.Conn
	listener net.Listener
}

// Check that SocketClient implements PrivValidator2.
var _ types.PrivValidator2 = (*SocketClient)(nil)

// NewSocketClient returns an instance of SocketClient.
func NewSocketClient(
	logger log.Logger,
	socketAddr string,
	privKey *crypto.PrivKeyEd25519,
) *SocketClient {
	sc := &SocketClient{
		addr:            socketAddr,
		connDeadline:    time.Second * defaultConnDeadlineSeconds,
		connWaitTimeout: time.Second * defaultConnWaitSeconds,
		privKey:         privKey,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "SocketClient", sc)

	return sc
}

// OnStart implements cmn.Service.
func (sc *SocketClient) OnStart() error {
	if sc.listener == nil {
		if err := sc.listen(); err != nil {
			sc.Logger.Error(
				"OnStart",
				"err", errors.Wrap(err, "failed to listen"),
			)

			return err
		}
	}

	conn, err := sc.waitConnection()
	if err != nil {
		sc.Logger.Error(
			"OnStart",
			"err", errors.Wrap(err, "failed to accept connection"),
		)

		return err
	}

	sc.conn = conn

	return nil
}

// OnStop implements cmn.Service.
func (sc *SocketClient) OnStop() {
	sc.BaseService.OnStop()

	if sc.conn != nil {
		if err := sc.conn.Close(); err != nil {
			sc.Logger.Error(
				"OnStop",
				"err", errors.Wrap(err, "failed to close connection"),
			)
		}
	}

	if sc.listener != nil {
		if err := sc.listener.Close(); err != nil {
			sc.Logger.Error(
				"OnStop",
				"err", errors.Wrap(err, "failed to close listener"),
			)
		}
	}
}

// GetAddress implements PrivValidator.
// TODO(xla): Remove when PrivValidator2 replaced PrivValidator.
func (sc *SocketClient) GetAddress() types.Address {
	addr, err := sc.Address()
	if err != nil {
		panic(err)
	}

	return addr
}

// Address is an alias for PubKey().Address().
func (sc *SocketClient) Address() (cmn.HexBytes, error) {
	p, err := sc.PubKey()
	if err != nil {
		return nil, err
	}

	return p.Address(), nil
}

// GetPubKey implements PrivValidator.
// TODO(xla): Remove when PrivValidator2 replaced PrivValidator.
func (sc *SocketClient) GetPubKey() crypto.PubKey {
	pubKey, err := sc.PubKey()
	if err != nil {
		panic(err)
	}

	return pubKey
}

// PubKey implements PrivValidator2.
func (sc *SocketClient) PubKey() (crypto.PubKey, error) {
	err := writeMsg(sc.conn, &PubKeyMsg{})
	if err != nil {
		return crypto.PubKey{}, err
	}

	res, err := readMsg(sc.conn)
	if err != nil {
		return crypto.PubKey{}, err
	}

	return res.(*PubKeyMsg).PubKey, nil
}

// SignVote implements PrivValidator2.
func (sc *SocketClient) SignVote(chainID string, vote *types.Vote) error {
	err := writeMsg(sc.conn, &SignVoteMsg{Vote: vote})
	if err != nil {
		return err
	}

	res, err := readMsg(sc.conn)
	if err != nil {
		return err
	}

	*vote = *res.(*SignVoteMsg).Vote

	return nil
}

// SignProposal implements PrivValidator2.
func (sc *SocketClient) SignProposal(
	chainID string,
	proposal *types.Proposal,
) error {
	err := writeMsg(sc.conn, &SignProposalMsg{Proposal: proposal})
	if err != nil {
		return err
	}

	res, err := readMsg(sc.conn)
	if err != nil {
		return err
	}

	*proposal = *res.(*SignProposalMsg).Proposal

	return nil
}

// SignHeartbeat implements PrivValidator2.
func (sc *SocketClient) SignHeartbeat(
	chainID string,
	heartbeat *types.Heartbeat,
) error {
	err := writeMsg(sc.conn, &SignHeartbeatMsg{Heartbeat: heartbeat})
	if err != nil {
		return err
	}

	res, err := readMsg(sc.conn)
	if err != nil {
		return err
	}

	*heartbeat = *res.(*SignHeartbeatMsg).Heartbeat

	return nil
}

func (sc *SocketClient) acceptConnection() (net.Conn, error) {
	conn, err := sc.listener.Accept()
	if err != nil {
		if !sc.IsRunning() {
			return nil, nil // Ignore error from listener closing.
		}
		return nil, err

	}

	if err := conn.SetDeadline(time.Now().Add(sc.connDeadline)); err != nil {
		return nil, err
	}

	if sc.privKey != nil {
		conn, err = p2pconn.MakeSecretConnection(conn, sc.privKey.Wrap())
		if err != nil {
			return nil, err
		}
	}

	return conn, nil
}

func (sc *SocketClient) listen() error {
	ln, err := net.Listen(cmn.ProtocolAndAddress(sc.addr))
	if err != nil {
		return err
	}

	sc.listener = netutil.LimitListener(ln, defaultSignersMax)

	return nil
}

// waitConnection uses the configured wait timeout to error if no external
// process connects in the time period.
func (sc *SocketClient) waitConnection() (net.Conn, error) {
	var (
		connc = make(chan net.Conn, 1)
		errc  = make(chan error, 1)
	)

	go func(connc chan<- net.Conn, errc chan<- error) {
		conn, err := sc.acceptConnection()
		if err != nil {
			errc <- err
			return
		}

		connc <- conn
	}(connc, errc)

	select {
	case conn := <-connc:
		return conn, nil
	case err := <-errc:
		return nil, err
	case <-time.After(sc.connWaitTimeout):
		return nil, ErrConnWaitTimeout
	}
}

//---------------------------------------------------------

// RemoteSignerOption sets an optional parameter on the RemoteSigner.
type RemoteSignerOption func(*RemoteSigner)

// RemoteSignerConnDeadline sets the read and write deadline for connections
// from external signing processes.
func RemoteSignerConnDeadline(deadline time.Duration) RemoteSignerOption {
	return func(ss *RemoteSigner) { ss.connDeadline = deadline }
}

// RemoteSignerConnRetries sets the amount of attempted retries to connect.
func RemoteSignerConnRetries(retries int) RemoteSignerOption {
	return func(ss *RemoteSigner) { ss.connRetries = retries }
}

// RemoteSigner implements PrivValidator.
// It responds to requests over a socket
type RemoteSigner struct {
	cmn.BaseService

	addr         string
	chainID      string
	connDeadline time.Duration
	connRetries  int
	privKey      *crypto.PrivKeyEd25519
	privVal      PrivValidator

	conn net.Conn
}

// NewRemoteSigner returns an instance of
// RemoteSigner.
func NewRemoteSigner(
	logger log.Logger,
	chainID, socketAddr string,
	privVal PrivValidator,
	privKey *crypto.PrivKeyEd25519,
) *RemoteSigner {
	rs := &RemoteSigner{
		addr:         socketAddr,
		chainID:      chainID,
		connDeadline: time.Second * defaultConnDeadlineSeconds,
		connRetries:  defaultDialRetries,
		privKey:      privKey,
		privVal:      privVal,
	}

	rs.BaseService = *cmn.NewBaseService(logger, "RemoteSigner", rs)

	return rs
}

// OnStart implements cmn.Service.
func (rs *RemoteSigner) OnStart() error {
	conn, err := rs.connect()
	if err != nil {
		rs.Logger.Error("OnStart", "err", errors.Wrap(err, "connect"))

		return err
	}

	go rs.handleConnection(conn)

	return nil
}

// OnStop implements cmn.Service.
func (rs *RemoteSigner) OnStop() {
	if rs.conn == nil {
		return
	}

	if err := rs.conn.Close(); err != nil {
		rs.Logger.Error("OnStop", "err", errors.Wrap(err, "closing listener failed"))
	}
}

func (rs *RemoteSigner) connect() (net.Conn, error) {
	retries := defaultDialRetries

RETRY_LOOP:
	for retries > 0 {
		// Don't sleep if it is the first retry.
		if retries != defaultDialRetries {
			time.Sleep(rs.connDeadline)
		}

		retries--

		conn, err := cmn.Connect(rs.addr)
		if err != nil {
			rs.Logger.Error(
				"connect",
				"addr", rs.addr,
				"err", errors.Wrap(err, "connection failed"),
			)

			continue RETRY_LOOP
		}

		if err := conn.SetDeadline(time.Now().Add(connDeadline)); err != nil {
			rs.Logger.Error(
				"connect",
				"err", errors.Wrap(err, "setting connection timeout failed"),
			)
			continue
		}

		if rs.privKey != nil {
			conn, err = p2pconn.MakeSecretConnection(conn, rs.privKey.Wrap())
			if err != nil {
				rs.Logger.Error(
					"sc connect",
					"err", errors.Wrap(err, "encrypting connection failed"),
				)

				continue RETRY_LOOP
			}
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

		req, err := readMsg(conn)
		if err != nil {
			if err != io.EOF {
				rs.Logger.Error("handleConnection", "err", err)
			}
			return
		}

		var res PrivValidatorSocketMsg

		switch r := req.(type) {
		case *PubKeyMsg:
			var p crypto.PubKey

			p, err = rs.privVal.PubKey()
			res = &PubKeyMsg{p}
		case *SignVoteMsg:
			err = rs.privVal.SignVote(rs.chainID, r.Vote)
			res = &SignVoteMsg{r.Vote}
		case *SignProposalMsg:
			err = rs.privVal.SignProposal(rs.chainID, r.Proposal)
			res = &SignProposalMsg{r.Proposal}
		case *SignHeartbeatMsg:
			err = rs.privVal.SignHeartbeat(rs.chainID, r.Heartbeat)
			res = &SignHeartbeatMsg{r.Heartbeat}
		default:
			err = fmt.Errorf("unknown msg: %v", r)
		}

		if err != nil {
			rs.Logger.Error("handleConnection", "err", err)
			return
		}

		err = writeMsg(conn, res)
		if err != nil {
			rs.Logger.Error("handleConnection", "err", err)
			return
		}
	}
}

//---------------------------------------------------------

const (
	msgTypePubKey        = byte(0x01)
	msgTypeSignVote      = byte(0x10)
	msgTypeSignProposal  = byte(0x11)
	msgTypeSignHeartbeat = byte(0x12)
)

// PrivValidatorSocketMsg is a message sent between PrivValidatorSocket client
// and server.
type PrivValidatorSocketMsg interface{}

var _ = wire.RegisterInterface(
	struct{ PrivValidatorSocketMsg }{},
	wire.ConcreteType{&PubKeyMsg{}, msgTypePubKey},
	wire.ConcreteType{&SignVoteMsg{}, msgTypeSignVote},
	wire.ConcreteType{&SignProposalMsg{}, msgTypeSignProposal},
	wire.ConcreteType{&SignHeartbeatMsg{}, msgTypeSignHeartbeat},
)

// PubKeyMsg is a PrivValidatorSocket message containing the public key.
type PubKeyMsg struct {
	PubKey crypto.PubKey
}

// SignVoteMsg is a PrivValidatorSocket message containing a vote.
type SignVoteMsg struct {
	Vote *types.Vote
}

// SignProposalMsg is a PrivValidatorSocket message containing a Proposal.
type SignProposalMsg struct {
	Proposal *types.Proposal
}

// SignHeartbeatMsg is a PrivValidatorSocket message containing a Heartbeat.
type SignHeartbeatMsg struct {
	Heartbeat *types.Heartbeat
}

func readMsg(r io.Reader) (PrivValidatorSocketMsg, error) {
	var (
		n   int
		err error
	)

	read := wire.ReadBinary(struct{ PrivValidatorSocketMsg }{}, r, 0, &n, &err)
	if err != nil {
		if opErr, ok := err.(*net.OpError); ok {
			return nil, errors.Wrapf(ErrConnTimeout, opErr.Addr.String())
		}

		return nil, err
	}

	w, ok := read.(struct{ PrivValidatorSocketMsg })
	if !ok {
		return nil, errors.New("unknown type")
	}

	return w.PrivValidatorSocketMsg, nil
}

func writeMsg(w io.Writer, msg interface{}) error {
	var (
		err error
		n   int
	)

	// TODO(xla): This extra wrap should be gone with the sdk-2 update.
	wire.WriteBinary(struct{ PrivValidatorSocketMsg }{msg}, w, &n, &err)
	if opErr, ok := err.(*net.OpError); ok {
		return errors.Wrapf(ErrConnTimeout, opErr.Addr.String())
	}

	return err
}
