package types

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/pkg/errors"
	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
	"golang.org/x/net/netutil"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------

var _ types.PrivValidator = (*PrivValidatorSocketClient)(nil)

// PrivValidatorSocketClient implements PrivValidator.
// It uses a socket to request signatures.
type PrivValidatorSocketClient struct {
	cmn.BaseService

	conn    net.Conn
	privKey *crypto.PrivKeyEd25519

	ID            types.ValidatorID
	SocketAddress string
}

const (
	dialRetryIntervalSeconds = 1
)

// NewPrivValidatorSocketClient returns an instance of
// PrivValidatorSocketClient.
func NewPrivValidatorSocketClient(
	logger log.Logger,
	socketAddr string,
	privKey *crypto.PrivKeyEd25519,
) *PrivValidatorSocketClient {
	pvsc := &PrivValidatorSocketClient{
		SocketAddress: socketAddr,
		privKey:       privKey,
	}

	pvsc.BaseService = *cmn.NewBaseService(logger, "privValidatorSocketClient", pvsc)

	return pvsc
}

// OnStart implements cmn.Service.
func (pvsc *PrivValidatorSocketClient) OnStart() error {
	if err := pvsc.BaseService.OnStart(); err != nil {
		return err
	}

	var (
		err  error
		conn net.Conn
	)

RETRY_LOOP:
	for {
		conn, err = cmn.Connect(pvsc.SocketAddress)
		if err != nil {
			pvsc.Logger.Error(
				"OnStart",
				"addr", pvsc.SocketAddress,
				"err", errors.Wrap(err, "connection failed"),
			)

			time.Sleep(time.Second * dialRetryIntervalSeconds)
			continue RETRY_LOOP
		}

		if pvsc.privKey != nil {
			conn, err = p2p.MakeSecretConnection(conn, *pvsc.privKey)
			if err != nil {
				pvsc.Logger.Error(
					"OnStart",
					"err", errors.Wrap(err, "encrypting connection failed"),
				)
				continue RETRY_LOOP
			}
		}

		pvsc.conn = conn
		return nil
	}
}

// OnStop implements cmn.Service.
func (pvsc *PrivValidatorSocketClient) OnStop() {
	pvsc.BaseService.OnStop()

	if pvsc.conn != nil {
		pvsc.conn.Close()
	}
}

// Address is an alias for PubKey().Address().
func (pvsc *PrivValidatorSocketClient) Address() data.Bytes {
	return pvsc.PubKey().Address()
}

// PubKey implements PrivValidator.
func (pvsc *PrivValidatorSocketClient) PubKey() crypto.PubKey {
	err := writeMsg(pvsc.conn, &PubKeyMsg{})
	if err != nil {
		panic(err)
	}

	res, err := readMsg(pvsc.conn)
	if err != nil {
		panic(err)
	}

	return res.(*PubKeyMsg).PubKey
}

// SignVote implements PrivValidator.
func (pvsc *PrivValidatorSocketClient) SignVote(chainID string, vote *types.Vote) error {
	err := writeMsg(pvsc.conn, &SignVoteMsg{Vote: vote})
	if err != nil {
		return err
	}

	res, err := readMsg(pvsc.conn)
	if err != nil {
		return err
	}

	*vote = *res.(*SignVoteMsg).Vote

	return nil
}

// SignProposal implements PrivValidator.
func (pvsc *PrivValidatorSocketClient) SignProposal(chainID string, proposal *types.Proposal) error {
	err := writeMsg(pvsc.conn, &SignProposalMsg{Proposal: proposal})
	if err != nil {
		return err
	}

	res, err := readMsg(pvsc.conn)
	if err != nil {
		return err
	}

	*proposal = *res.(*SignProposalMsg).Proposal

	return nil
}

// SignHeartbeat implements PrivValidator.
func (pvsc *PrivValidatorSocketClient) SignHeartbeat(chainID string, heartbeat *types.Heartbeat) error {
	err := writeMsg(pvsc.conn, &SignHeartbeatMsg{Heartbeat: heartbeat})
	if err != nil {
		return err
	}

	res, err := readMsg(pvsc.conn)
	if err != nil {
		return err
	}

	*heartbeat = *res.(*SignHeartbeatMsg).Heartbeat

	return nil
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

		if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
			pvss.Logger.Error(
				"acceptConnetions",
				"err", errors.Wrap(err, "setting connection timeout failed"),
			)
			continue
		}

		if pvss.privKey != nil {
			conn, err = p2p.MakeSecretConnection(conn, *pvss.privKey)
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
			res = &PubKeyMsg{pvss.privVal.PubKey()}
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
