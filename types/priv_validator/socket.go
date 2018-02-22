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
	defaultConnDeadlineSeconds      = 3
	defaultDialRetryIntervalSeconds = 1
	defaultDialRetryMax             = 10
)

// Socket errors.
var (
	ErrDialRetryMax = errors.New("Error max client retries")
)

var (
	connDeadline = time.Second * defaultConnDeadlineSeconds
)

// SocketClientOption sets an optional parameter on the SocketClient.
type SocketClientOption func(*socketClient)

// SocketClientTimeout sets the timeout for connecting to the external socket
// address.
func SocketClientTimeout(timeout time.Duration) SocketClientOption {
	return func(sc *socketClient) { sc.connectTimeout = timeout }
}

// socketClient implements PrivValidator, it uses a socket to request signatures
// from an external process.
type socketClient struct {
	cmn.BaseService

	conn    net.Conn
	privKey *crypto.PrivKeyEd25519

	addr           string
	connectTimeout time.Duration
}

// Check that socketClient implements PrivValidator2.
var _ types.PrivValidator2 = (*socketClient)(nil)

// NewsocketClient returns an instance of socketClient.
func NewSocketClient(
	logger log.Logger,
	socketAddr string,
	privKey *crypto.PrivKeyEd25519,
) *socketClient {
	sc := &socketClient{
		addr:           socketAddr,
		connectTimeout: time.Second * defaultConnDeadlineSeconds,
		privKey:        privKey,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "privValidatorsocketClient", sc)

	return sc
}

// OnStart implements cmn.Service.
func (sc *socketClient) OnStart() error {
	if err := sc.BaseService.OnStart(); err != nil {
		return err
	}

	conn, err := sc.connect()
	if err != nil {
		return err
	}

	sc.conn = conn

	return nil
}

// OnStop implements cmn.Service.
func (sc *socketClient) OnStop() {
	sc.BaseService.OnStop()

	if sc.conn != nil {
		sc.conn.Close()
	}
}

// GetAddress implements PrivValidator.
// TODO(xla): Remove when PrivValidator2 replaced PrivValidator.
func (sc *socketClient) GetAddress() types.Address {
	addr, err := sc.Address()
	if err != nil {
		panic(err)
	}

	return addr
}

// Address is an alias for PubKey().Address().
func (sc *socketClient) Address() (cmn.HexBytes, error) {
	p, err := sc.PubKey()
	if err != nil {
		return nil, err
	}

	return p.Address(), nil
}

// GetPubKey implements PrivValidator.
// TODO(xla): Remove when PrivValidator2 replaced PrivValidator.
func (sc *socketClient) GetPubKey() crypto.PubKey {
	pubKey, err := sc.PubKey()
	if err != nil {
		panic(err)
	}

	return pubKey
}

