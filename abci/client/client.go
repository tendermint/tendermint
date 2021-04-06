package abcicli

import (
	"context"
	"fmt"
	"sync"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
)

const (
	dialRetryIntervalSeconds = 3
	echoRetryIntervalSeconds = 1
)

//go:generate mockery --case underscore --name Client

// Client defines an interface for an ABCI client.
//
// All `Async` methods return a `ReqRes` object and an error.
// All `Sync` methods return the appropriate protobuf ResponseXxx struct and an error.
//
// NOTE these are client errors, eg. ABCI socket connectivity issues.
// Application-related errors are reflected in response via ABCI error codes
// and logs.
type Client interface {
	service.Service

	SetResponseCallback(Callback)
	Error() error

	// Asynchronous requests
	FlushAsync(context.Context) (*ReqRes, error)
	EchoAsync(ctx context.Context, msg string) (*ReqRes, error)
	InfoAsync(context.Context, types.RequestInfo) (*ReqRes, error)
	DeliverTxAsync(context.Context, types.RequestDeliverTx) (*ReqRes, error)
	CheckTxAsync(context.Context, types.RequestCheckTx) (*ReqRes, error)
	QueryAsync(context.Context, types.RequestQuery) (*ReqRes, error)
	CommitAsync(context.Context) (*ReqRes, error)
	InitChainAsync(context.Context, types.RequestInitChain) (*ReqRes, error)
	BeginBlockAsync(context.Context, types.RequestBeginBlock) (*ReqRes, error)
	EndBlockAsync(context.Context, types.RequestEndBlock) (*ReqRes, error)
	ListSnapshotsAsync(context.Context, types.RequestListSnapshots) (*ReqRes, error)
	OfferSnapshotAsync(context.Context, types.RequestOfferSnapshot) (*ReqRes, error)
	LoadSnapshotChunkAsync(context.Context, types.RequestLoadSnapshotChunk) (*ReqRes, error)
	ApplySnapshotChunkAsync(context.Context, types.RequestApplySnapshotChunk) (*ReqRes, error)

	// Synchronous requests
	FlushSync(context.Context) error
	EchoSync(ctx context.Context, msg string) (*types.ResponseEcho, error)
	InfoSync(context.Context, types.RequestInfo) (*types.ResponseInfo, error)
	DeliverTxSync(context.Context, types.RequestDeliverTx) (*types.ResponseDeliverTx, error)
	CheckTxSync(context.Context, types.RequestCheckTx) (*types.ResponseCheckTx, error)
	QuerySync(context.Context, types.RequestQuery) (*types.ResponseQuery, error)
	CommitSync(context.Context) (*types.ResponseCommit, error)
	InitChainSync(context.Context, types.RequestInitChain) (*types.ResponseInitChain, error)
	BeginBlockSync(context.Context, types.RequestBeginBlock) (*types.ResponseBeginBlock, error)
	EndBlockSync(context.Context, types.RequestEndBlock) (*types.ResponseEndBlock, error)
	ListSnapshotsSync(context.Context, types.RequestListSnapshots) (*types.ResponseListSnapshots, error)
	OfferSnapshotSync(context.Context, types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error)
	LoadSnapshotChunkSync(context.Context, types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error)
	ApplySnapshotChunkSync(context.Context, types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error)
}

//----------------------------------------

// NewClient returns a new ABCI client of the specified transport type.
// It returns an error if the transport is not "socket" or "grpc"
func NewClient(addr, transport string, mustConnect bool) (client Client, err error) {
	switch transport {
	case "socket":
		client = NewSocketClient(addr, mustConnect)
	case "grpc":
		client = NewGRPCClient(addr, mustConnect)
	default:
		err = fmt.Errorf("unknown abci transport %s", transport)
	}
	return
}

type Callback func(*types.Request, *types.Response)

type ReqRes struct {
	*types.Request
	*sync.WaitGroup
	*types.Response // Not set atomically, so be sure to use WaitGroup.

	mtx  tmsync.RWMutex
	done bool                  // Gets set to true once *after* WaitGroup.Done().
	cb   func(*types.Response) // A single callback that may be set.
}

func NewReqRes(req *types.Request) *ReqRes {
	return &ReqRes{
		Request:   req,
		WaitGroup: waitGroup1(),
		Response:  nil,

		done: false,
		cb:   nil,
	}
}

// Sets sets the callback. If reqRes is already done, it will call the cb
// immediately. Note, reqRes.cb should not change if reqRes.done and only one
// callback is supported.
func (r *ReqRes) SetCallback(cb func(res *types.Response)) {
	r.mtx.Lock()

	if r.done {
		r.mtx.Unlock()
		cb(r.Response)
		return
	}

	r.cb = cb
	r.mtx.Unlock()
}

// InvokeCallback invokes a thread-safe execution of the configured callback
// if non-nil.
func (r *ReqRes) InvokeCallback() {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	if r.cb != nil {
		r.cb(r.Response)
	}
}

// GetCallback returns the configured callback of the ReqRes object which may be
// nil. Note, it is not safe to concurrently call this in cases where it is
// marked done and SetCallback is called before calling GetCallback as that
// will invoke the callback twice and create a potential race condition.
//
// ref: https://github.com/tendermint/tendermint/issues/5439
func (r *ReqRes) GetCallback() func(*types.Response) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.cb
}

// SetDone marks the ReqRes object as done.
func (r *ReqRes) SetDone() {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.done = true
}

func waitGroup1() (wg *sync.WaitGroup) {
	wg = &sync.WaitGroup{}
	wg.Add(1)
	return
}
