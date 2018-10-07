package privval

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	p2pconn "github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/types"
)

const (
	defaultAcceptDeadlineSeconds = 3
	defaultConnDeadlineSeconds   = 3
	defaultConnHeartBeatSeconds  = 2
	defaultConnWaitSeconds       = 60
	defaultDialRetries           = 10
)

// Socket errors.
var (
	ErrDialRetryMax       = errors.New("dialed maximum retries")
	ErrConnWaitTimeout    = errors.New("waited for remote signer for too long")
	ErrConnTimeout        = errors.New("remote signer timed out")
	ErrUnexpectedResponse = errors.New("received unexpected response")
)

var (
	acceptDeadline = time.Second * defaultAcceptDeadlineSeconds
	connTimeout    = time.Second * defaultConnDeadlineSeconds
	connHeartbeat  = time.Second * defaultConnHeartBeatSeconds
)

// SocketPV implements PrivValidator, it uses a socket to request signatures
// from an external process.
type SocketPV struct {
	cmn.BaseService
	*RemoteSignerClient

	addr            string
	acceptDeadline  time.Duration
	connTimeout     time.Duration
	connWaitTimeout time.Duration
	privKey         ed25519.PrivKeyEd25519

	listener   net.Listener
	lock       sync.Mutex
	cancelPing chan bool
}

// Check that SocketPV implements PrivValidator.
var _ types.PrivValidator = (*SocketPV)(nil)

// NewSocketPV returns an instance of SocketPV.
func NewSocketPV(
	logger log.Logger,
	socketAddr string,
	privKey ed25519.PrivKeyEd25519,
) *SocketPV {
	sc := &SocketPV{
		addr:            socketAddr,
		acceptDeadline:  acceptDeadline,
		connTimeout:     connTimeout,
		connWaitTimeout: time.Second * defaultConnWaitSeconds,
		privKey:         privKey,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "SocketPV", sc)
	sc.RemoteSignerClient = NewRemoteSignerClient(sc.Logger, nil)

	return sc
}

// OnStart implements cmn.Service.
func (sc *SocketPV) OnStart() error {
	if err := sc.listen(); err != nil {
		err = cmn.ErrorWrap(err, "failed to listen")
		sc.Logger.Error(
			"OnStart",
			"err", err,
		)
		return err
	}

	conn, err := sc.waitConnection()
	if err != nil {
		err = cmn.ErrorWrap(err, "failed to accept connection")
		sc.Logger.Error(
			"OnStart",
			"err", err,
		)

		return err
	}

	sc.conn = conn

	err = sc.RemoteSignerClient.Start()
	if err != nil {
		err = cmn.ErrorWrap(err, "failed to start RemoteSignerClient")
		sc.Logger.Error(
			"OnStart",
			"err", err,
		)

		return err
	}

	return nil
}

// OnStop implements cmn.Service.
func (sc *SocketPV) OnStop() {
	if err := sc.RemoteSignerClient.Stop(); err != nil {
		err = cmn.ErrorWrap(err, "failed to stop RemoteSignerClient")
		sc.Logger.Error(
			"OnStop",
			"err", err,
		)
	}

	if sc.conn != nil {
		if err := sc.conn.Close(); err != nil {
			err = cmn.ErrorWrap(err, "failed to close connection")
			sc.Logger.Error(
				"OnStop",
				"err", err,
			)
		}
	}

	if sc.listener != nil {
		if err := sc.listener.Close(); err != nil {
			err = cmn.ErrorWrap(err, "failed to close listener")
			sc.Logger.Error(
				"OnStop",
				"err", err,
			)
		}
	}
}

