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

	p2pconn "github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/types"
)

const (
	defaultAcceptDeadlineSeconds = 3
	defaultConnDeadlineSeconds   = 3
	defaultConnHeartBeatSeconds  = 30
	defaultConnWaitSeconds       = 60
	defaultDialRetries           = 10
)

// Socket errors.
var (
	ErrDialRetryMax    = errors.New("dialed maximum retries")
	ErrConnWaitTimeout = errors.New("waited for remote signer for too long")
	ErrConnTimeout     = errors.New("remote signer timed out")
)

var (
	acceptDeadline = time.Second + defaultAcceptDeadlineSeconds
	connDeadline   = time.Second * defaultConnDeadlineSeconds
	connHeartbeat  = time.Second * defaultConnHeartBeatSeconds
)

// SocketClientOption sets an optional parameter on the SocketClient.
type SocketClientOption func(*SocketClient)

// SocketClientAcceptDeadline sets the deadline for the SocketClient listener.
// A zero time value disables the deadline.
func SocketClientAcceptDeadline(deadline time.Duration) SocketClientOption {
	return func(sc *SocketClient) { sc.acceptDeadline = deadline }
}

// SocketClientConnDeadline sets the read and write deadline for connections
// from external signing processes.
func SocketClientConnDeadline(deadline time.Duration) SocketClientOption {
	return func(sc *SocketClient) { sc.connDeadline = deadline }
}

// SocketClientHeartbeat sets the period on which to check the liveness of the
// connected Signer connections.
func SocketClientHeartbeat(period time.Duration) SocketClientOption {
	return func(sc *SocketClient) { sc.connHeartbeat = period }
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
	acceptDeadline  time.Duration
	connDeadline    time.Duration
	connHeartbeat   time.Duration
	connWaitTimeout time.Duration
	privKey         crypto.PrivKeyEd25519

	conn     net.Conn
	listener net.Listener
}

// Check that SocketClient implements PrivValidator2.
var _ types.PrivValidator2 = (*SocketClient)(nil)

// NewSocketClient returns an instance of SocketClient.
func NewSocketClient(
	logger log.Logger,
	socketAddr string,
	privKey crypto.PrivKeyEd25519,
) *SocketClient {
	sc := &SocketClient{
		addr:            socketAddr,
		acceptDeadline:  acceptDeadline,
		connDeadline:    connDeadline,
		connHeartbeat:   connHeartbeat,
		connWaitTimeout: time.Second * defaultConnWaitSeconds,
		privKey:         privKey,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "SocketClient", sc)

	return sc
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

// OnStart implements cmn.Service.
func (sc *SocketClient) OnStart() error {
	if err := sc.listen(); err != nil {
		sc.Logger.Error(
			"OnStart",
			"err", errors.Wrap(err, "failed to listen"),
		)

		return err
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

func (sc *SocketClient) acceptConnection() (net.Conn, error) {
	conn, err := sc.listener.Accept()
	if err != nil {
		if !sc.IsRunning() {
			return nil, nil // Ignore error from listener closing.
		}
		return nil, err

	}

	conn, err = p2pconn.MakeSecretConnection(conn, sc.privKey.Wrap())
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (sc *SocketClient) listen() error {
	ln, err := net.Listen(cmn.ProtocolAndAddress(sc.addr))
	if err != nil {
		return err
	}

	sc.listener = newTCPTimeoutListener(
		ln,
		sc.acceptDeadline,
		sc.connDeadline,
		sc.connHeartbeat,
	)

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
		if _, ok := err.(timeoutError); ok {
			return nil, errors.Wrap(ErrConnWaitTimeout, err.Error())
		}
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

// RemoteSigner implements PrivValidator by dialing to a socket.
type RemoteSigner struct {
	cmn.BaseService

	addr         string
	chainID      string
	connDeadline time.Duration
	connRetries  int
	privKey      crypto.PrivKeyEd25519
	privVal      PrivValidator

	conn net.Conn
}

// NewRemoteSigner returns an instance of RemoteSigner.
func NewRemoteSigner(
	logger log.Logger,
	chainID, socketAddr string,
	privVal PrivValidator,
	privKey crypto.PrivKeyEd25519,
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
	for retries := rs.connRetries; retries > 0; retries-- {
		// Don't sleep if it is the first retry.
		if retries != rs.connRetries {
			time.Sleep(rs.connDeadline)
		}

		conn, err := cmn.Connect(rs.addr)
		if err != nil {
			rs.Logger.Error(
				"connect",
				"addr", rs.addr,
				"err", errors.Wrap(err, "connection failed"),
			)

			continue
		}

		if err := conn.SetDeadline(time.Now().Add(connDeadline)); err != nil {
			rs.Logger.Error(
				"connect",
				"err", errors.Wrap(err, "setting connection timeout failed"),
			)
			continue
		}

		conn, err = p2pconn.MakeSecretConnection(conn, rs.privKey.Wrap())
		if err != nil {
			rs.Logger.Error(
				"connect",
				"err", errors.Wrap(err, "encrypting connection failed"),
			)

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

		req, err := readMsg(conn)
		if err != nil {
			if err != io.EOF {
				rs.Logger.Error("handleConnection", "err", err)
			}
			return
		}

		var res PrivValMsg

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

// PrivValMsg is sent between RemoteSigner and SocketClient.
type PrivValMsg interface{}

var _ = wire.RegisterInterface(
	struct{ PrivValMsg }{},
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

func readMsg(r io.Reader) (PrivValMsg, error) {
	var (
		n   int
		err error
	)

	read := wire.ReadBinary(struct{ PrivValMsg }{}, r, 0, &n, &err)
	if err != nil {
		if _, ok := err.(timeoutError); ok {
			return nil, errors.Wrap(ErrConnTimeout, err.Error())
		}

		return nil, err
	}

	w, ok := read.(struct{ PrivValMsg })
	if !ok {
		return nil, errors.New("unknown type")
	}

	return w.PrivValMsg, nil
}

func writeMsg(w io.Writer, msg interface{}) error {
	var (
		err error
		n   int
	)

	// TODO(xla): This extra wrap should be gone with the sdk-2 update.
	wire.WriteBinary(struct{ PrivValMsg }{msg}, w, &n, &err)
	if _, ok := err.(timeoutError); ok {
		return errors.Wrap(ErrConnTimeout, err.Error())
	}

	return err
}
