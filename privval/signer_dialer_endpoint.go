package privval

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

const (
	defaultMaxDialRetries = 10
)

// SignerServiceEndpointOption sets an optional parameter on the SignerDialerEndpoint.
type SignerServiceEndpointOption func(*SignerDialerEndpoint)

// SignerServiceEndpointTimeoutReadWrite sets the read and write timeout for connections
// from external signing processes.
func SignerServiceEndpointTimeoutReadWrite(timeout time.Duration) SignerServiceEndpointOption {
	return func(ss *SignerDialerEndpoint) { ss.timeoutReadWrite = timeout }
}

// SignerServiceEndpointConnRetries sets the amount of attempted retries to AcceptNewConnection.
func SignerServiceEndpointConnRetries(retries int) SignerServiceEndpointOption {
	return func(ss *SignerDialerEndpoint) { ss.maxConnRetries = retries }
}

// TODO: Create a common type for a signerEndpoint (common for both listener/dialer)
// getConnection
// AcceptNewConnection
// read
// write
// close

// SignerDialerEndpoint dials using its dialer and responds to any
// signature requests using its privVal.
type SignerDialerEndpoint struct {
	cmn.BaseService

	mtx    sync.Mutex
	dialer SocketDialer
	conn   net.Conn

	timeoutReadWrite time.Duration
	maxConnRetries   int

	chainID string
	privVal types.PrivValidator

	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewSignerDialerEndpoint returns a SignerDialerEndpoint that will dial using the given
// dialer and respond to any signature requests over the connection
// using the given privVal.
func NewSignerDialerEndpoint(
	logger log.Logger,
	chainID string,
	privVal types.PrivValidator,
	dialer SocketDialer,
) *SignerDialerEndpoint {

	se := &SignerDialerEndpoint{
		dialer:           dialer,
		timeoutReadWrite: defaultTimeoutReadWriteSeconds * time.Second,
		maxConnRetries:   defaultMaxDialRetries,

		chainID: chainID,
		privVal: privVal,
	}

	se.BaseService = *cmn.NewBaseService(logger, "SignerDialerEndpoint", se)
	return se
}

// OnStart implements cmn.Service.
func (ss *SignerDialerEndpoint) OnStart() error {
	ss.Logger.Debug("SignerDialerEndpoint: OnStart")

	ss.stopCh = make(chan struct{})
	ss.stoppedCh = make(chan struct{})

	go ss.serviceLoop()

	ss.Logger.Debug("OnStart - done")
	return nil
}

// OnStop implements cmn.Service.
func (ss *SignerDialerEndpoint) OnStop() {
	// Trigger a stop and wait
	close(ss.stopCh)
	<-ss.stoppedCh

	if ss.conn != nil {
		if err := ss.conn.Close(); err != nil {
			ss.Logger.Error("OnStop", "err", cmn.ErrorWrap(err, "closing listener failed"))
			ss.conn = nil
		}
	}
}

func (ss *SignerDialerEndpoint) serviceLoop() {
	defer close(ss.stoppedCh)

	retries := 0
	var err error = nil

	for {
		select {
		default:
			{
				if retries > ss.maxConnRetries {
					ss.Logger.Error("Maximum retries reached", "retries", retries)
					return
				}

				if ss.conn == nil {
					ss.conn, err = ss.dialer()
					if err != nil {
						ss.Logger.Error("SignerDialerEndpoint::serviceLoop", "err", err)
						retries += 1
						continue
					}
				}

				retries = 0
				ss.handleRequest()
			}

		case <-ss.stopCh:
			{
				return
			}
		}
	}
}

func (ss *SignerDialerEndpoint) readMessage() (msg RemoteSignerMsg, err error) {
	// TODO: Avoid duplication. Unify endpoints
	if ss.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(ss.timeoutReadWrite)
	ss.Logger.Debug("SignerDialerEndpoint: readMessage", "deadline", deadline)
	err = ss.conn.SetReadDeadline(deadline)
	if err != nil {
		return
	}

	const maxRemoteSignerMsgSize = 1024 * 10
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(ss.conn, &msg, maxRemoteSignerMsgSize)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrDialerTimeout, err.Error())
	}

	return
}

func (ss *SignerDialerEndpoint) writeMessage(msg RemoteSignerMsg) (err error) {
	// TODO: Avoid duplication. Unify endpoints

	if ss.conn == nil {
		return fmt.Errorf("not connected")
	}

	// Reset read deadline
	deadline := time.Now().Add(ss.timeoutReadWrite)
	ss.Logger.Debug("SignerDialerEndpoint: readMessage", "deadline", deadline)
	err = ss.conn.SetWriteDeadline(deadline)
	if err != nil {
		return
	}

	_, err = cdc.MarshalBinaryLengthPrefixedWriter(ss.conn, msg)
	if _, ok := err.(timeoutError); ok {
		err = cmn.ErrorWrap(ErrDialerTimeout, err.Error())
	}

	return
}

func (ss *SignerDialerEndpoint) handleRequest() {
	if !ss.IsRunning() {
		return // Ignore error from listener closing.
	}

	ss.Logger.Info("SignerDialerEndpoint: connected", "timeout", ss.timeoutReadWrite)

	req, err := ss.readMessage()
	if err != nil {
		if err != io.EOF {
			ss.Logger.Error("SignerDialerEndpoint handleMessage", "err", err)
		}
		return
	}

	res, err := handleMessage(req, ss.chainID, ss.privVal)

	if err != nil {
		// only log the error; we'll reply with an error in res
		ss.Logger.Error("handleMessage handleMessage", "err", err)
	}

	err = ss.writeMessage(res)
	if err != nil {
		ss.Logger.Error("handleMessage writeMessage", "err", err)
		return
	}
}

func handleMessage(req RemoteSignerMsg, chainID string, privVal types.PrivValidator) (RemoteSignerMsg, error) {
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

	default:
		err = fmt.Errorf("unknown msg: %v", r)
	}

	return res, err
}