func (sc *SocketPV) acceptConnection() (net.Conn, error) {
	conn, err := sc.listener.Accept()
	if err != nil {
		if !sc.IsRunning() {
			return nil, nil // Ignore error from listener closing.
		}
		return nil, err

	}

	conn, err = p2pconn.MakeSecretConnection(conn, sc.privKey)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (sc *SocketPV) listen() error {
	ln, err := net.Listen(cmn.ProtocolAndAddress(sc.addr))
	if err != nil {
		return err
	}

	sc.listener = newTCPTimeoutListener(
		ln,
		sc.acceptDeadline,
		sc.connTimeout,
		sc.connHeartbeat,
	)

	return nil
}

// waitConnection uses the configured wait timeout to error if no external
// process connects in the time period.
func (sc *SocketPV) waitConnection() (net.Conn, error) {
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
			return nil, cmn.ErrorWrap(ErrConnWaitTimeout, err.Error())
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
	privKey      ed25519.PrivKeyEd25519
	privVal      types.PrivValidator

	conn net.Conn
}

// NewRemoteSigner returns an instance of RemoteSigner.
func NewRemoteSigner(
	logger log.Logger,
	chainID, socketAddr string,
	privVal types.PrivValidator,
	privKey ed25519.PrivKeyEd25519,
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
		err = cmn.ErrorWrap(err, "connect")
		rs.Logger.Error("OnStart", "err", err)
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
		rs.Logger.Error("OnStop", "err", cmn.ErrorWrap(err, "closing listener failed"))
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
			err = cmn.ErrorWrap(err, "connection failed")
			rs.Logger.Error(
				"connect",
				"addr", rs.addr,
				"err", err,
			)

			continue
		}

		if err := conn.SetDeadline(time.Now().Add(connTimeout)); err != nil {
			err = cmn.ErrorWrap(err, "setting connection timeout failed")
			rs.Logger.Error(
				"connect",
				"err", err,
			)
			continue
		}

		conn, err = p2pconn.MakeSecretConnection(conn, rs.privKey)
		if err != nil {
			err = cmn.ErrorWrap(err, "encrypting connection failed")
			rs.Logger.Error(
				"connect",
				"err", err,
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

		// Reset the connection deadline
		conn.SetDeadline(time.Now().Add(rs.connDeadline))

		req, err := readMsg(conn)
		if err != nil {
			if err != io.EOF {
				rs.Logger.Error("handleConnection", "err", err)
			}
			return
		}

		res, err := handleRequest(req, rs.chainID, rs.privVal)

		if err != nil {
			// only log the error; we'll reply with an error in res
			rs.Logger.Error("handleConnection", "err", err)
		}

		err = writeMsg(conn, res)
		if err != nil {
			rs.Logger.Error("handleConnection", "err", err)
			return
		}
	}
}

func handleRequest(req SocketPVMsg, chainID string, privVal types.PrivValidator) (SocketPVMsg, error) {
	var res SocketPVMsg
	var err error

	switch r := req.(type) {
	case *PubKeyMsg:
		var p crypto.PubKey
		p = privVal.GetPubKey()
		res = &PubKeyMsg{p}
	case *SignVoteRequest:
		err = privVal.SignVote(chainID, r.Vote)
		if err != nil {
			res = &SignedVoteResponse{nil, &RemoteSignerError{0, err.Error()}}
		} else {
			res = &SignedVoteResponse{r.Vote, nil}
		}
	case *SignProposalRequest:
		err = privVal.SignProposal(chainID, r.Proposal)
		if err != nil {
			res = &SignedProposalResponse{nil, &RemoteSignerError{0, err.Error()}}
		} else {
			res = &SignedProposalResponse{r.Proposal, nil}
		}
	case *SignHeartbeatRequest:
		err = privVal.SignHeartbeat(chainID, r.Heartbeat)
		if err != nil {
			res = &SignedHeartbeatResponse{nil, &RemoteSignerError{0, err.Error()}}
		} else {
			res = &SignedHeartbeatResponse{r.Heartbeat, nil}
		}
	case *PingRequest:
		res = &PingResponse{}
	default:
		err = fmt.Errorf("unknown msg: %v", r)
	}

	return res, err
}

//---------------------------------------------------------

// SocketPVMsg is sent between RemoteSigner and SocketPV.
type SocketPVMsg interface{}

func RegisterSocketPVMsg(cdc *amino.Codec) {
	cdc.RegisterInterface((*SocketPVMsg)(nil), nil)
	cdc.RegisterConcrete(&PubKeyMsg{}, "tendermint/socketpv/PubKeyMsg", nil)
	cdc.RegisterConcrete(&SignVoteRequest{}, "tendermint/socketpv/SignVoteRequest", nil)
	cdc.RegisterConcrete(&SignedVoteResponse{}, "tendermint/socketpv/SignedVoteResponse", nil)
	cdc.RegisterConcrete(&SignProposalRequest{}, "tendermint/socketpv/SignProposalRequest", nil)
	cdc.RegisterConcrete(&SignedProposalResponse{}, "tendermint/socketpv/SignedProposalResponse", nil)
	cdc.RegisterConcrete(&SignHeartbeatRequest{}, "tendermint/socketpv/SignHeartbeatRequest", nil)
	cdc.RegisterConcrete(&SignedHeartbeatResponse{}, "tendermint/socketpv/SignedHeartbeatResponse", nil)
	cdc.RegisterConcrete(&PingRequest{}, "tendermint/socketpv/PingRequest", nil)
	cdc.RegisterConcrete(&PingResponse{}, "tendermint/socketpv/PingResponse", nil)
}

// PubKeyMsg is a PrivValidatorSocket message containing the public key.
type PubKeyMsg struct {
	PubKey crypto.PubKey
}

// SignVoteRequest is a PrivValidatorSocket message containing a vote.
type SignVoteRequest struct {
	Vote *types.Vote
}

// SignedVoteResponse is a PrivValidatorSocket message containing a signed vote along with a potenial error message.
type SignedVoteResponse struct {
	Vote  *types.Vote
	Error *RemoteSignerError
}

// SignProposalRequest is a PrivValidatorSocket message containing a Proposal.
type SignProposalRequest struct {
	Proposal *types.Proposal
}

type SignedProposalResponse struct {
	Proposal *types.Proposal
	Error    *RemoteSignerError
}

// SignHeartbeatRequest is a PrivValidatorSocket message containing a Heartbeat.
type SignHeartbeatRequest struct {
	Heartbeat *types.Heartbeat
}

type SignedHeartbeatResponse struct {
	Heartbeat *types.Heartbeat
	Error     *RemoteSignerError
}

// PingRequest is a PrivValidatorSocket message to keep the connection alive.
type PingRequest struct {
}

type PingResponse struct {
}

// RemoteSignerError allows (remote) validators to include meaningful error descriptions in their reply.
type RemoteSignerError struct {
	// TODO(ismail): create an enum of known errors
	Code        int
	Description string
}

func readMsg(r io.Reader) (msg SocketPVMsg, err error) {
	const maxSocketPVMsgSize = 1024 * 10
	_, err = cdc.UnmarshalBinaryReader(r, &msg, maxSocketPVMsgSize)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrConnTimeout, err.Error())
	}
	return
}

func writeMsg(w io.Writer, msg interface{}) (err error) {
	_, err = cdc.MarshalBinaryWriter(w, msg)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrConnTimeout, err.Error())
	}
	return
}
