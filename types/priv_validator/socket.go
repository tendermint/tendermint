package types

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"time"

	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
	"golang.org/x/net/netutil"

	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------

var _ types.PrivValidator = (*PrivValidatorSocketClient)(nil)

// PrivValidatorSocketClient implements PrivValidator.
// It uses a socket to request signatures.
type PrivValidatorSocketClient struct {
	cmn.BaseService

	conn net.Conn

	ID            types.ValidatorID
	SocketAddress string
}

const (
	dialRetryIntervalSeconds = 1
)

// NewPrivValidatorSocketClient returns an instance of
// PrivValidatorSocketClient.
func NewPrivValidatorSocketClient(logger log.Logger, socketAddr string) *PrivValidatorSocketClient {
	pvsc := &PrivValidatorSocketClient{
		SocketAddress: socketAddr,
	}
	pvsc.BaseService = *cmn.NewBaseService(logger, "privValidatorSocketClient", pvsc)
	return pvsc
}

// OnStart implements cmn.Service.
func (pvsc *PrivValidatorSocketClient) OnStart() error {
	if err := pvsc.BaseService.OnStart(); err != nil {
		return err
	}

	var err error
	var conn net.Conn
RETRY_LOOP:
	for {
		conn, err = cmn.Connect(pvsc.SocketAddress)
		if err != nil {
			pvsc.Logger.Error(fmt.Sprintf("PrivValidatorSocket failed to connect to %v.  Retrying...", pvsc.SocketAddress))
			time.Sleep(time.Second * dialRetryIntervalSeconds)
			continue RETRY_LOOP
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
	pubKey := pvsc.PubKey()
	return pubKey.Address()
}

// PubKey implements PrivValidator.
func (pvsc *PrivValidatorSocketClient) PubKey() crypto.PubKey {
	res, err := readWrite(pvsc.conn, &PubKeyMsg{})
	if err != nil {
		panic(err)
	}
	return res.(*PubKeyMsg).PubKey
}

// SignVote implements PrivValidator.
func (pvsc *PrivValidatorSocketClient) SignVote(chainID string, vote *types.Vote) error {
	res, err := readWrite(pvsc.conn, &SignVoteMsg{Vote: vote})
	if err != nil {
		return err
	}
	*vote = *res.(SignVoteMsg).Vote
	return nil
}

// SignProposal implements PrivValidator.
func (pvsc *PrivValidatorSocketClient) SignProposal(chainID string, proposal *types.Proposal) error {
	res, err := readWrite(pvsc.conn, &SignProposalMsg{Proposal: proposal})
	if err != nil {
		return err
	}
	*proposal = *res.(SignProposalMsg).Proposal
	return nil
}

// SignHeartbeat implements PrivValidator.
func (pvsc *PrivValidatorSocketClient) SignHeartbeat(chainID string, heartbeat *types.Heartbeat) error {
	res, err := readWrite(pvsc.conn, &SignHeartbeatMsg{Heartbeat: heartbeat})
	if err != nil {
		return err
	}
	*heartbeat = *res.(SignHeartbeatMsg).Heartbeat
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
	privKey        crypto.PrivKeyEd25519

	privVal PrivValidator
	chainID string
}

// NewPrivValidatorSocketServer returns an instance of
// PrivValidatorSocketServer.
func NewPrivValidatorSocketServer(
	logger log.Logger,
	socketAddr, chainID string,
	privVal PrivValidator,
	privKey crypto.PrivKeyEd25519,
	maxConnections int,
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

	go pvss.acceptConnectionsRoutine()

	return nil
}

// OnStop implements cmn.Service.
func (pvss *PrivValidatorSocketServer) OnStop() {
	if pvss.listener == nil {
		return
	}

	if err := pvss.listener.Close(); err != nil {
		pvss.Logger.Error("Error closing listener", "err", err)
	}
}

func (pvss *PrivValidatorSocketServer) acceptConnectionsRoutine() {
	for {
		conn, err := pvss.listener.Accept()
		if err != nil {
			if !pvss.IsRunning() {
				return // Ignore error from listener closing.
			}
			pvss.Logger.Error("Failed to accept connection: " + err.Error())
			continue
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
			pvss.Logger.Error("readMsg", "err", err)
			return
		}

		var res PrivValidatorSocketMsg

		switch r := req.(type) {
		case *PubKeyMsg:
			res = &PubKeyMsg{pvss.privVal.PubKey()}
		case *SignVoteMsg:
			pvss.privVal.SignVote(pvss.chainID, r.Vote)
			res = &SignVoteMsg{r.Vote}
		case *SignProposalMsg:
			pvss.privVal.SignProposal(pvss.chainID, r.Proposal)
			res = &SignProposalMsg{r.Proposal}
		case *SignHeartbeatMsg:
			pvss.privVal.SignHeartbeat(pvss.chainID, r.Heartbeat)
			res = &SignHeartbeatMsg{r.Heartbeat}
		default:
			panic(fmt.Sprintf("unknown msg: %v", r))
		}

		b := wire.BinaryBytes(res)
		_, err = conn.Write(b)
		if err != nil {
			panic(err)
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
		return nil, fmt.Errorf("unknown type")
	}

	return w.PrivValidatorSocketMsg, nil
}

func readWrite(conn net.Conn, req PrivValidatorSocketMsg) (res PrivValidatorSocketMsg, err error) {
	b := wire.BinaryBytes(req)

	_, err = conn.Write(b)
	if err != nil {
		return nil, err
	}

	return readMsg(conn)
}

func decodeMsg(bz []byte) (msg PrivValidatorSocketMsg, err error) {
	var (
		r = bytes.NewReader(bz)

		n int
	)

	msgI := wire.ReadBinary(struct{ PrivValidatorSocketMsg }{}, r, 0, &n, &err)

	msg = msgI.(struct{ PrivValidatorSocketMsg }).PrivValidatorSocketMsg

	return msg, err
}

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
