package abciclient

import (
	"context"
	"fmt"
	"sync"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

const (
	dialRetryIntervalSeconds = 3
	echoRetryIntervalSeconds = 1
)

//go:generate ../../scripts/mockery_generate.sh Client

// Client defines an interface for an ABCI client.
//
// All methods return the appropriate protobuf ResponseXxx struct and
// an error.
//
// NOTE these are client errors, eg. ABCI socket connectivity issues.
// Application-related errors are reflected in response via ABCI error codes
// and logs.
type Client interface {
	service.Service

	Error() error

	Flush(context.Context) error
	Echo(ctx context.Context, msg string) (*types.ResponseEcho, error)
	Info(context.Context, types.RequestInfo) (*types.ResponseInfo, error)
	CheckTx(context.Context, types.RequestCheckTx) (*types.ResponseCheckTx, error)
	Query(context.Context, types.RequestQuery) (*types.ResponseQuery, error)
	Commit(context.Context) (*types.ResponseCommit, error)
	InitChain(context.Context, types.RequestInitChain) (*types.ResponseInitChain, error)
	PrepareProposal(context.Context, types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error)
	ProcessProposal(context.Context, types.RequestProcessProposal) (*types.ResponseProcessProposal, error)
	ExtendVote(context.Context, types.RequestExtendVote) (*types.ResponseExtendVote, error)
	VerifyVoteExtension(context.Context, types.RequestVerifyVoteExtension) (*types.ResponseVerifyVoteExtension, error)
	FinalizeBlock(context.Context, types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error)
	ListSnapshots(context.Context, types.RequestListSnapshots) (*types.ResponseListSnapshots, error)
	OfferSnapshot(context.Context, types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error)
	LoadSnapshotChunk(context.Context, types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error)
	ApplySnapshotChunk(context.Context, types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error)
}

//----------------------------------------

// NewClient returns a new ABCI client of the specified transport type.
// It returns an error if the transport is not "socket" or "grpc"
func NewClient(logger log.Logger, addr, transport string, mustConnect bool) (Client, error) {
	switch transport {
	case "socket":
		return NewSocketClient(logger, addr, mustConnect), nil
	case "grpc":
		return NewGRPCClient(logger, addr, mustConnect), nil
	default:
		return nil, fmt.Errorf("unknown abci transport %s", transport)
	}
}

type requestAndResponse struct {
	*types.Request
	*types.Response

	mtx    sync.Mutex
	signal chan struct{}
}

func makeReqRes(req *types.Request) *requestAndResponse {
	return &requestAndResponse{
		Request:  req,
		Response: nil,
		signal:   make(chan struct{}),
	}
}

// markDone marks the ReqRes object as done.
func (r *requestAndResponse) markDone() {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	close(r.signal)
}
