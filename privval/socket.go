package privval

import (
	"errors"
	"fmt"
	"io"
	"net"
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
	defaultAcceptDeadlineSeconds = 30 // tendermint waits this long for remote val to connect
	defaultConnDeadlineSeconds   = 3  // must be set before each read
	defaultConnHeartBeatSeconds  = 30 // tcp keep-alive period
	defaultConnWaitSeconds       = 60 // XXX: is this redundant with the accept deadline?
	defaultDialRetries           = 10 // try to connect to tendermint this many times
)

// Socket errors.
var (
	ErrDialRetryMax       = errors.New("dialed maximum retries")
	ErrConnWaitTimeout    = errors.New("waited for remote signer for too long")
	ErrConnTimeout        = errors.New("remote signer timed out")
	ErrUnexpectedResponse = errors.New("received unexpected response")
)

// SocketPVOption sets an optional parameter on the SocketPV.
type SocketPVOption func(*SocketPV)

// SocketPVAcceptDeadline sets the deadline for the SocketPV listener.
// A zero time value disables the deadline.
func SocketPVAcceptDeadline(deadline time.Duration) SocketPVOption {
	return func(sc *SocketPV) { sc.acceptDeadline = deadline }
}

// SocketPVConnDeadline sets the read and write deadline for connections
// from external signing processes.
func SocketPVConnDeadline(deadline time.Duration) SocketPVOption {
	return func(sc *SocketPV) { sc.connDeadline = deadline }
}

// SocketPVHeartbeat sets the period on which to check the liveness of the
// connected Signer connections.
func SocketPVHeartbeat(period time.Duration) SocketPVOption {
	return func(sc *SocketPV) { sc.connHeartbeat = period }
}

// SocketPVConnWait sets the timeout duration before connection of external
// signing processes are considered to be unsuccessful.
func SocketPVConnWait(timeout time.Duration) SocketPVOption {
	return func(sc *SocketPV) { sc.connWaitTimeout = timeout }
}

// SocketPV implements PrivValidator, it uses a socket to request signatures
// from an external process.
type SocketPV struct {
	cmn.BaseService

	addr            string
	acceptDeadline  time.Duration
	connDeadline    time.Duration
	connHeartbeat   time.Duration
	connWaitTimeout time.Duration
	privKey         ed25519.PrivKeyEd25519

	conn     net.Conn
	listener net.Listener
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
		acceptDeadline:  time.Second * defaultAcceptDeadlineSeconds,
		connDeadline:    time.Second * defaultConnDeadlineSeconds,
		connHeartbeat:   time.Second * defaultConnHeartBeatSeconds,
		connWaitTimeout: time.Second * defaultConnWaitSeconds,
		privKey:         privKey,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "SocketPV", sc)

	return sc
}

// GetAddress implements PrivValidator.
func (sc *SocketPV) GetAddress() types.Address {
	addr, err := sc.getAddress()
	if err != nil {
		panic(err)
	}

	return addr
}

// Address is an alias for PubKey().Address().
func (sc *SocketPV) getAddress() (cmn.HexBytes, error) {
	p, err := sc.getPubKey()
	if err != nil {
		return nil, err
	}

	return p.Address(), nil
}

// GetPubKey implements PrivValidator.
func (sc *SocketPV) GetPubKey() crypto.PubKey {
	pubKey, err := sc.getPubKey()
	if err != nil {
		panic(err)
	}

	return pubKey
}

func (sc *SocketPV) getPubKey() (crypto.PubKey, error) {
	err := writeMsg(sc.conn, &PubKeyMsg{})
	if err != nil {
		return nil, err
	}

	res, err := readMsg(sc.conn)
	if err != nil {
		return nil, err
	}

	return res.(*PubKeyMsg).PubKey, nil
}

