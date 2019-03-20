package privval

import (
	"fmt"
	"io"
	"net"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/types"
)

// SignerRemote implements PrivValidator.
// It uses a net.Conn to request signatures from an external process.
type SignerRemote struct {
	conn net.Conn

	// memoized
	consensusPubKey crypto.PubKey
}

// Check that SignerRemote implements PrivValidator.
var _ types.PrivValidator = (*SignerRemote)(nil)

// NewSignerRemote returns an instance of SignerRemote.
func NewSignerRemote(conn net.Conn) (*SignerRemote, error) {

	// retrieve and memoize the consensus public key once.
	pubKey, err := getPubKey(conn)
	if err != nil {
		return nil, cmn.ErrorWrap(err, "error while retrieving public key for remote signer")
	}
	return &SignerRemote{
		conn:            conn,
		consensusPubKey: pubKey,
	}, nil
}

// Close calls Close on the underlying net.Conn.
func (sc *SignerRemote) Close() error {
	return sc.conn.Close()
}

// GetPubKey implements PrivValidator.
func (sc *SignerRemote) GetPubKey() crypto.PubKey {
	return sc.consensusPubKey
}

// not thread-safe (only called on startup).
func getPubKey(conn net.Conn) (crypto.PubKey, error) {
	err := writeMsg(conn, &PubKeyRequest{})
	if err != nil {
		return nil, err
	}

	res, err := readMsg(conn)
	if err != nil {
		return nil, err
	}

	pubKeyResp, ok := res.(*PubKeyResponse)
	if !ok {
		return nil, errors.Wrap(ErrUnexpectedResponse, "response is not PubKeyResponse")
	}

	if pubKeyResp.Error != nil {
		return nil, errors.Wrap(pubKeyResp.Error, "failed to get private validator's public key")
	}

	return pubKeyResp.PubKey, nil
}

// SignVote implements PrivValidator.
func (sc *SignerRemote) SignVote(chainID string, vote *types.Vote) error {
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
func (sc *SignerRemote) SignProposal(chainID string, proposal *types.Proposal) error {
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
func (sc *SignerRemote) Ping() error {
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
	case *PubKeyRequest:
		var p crypto.PubKey
		p = privVal.GetPubKey()
		res = &PubKeyResponse{p, nil}

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
