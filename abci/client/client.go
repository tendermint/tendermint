package abcicli

import (
	"context"
	"fmt"
	"sync"

	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/pkg/abci"
)

const (
	dialRetryIntervalSeconds = 3
	echoRetryIntervalSeconds = 1
)

//go:generate ../../scripts/mockery_generate.sh Client

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
	InfoAsync(context.Context, abci.RequestInfo) (*ReqRes, error)
	DeliverTxAsync(context.Context, abci.RequestDeliverTx) (*ReqRes, error)
	CheckTxAsync(context.Context, abci.RequestCheckTx) (*ReqRes, error)
	QueryAsync(context.Context, abci.RequestQuery) (*ReqRes, error)
	CommitAsync(context.Context) (*ReqRes, error)
	InitChainAsync(context.Context, abci.RequestInitChain) (*ReqRes, error)
	BeginBlockAsync(context.Context, abci.RequestBeginBlock) (*ReqRes, error)
	EndBlockAsync(context.Context, abci.RequestEndBlock) (*ReqRes, error)
	ListSnapshotsAsync(context.Context, abci.RequestListSnapshots) (*ReqRes, error)
	OfferSnapshotAsync(context.Context, abci.RequestOfferSnapshot) (*ReqRes, error)
	LoadSnapshotChunkAsync(context.Context, abci.RequestLoadSnapshotChunk) (*ReqRes, error)
	ApplySnapshotChunkAsync(context.Context, abci.RequestApplySnapshotChunk) (*ReqRes, error)

	// Synchronous requests
	FlushSync(context.Context) error
	EchoSync(ctx context.Context, msg string) (*abci.ResponseEcho, error)
	InfoSync(context.Context, abci.RequestInfo) (*abci.ResponseInfo, error)
	DeliverTxSync(context.Context, abci.RequestDeliverTx) (*abci.ResponseDeliverTx, error)
	CheckTxSync(context.Context, abci.RequestCheckTx) (*abci.ResponseCheckTx, error)
	QuerySync(context.Context, abci.RequestQuery) (*abci.ResponseQuery, error)
	CommitSync(context.Context) (*abci.ResponseCommit, error)
	InitChainSync(context.Context, abci.RequestInitChain) (*abci.ResponseInitChain, error)
	BeginBlockSync(context.Context, abci.RequestBeginBlock) (*abci.ResponseBeginBlock, error)
	EndBlockSync(context.Context, abci.RequestEndBlock) (*abci.ResponseEndBlock, error)
	ListSnapshotsSync(context.Context, abci.RequestListSnapshots) (*abci.ResponseListSnapshots, error)
	OfferSnapshotSync(context.Context, abci.RequestOfferSnapshot) (*abci.ResponseOfferSnapshot, error)
	LoadSnapshotChunkSync(context.Context, abci.RequestLoadSnapshotChunk) (*abci.ResponseLoadSnapshotChunk, error)
	ApplySnapshotChunkSync(context.Context, abci.RequestApplySnapshotChunk) (*abci.ResponseApplySnapshotChunk, error)
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

type Callback func(*abci.Request, *abci.Response)

type ReqRes struct {
	*abci.Request
	*sync.WaitGroup
	*abci.Response // Not set atomically, so be sure to use WaitGroup.

	mtx  tmsync.RWMutex
	done bool                 // Gets set to true once *after* WaitGroup.Done().
	cb   func(*abci.Response) // A single callback that may be set.
}

func NewReqRes(req *abci.Request) *ReqRes {
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
func (r *ReqRes) SetCallback(cb func(res *abci.Response)) {
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
func (r *ReqRes) GetCallback() func(*abci.Response) {
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
