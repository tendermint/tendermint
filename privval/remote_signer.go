package privval

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/types"
)

// RemoteSignerClient implements PrivValidator, it uses a socket to request signatures
// from an external process.
type RemoteSignerClient struct {
	conn net.Conn
	lock sync.Mutex
}

// Check that RemoteSignerClient implements PrivValidator.
var _ types.PrivValidator = (*RemoteSignerClient)(nil)

// NewRemoteSignerClient returns an instance of RemoteSignerClient.
func NewRemoteSignerClient(
	conn net.Conn,
) *RemoteSignerClient {
	sc := &RemoteSignerClient{
		conn: conn,
	}
	return sc
}

// GetAddress implements PrivValidator.
func (sc *RemoteSignerClient) GetAddress() types.Address {
	pubKey, err := sc.getPubKey()
	if err != nil {
		panic(err)
	}

	return pubKey.Address()
}

// GetPubKey implements PrivValidator.
func (sc *RemoteSignerClient) GetPubKey() crypto.PubKey {
	pubKey, err := sc.getPubKey()
	if err != nil {
		panic(err)
	}

	return pubKey
}

func (sc *RemoteSignerClient) getPubKey() (crypto.PubKey, error) {
	sc.lock.Lock()
	defer sc.lock.Unlock()

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
func (sc *RemoteSignerClient) SignVote(chainID string, vote *types.Vote) error {
	sc.lock.Lock()
	defer sc.lock.Unlock()

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
		return resp.Error
	}
	*vote = *resp.Vote

	return nil
}

// SignProposal implements PrivValidator.
func (sc *RemoteSignerClient) SignProposal(
	chainID string,
	proposal *types.Proposal,
) error {
	sc.lock.Lock()
	defer sc.lock.Unlock()

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
		return resp.Error
	}
	*proposal = *resp.Proposal

	return nil
}

// Ping is used to check connection health.
func (sc *RemoteSignerClient) Ping() error {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	err := writeMsg(sc.conn, &PingRequest{})
	if err != nil {
		return err
	}

	res, err := readMsg(sc.conn)
	if err != nil {
		return err
	}
	_, ok := res.(*PingResponse)
	if !ok {
		return ErrUnexpectedResponse
	}

	return nil
}

// RemoteSignerMsg is sent between RemoteSigner and the RemoteSigner client.
type RemoteSignerMsg interface{}

func RegisterRemoteSignerMsg(cdc *amino.Codec) {
	cdc.RegisterInterface((*RemoteSignerMsg)(nil), nil)
	cdc.RegisterConcrete(&PubKeyMsg{}, "tendermint/remotesigner/PubKeyMsg", nil)
	cdc.RegisterConcrete(&SignVoteRequest{}, "tendermint/remotesigner/SignVoteRequest", nil)
	cdc.RegisterConcrete(&SignedVoteResponse{}, "tendermint/remotesigner/SignedVoteResponse", nil)
	cdc.RegisterConcrete(&SignProposalRequest{}, "tendermint/remotesigner/SignProposalRequest", nil)
	cdc.RegisterConcrete(&SignedProposalResponse{}, "tendermint/remotesigner/SignedProposalResponse", nil)
	cdc.RegisterConcrete(&PingRequest{}, "tendermint/remotesigner/PingRequest", nil)
	cdc.RegisterConcrete(&PingResponse{}, "tendermint/remotesigner/PingResponse", nil)
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

func (e *RemoteSignerError) Error() string {
	return fmt.Sprintf("RemoteSigner returned error #%d: %s", e.Code, e.Description)
}

func readMsg(r io.Reader) (msg RemoteSignerMsg, err error) {
	const maxRemoteSignerMsgSize = 1024 * 10
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(r, &msg, maxRemoteSignerMsgSize)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrConnTimeout, err.Error())
	}
	return
}

func writeMsg(w io.Writer, msg interface{}) (err error) {
	_, err = cdc.MarshalBinaryLengthPrefixedWriter(w, msg)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrConnTimeout, err.Error())
	}
	return
}

func handleRequest(req RemoteSignerMsg, chainID string, privVal types.PrivValidator) (RemoteSignerMsg, error) {
	var res RemoteSignerMsg
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
	case *PingRequest:
		res = &PingResponse{}
	default:
		err = fmt.Errorf("unknown msg: %v", r)
	}

	return res, err
}