// PubKey implements PrivValidator2.
func (sc *socketClient) PubKey() (crypto.PubKey, error) {
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
func (sc *socketClient) SignVote(chainID string, vote *types.Vote) error {
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
func (sc *socketClient) SignProposal(chainID string, proposal *types.Proposal) error {
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
func (sc *socketClient) SignHeartbeat(chainID string, heartbeat *types.Heartbeat) error {
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

func (sc *socketClient) connect() (net.Conn, error) {
	retries := defaultDialRetryMax

RETRY_LOOP:
	for retries > 0 {
		if retries != defaultDialRetryMax {
			time.Sleep(sc.connectTimeout)
		}

		retries--

		conn, err := cmn.Connect(sc.addr)
		if err != nil {
			sc.Logger.Error(
				"sc connect",
				"addr", sc.addr,
				"err", errors.Wrap(err, "connection failed"),
			)

			continue RETRY_LOOP
		}

		if err := conn.SetDeadline(time.Now().Add(connDeadline)); err != nil {
			sc.Logger.Error(
				"sc connect",
				"err", errors.Wrap(err, "setting connection timeout failed"),
			)
			continue
		}

		if sc.privKey != nil {
			conn, err = p2pconn.MakeSecretConnection(conn, sc.privKey.Wrap())
			if err != nil {
				sc.Logger.Error(
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

//---------------------------------------------------------

// PrivValidatorSocketServer implements PrivValidator.
// It responds to requests over a socket
type PrivValidatorSocketServer struct {
	cmn.BaseService

	proto, addr    string
	listener       net.Listener
	maxConnections int
	privKey        *crypto.PrivKeyEd25519

	privVal PrivValidator
	chainID string
}

// NewPrivValidatorSocketServer returns an instance of
// PrivValidatorSocketServer.
func NewPrivValidatorSocketServer(
	logger log.Logger,
	chainID, socketAddr string,
	maxConnections int,
	privVal PrivValidator,
	privKey *crypto.PrivKeyEd25519,
) *PrivValidatorSocketServer {
	proto, addr := cmn.ProtocolAndAddress(socketAddr)
	pvss := &PrivValidatorSocketServer{
		proto:          proto,
		addr:           addr,
		maxConnections: maxConnections,
		privKey:        privKey,
		privVal:        privVal,
		chainID:        chainID,
	}
	pvss.BaseService = *cmn.NewBaseService(logger, "privValidatorSocketServer", pvss)
	return pvss
}

// OnStart implements cmn.Service.
func (pvss *PrivValidatorSocketServer) OnStart() error {
	ln, err := net.Listen(pvss.proto, pvss.addr)
	if err != nil {
		return err
	}

	pvss.listener = netutil.LimitListener(ln, pvss.maxConnections)

	go pvss.acceptConnections()

	return nil
}

// OnStop implements cmn.Service.
func (pvss *PrivValidatorSocketServer) OnStop() {
	if pvss.listener == nil {
		return
	}

	if err := pvss.listener.Close(); err != nil {
		pvss.Logger.Error("OnStop", "err", errors.Wrap(err, "closing listener failed"))
	}
}

func (pvss *PrivValidatorSocketServer) acceptConnections() {
	for {
		conn, err := pvss.listener.Accept()
		if err != nil {
			if !pvss.IsRunning() {
				return // Ignore error from listener closing.
			}
			pvss.Logger.Error(
				"accpetConnections",
				"err", errors.Wrap(err, "failed to accept connection"),
			)
			continue
		}

		if err := conn.SetDeadline(time.Now().Add(connDeadline)); err != nil {
			pvss.Logger.Error(
				"acceptConnetions",
				"err", errors.Wrap(err, "setting connection timeout failed"),
			)
			continue
		}

		if pvss.privKey != nil {
			conn, err = p2pconn.MakeSecretConnection(conn, pvss.privKey.Wrap())
			if err != nil {
				pvss.Logger.Error(
					"acceptConnections",
					"err", errors.Wrap(err, "secret connection failed"),
				)
				continue
			}
		}

		go pvss.handleConnection(conn)
	}
}

func (pvss *PrivValidatorSocketServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		if !pvss.IsRunning() {
			return // Ignore error from listener closing.
		}

		req, err := readMsg(conn)
		if err != nil {
			if err != io.EOF {
				pvss.Logger.Error("handleConnection", "err", err)
			}
			return
		}

		var res PrivValidatorSocketMsg

		switch r := req.(type) {
		case *PubKeyMsg:
			var p crypto.PubKey

			p, err = pvss.privVal.PubKey()
			res = &PubKeyMsg{p}
		case *SignVoteMsg:
			err = pvss.privVal.SignVote(pvss.chainID, r.Vote)
			res = &SignVoteMsg{r.Vote}
		case *SignProposalMsg:
			err = pvss.privVal.SignProposal(pvss.chainID, r.Proposal)
			res = &SignProposalMsg{r.Proposal}
		case *SignHeartbeatMsg:
			err = pvss.privVal.SignHeartbeat(pvss.chainID, r.Heartbeat)
			res = &SignHeartbeatMsg{r.Heartbeat}
		default:
			err = fmt.Errorf("unknown msg: %v", r)
		}

		if err != nil {
			pvss.Logger.Error("handleConnection", "err", err)
			return
		}

		err = writeMsg(conn, res)
		if err != nil {
			pvss.Logger.Error("handleConnection", "err", err)
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
		return nil, err
	}

	w, ok := read.(struct{ PrivValidatorSocketMsg })
	if !ok {
		return nil, errors.New("unknwon type")
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

	return err
}
