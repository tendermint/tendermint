package privval

import (
	"fmt"
	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
	"net"
	"sync"
	"time"
)

// RemoteSignerClient implements PrivValidator, it uses a socket to request signatures
// from an external process.
type RemoteSignerClient struct {
	cmn.BaseService

	connHeartbeat time.Duration

	conn       net.Conn
	lock       sync.Mutex
	cancelPing chan bool
}

// Check that RemoteSignerClient implements PrivValidator.
var _ types.PrivValidator = (*RemoteSignerClient)(nil)

// NewRemoteSignerClient returns an instance of RemoteSignerClient.
func NewRemoteSignerClient(
	logger log.Logger,
	conn net.Conn,
) *RemoteSignerClient {
	sc := &RemoteSignerClient{
		conn:          conn,
		connHeartbeat: connHeartbeat,
	}

	sc.BaseService = *cmn.NewBaseService(logger, "RemoteSignerClient", sc)

	return sc
}

// GetAddress implements PrivValidator.
func (sc *RemoteSignerClient) GetAddress() types.Address {
	addr, err := sc.getAddress()
	if err != nil {
		panic(err)
	}

	return addr
}

// Address is an alias for PubKey().Address().
func (sc *RemoteSignerClient) getAddress() (cmn.HexBytes, error) {
	p, err := sc.getPubKey()
	if err != nil {
		return nil, err
	}

	return p.Address(), nil
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
		return fmt.Errorf("remote error occurred: code: %v, description: %s",
			resp.Error.Code,
			resp.Error.Description)
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
		return fmt.Errorf("remote error occurred: code: %v, description: %s",
			resp.Error.Code,
			resp.Error.Description)
	}
	*proposal = *resp.Proposal

	return nil
}

// SignHeartbeat implements PrivValidator.
func (sc *RemoteSignerClient) SignHeartbeat(
	chainID string,
	heartbeat *types.Heartbeat,
) error {
	sc.lock.Lock()
	defer sc.lock.Unlock()

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

// OnStart implements cmn.Service.
func (sc *RemoteSignerClient) OnStart() error {
	// Start a routine to keep the connection alive
	sc.cancelPing = make(chan bool, 1)
	go func() {
		for {
			select {
			case <-time.Tick(sc.connHeartbeat):
				err := sc.Ping()
				if err != nil {
					sc.Logger.Error(
						"Ping",
						"err", err,
					)
				}
			case <-sc.cancelPing:
				return
			}
		}
	}()

	return nil
}

// OnStop implements cmn.Service.
func (sc *RemoteSignerClient) OnStop() {
	if sc.cancelPing != nil {
		select {
		case sc.cancelPing <- true:
		default:
		}
	}
}