// SignVote implements PrivValidator.
func (sc *SocketPV) SignVote(chainID string, vote *types.Vote) error {
	err := writeMsg(sc.conn, &SignVoteRequest{Vote: vote})
	if err != nil {
		return err
	}

	res, err := readMsg(sc.conn)
	if err != nil {
		return err
	}

	resp, ok := res.(*SignedVoteResponse)
	if !ok {
		return ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return fmt.Errorf("remote error occurred: code: %v, description: %s",
			resp.Error.Code,
			resp.Error.Description)
	}
	*vote = *resp.Vote

	return nil
}

// SignProposal implements PrivValidator.
func (sc *SocketPV) SignProposal(
	chainID string,
	proposal *types.Proposal,
) error {
	err := writeMsg(sc.conn, &SignProposalRequest{Proposal: proposal})
	if err != nil {
		return err
	}

	res, err := readMsg(sc.conn)
	if err != nil {
		return err
	}
	resp, ok := res.(*SignedProposalResponse)
	if !ok {
		return ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return fmt.Errorf("remote error occurred: code: %v, description: %s",
			resp.Error.Code,
			resp.Error.Description)
	}
	*proposal = *resp.Proposal

	return nil
}

// SignHeartbeat implements PrivValidator.
func (sc *SocketPV) SignHeartbeat(
	chainID string,
	heartbeat *types.Heartbeat,
) error {
	err := writeMsg(sc.conn, &SignHeartbeatRequest{Heartbeat: heartbeat})
	if err != nil {
		return err
	}

	res, err := readMsg(sc.conn)
	if err != nil {
		return err
	}
	resp, ok := res.(*SignedHeartbeatResponse)
	if !ok {
		return ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return fmt.Errorf("remote error occurred: code: %v, description: %s",
			resp.Error.Code,
			resp.Error.Description)
	}
	*heartbeat = *resp.Heartbeat

	return nil
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

	return nil
}

// OnStop implements cmn.Service.
func (sc *SocketPV) OnStop() {
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
		sc.connDeadline,
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

		if err := conn.SetDeadline(time.Now().Add(time.Second * defaultConnDeadlineSeconds)); err != nil {
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

		req, err := readMsg(conn)
		if err != nil {
			if err != io.EOF {
				rs.Logger.Error("handleConnection", "err", err)
			}
			return
		}

		var res SocketPVMsg

		switch r := req.(type) {
		case *PubKeyMsg:
			var p crypto.PubKey
			p = rs.privVal.GetPubKey()
			res = &PubKeyMsg{p}
		case *SignVoteRequest:
			err = rs.privVal.SignVote(rs.chainID, r.Vote)
			if err != nil {
				res = &SignedVoteResponse{nil, &RemoteSignerError{0, err.Error()}}
			} else {
				res = &SignedVoteResponse{r.Vote, nil}
			}
		case *SignProposalRequest:
			err = rs.privVal.SignProposal(rs.chainID, r.Proposal)
			if err != nil {
				res = &SignedProposalResponse{nil, &RemoteSignerError{0, err.Error()}}
			} else {
				res = &SignedProposalResponse{r.Proposal, nil}
			}
		case *SignHeartbeatRequest:
			err = rs.privVal.SignHeartbeat(rs.chainID, r.Heartbeat)
			if err != nil {
				res = &SignedHeartbeatResponse{nil, &RemoteSignerError{0, err.Error()}}
			} else {
				res = &SignedHeartbeatResponse{r.Heartbeat, nil}
			}
		default:
			err = fmt.Errorf("unknown msg: %v", r)
		}

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

// RemoteSignerError allows (remote) validators to include meaningful error descriptions in their reply.
type RemoteSignerError struct {
	// TODO(ismail): create an enum of known errors
	Code        int
	Description string
}

func readMsg(r io.Reader) (msg SocketPVMsg, err error) {
	const maxSocketPVMsgSize = 1024 * 10

	// set deadline before trying to read
	conn := r.(net.Conn)
	if err := conn.SetDeadline(time.Now().Add(time.Second * defaultConnDeadlineSeconds)); err != nil {
		err = cmn.ErrorWrap(err, "setting connection timeout failed in readMsg")
		return msg, err
	}

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
