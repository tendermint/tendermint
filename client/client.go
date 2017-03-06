package abcicli

import (
	"fmt"
	"sync"

	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/go-common"
)

type Client interface {
	cmn.Service

	SetResponseCallback(Callback)
	Error() error

	FlushAsync() *ReqRes
	EchoAsync(msg string) *ReqRes
	InfoAsync() *ReqRes
	SetOptionAsync(key string, value string) *ReqRes
	DeliverTxAsync(tx []byte) *ReqRes
	CheckTxAsync(tx []byte) *ReqRes
	QueryAsync(reqQuery types.RequestQuery) *ReqRes
	CommitAsync() *ReqRes

	FlushSync() error
	EchoSync(msg string) (res types.Result)
	InfoSync() (resInfo types.ResponseInfo, err error)
	SetOptionSync(key string, value string) (res types.Result)
	DeliverTxSync(tx []byte) (res types.Result)
	CheckTxSync(tx []byte) (res types.Result)
	QuerySync(reqQuery types.RequestQuery) (resQuery types.ResponseQuery, err error)
	CommitSync() (res types.Result)

	InitChainAsync(validators []*types.Validator) *ReqRes
	BeginBlockAsync(hash []byte, header *types.Header) *ReqRes
	EndBlockAsync(height uint64) *ReqRes

	InitChainSync(validators []*types.Validator) (err error)
	BeginBlockSync(hash []byte, header *types.Header) (err error)
	EndBlockSync(height uint64) (resEndBlock types.ResponseEndBlock, err error)
}

//----------------------------------------

func NewClient(addr, transport string, mustConnect bool) (client Client, err error) {
	switch transport {
	case "socket":
		client, err = NewSocketClient(addr, mustConnect)
	case "grpc":
		client, err = NewGRPCClient(addr, mustConnect)
	default:
		err = fmt.Errorf("Unknown abci transport %s", transport)

	}
	return
}

//----------------------------------------

type Callback func(*types.Request, *types.Response)

//----------------------------------------

type ReqRes struct {
	*types.Request
	*sync.WaitGroup
	*types.Response // Not set atomically, so be sure to use WaitGroup.

	mtx  sync.Mutex
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

// Sets the callback for this ReqRes atomically.
// If reqRes is already done, calls cb immediately.
// NOTE: reqRes.cb should not change if reqRes.done.
// NOTE: only one callback is supported.
func (reqRes *ReqRes) SetCallback(cb func(res *types.Response)) {
	reqRes.mtx.Lock()

	if reqRes.done {
		reqRes.mtx.Unlock()
		cb(reqRes.Response)
		return
	}

	defer reqRes.mtx.Unlock()
	reqRes.cb = cb
}

func (reqRes *ReqRes) GetCallback() func(*types.Response) {
	reqRes.mtx.Lock()
	defer reqRes.mtx.Unlock()
	return reqRes.cb
}

// NOTE: it should be safe to read reqRes.cb without locks after this.
func (reqRes *ReqRes) SetDone() {
	reqRes.mtx.Lock()
	reqRes.done = true
	reqRes.mtx.Unlock()
}

func waitGroup1() (wg *sync.WaitGroup) {
	wg = &sync.WaitGroup{}
	wg.Add(1)
	return
}
