package abcicli

import (
	"bufio"
	"container/list"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
)

const reqQueueSize = 256 // TODO make configurable
// const maxResponseSize = 1048576 // 1MB TODO make configurable
const flushThrottleMS = 20 // Don't wait longer than...

var _ Client = (*socketClient)(nil)

// This is goroutine-safe, but users should beware that
// the application in general is not meant to be interfaced
// with concurrent callers.
type socketClient struct {
	cmn.BaseService

	reqQueue    chan *ReqRes
	flushTimer  *cmn.ThrottleTimer
	mustConnect bool

	mtx     sync.Mutex
	addr    string
	conn    net.Conn
	err     error
	reqSent *list.List
	resCb   func(*types.Request, *types.Response) // listens to all callbacks

}

func NewSocketClient(addr string, mustConnect bool) *socketClient {
	cli := &socketClient{
		reqQueue:    make(chan *ReqRes, reqQueueSize),
		flushTimer:  cmn.NewThrottleTimer("socketClient", flushThrottleMS),
		mustConnect: mustConnect,

		addr:    addr,
		reqSent: list.New(),
		resCb:   nil,
	}
	cli.BaseService = *cmn.NewBaseService(nil, "socketClient", cli)
	return cli
}

func (cli *socketClient) OnStart() error {
	if err := cli.BaseService.OnStart(); err != nil {
		return err
	}

	var err error
	var conn net.Conn
RETRY_LOOP:
	for {
		conn, err = cmn.Connect(cli.addr)
		if err != nil {
			if cli.mustConnect {
				return err
			}
			cli.Logger.Error(fmt.Sprintf("abci.socketClient failed to connect to %v.  Retrying...", cli.addr), "err", err)
			time.Sleep(time.Second * dialRetryIntervalSeconds)
			continue RETRY_LOOP
		}
		cli.conn = conn

		go cli.sendRequestsRoutine(conn)
		go cli.recvResponseRoutine(conn)

		return nil
	}
}

func (cli *socketClient) OnStop() {
	cli.BaseService.OnStop()

	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	if cli.conn != nil {
		cli.conn.Close()
	}

	cli.flushQueue()
}

// Stop the client and set the error
func (cli *socketClient) StopForError(err error) {
	if !cli.IsRunning() {
		return
	}

	cli.mtx.Lock()
	if cli.err == nil {
		cli.err = err
	}
	cli.mtx.Unlock()

	cli.Logger.Error(fmt.Sprintf("Stopping abci.socketClient for error: %v", err.Error()))
	cli.Stop()
}

func (cli *socketClient) Error() error {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	return cli.err
}

// Set listener for all responses
// NOTE: callback may get internally generated flush responses.
func (cli *socketClient) SetResponseCallback(resCb Callback) {
	cli.mtx.Lock()
	cli.resCb = resCb
	cli.mtx.Unlock()
}

//----------------------------------------

func (cli *socketClient) sendRequestsRoutine(conn net.Conn) {

	w := bufio.NewWriter(conn)
	for {
		select {
		case <-cli.flushTimer.Ch:
			select {
			case cli.reqQueue <- NewReqRes(types.ToRequestFlush()):
			default:
				// Probably will fill the buffer, or retry later.
			}
		case <-cli.Quit():
			return
		case reqres := <-cli.reqQueue:
			cli.willSendReq(reqres)
			err := types.WriteMessage(reqres.Request, w)
			if err != nil {
				cli.StopForError(fmt.Errorf("Error writing msg: %v", err))
				return
			}
			// cli.Logger.Debug("Sent request", "requestType", reflect.TypeOf(reqres.Request), "request", reqres.Request)
			if _, ok := reqres.Request.Value.(*types.Request_Flush); ok {
				err = w.Flush()
				if err != nil {
					cli.StopForError(fmt.Errorf("Error flushing writer: %v", err))
					return
				}
			}
		}
	}
}

func (cli *socketClient) recvResponseRoutine(conn net.Conn) {

	r := bufio.NewReader(conn) // Buffer reads
	for {
		var res = &types.Response{}
		err := types.ReadMessage(r, res)
		if err != nil {
			cli.StopForError(err)
			return
		}
		switch r := res.Value.(type) {
		case *types.Response_Exception:
			// XXX After setting cli.err, release waiters (e.g. reqres.Done())
			cli.StopForError(errors.New(r.Exception.Error))
			return
		default:
			// cli.Logger.Debug("Received response", "responseType", reflect.TypeOf(res), "response", res)
			err := cli.didRecvResponse(res)
			if err != nil {
				cli.StopForError(err)
				return
			}
		}
	}
}

func (cli *socketClient) willSendReq(reqres *ReqRes) {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	cli.reqSent.PushBack(reqres)
}

func (cli *socketClient) didRecvResponse(res *types.Response) error {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()

	// Get the first ReqRes
	next := cli.reqSent.Front()
	if next == nil {
		return fmt.Errorf("Unexpected result type %v when nothing expected", reflect.TypeOf(res.Value))
	}
	reqres := next.Value.(*ReqRes)
	if !resMatchesReq(reqres.Request, res) {
		return fmt.Errorf("Unexpected result type %v when response to %v expected",
			reflect.TypeOf(res.Value), reflect.TypeOf(reqres.Request.Value))
	}

	reqres.Response = res    // Set response
	reqres.Done()            // Release waiters
	cli.reqSent.Remove(next) // Pop first item from linked list

	// Notify reqRes listener if set
	if cb := reqres.GetCallback(); cb != nil {
		cb(res)
	}

	// Notify client listener if set
	if cli.resCb != nil {
		cli.resCb(reqres.Request, res)
	}

	return nil
}

//----------------------------------------

func (cli *socketClient) EchoAsync(msg string) *ReqRes {
	return cli.queueRequest(types.ToRequestEcho(msg))
}

func (cli *socketClient) FlushAsync() *ReqRes {
	return cli.queueRequest(types.ToRequestFlush())
}

func (cli *socketClient) InfoAsync(req types.RequestInfo) *ReqRes {
	return cli.queueRequest(types.ToRequestInfo(req))
}

func (cli *socketClient) SetOptionAsync(req types.RequestSetOption) *ReqRes {
	return cli.queueRequest(types.ToRequestSetOption(req))
}

func (cli *socketClient) DeliverTxAsync(tx []byte) *ReqRes {
	return cli.queueRequest(types.ToRequestDeliverTx(tx))
}

func (cli *socketClient) CheckTxAsync(tx []byte) *ReqRes {
	return cli.queueRequest(types.ToRequestCheckTx(tx))
}

func (cli *socketClient) QueryAsync(req types.RequestQuery) *ReqRes {
	return cli.queueRequest(types.ToRequestQuery(req))
}

func (cli *socketClient) CommitAsync() *ReqRes {
	return cli.queueRequest(types.ToRequestCommit())
}

func (cli *socketClient) InitChainAsync(req types.RequestInitChain) *ReqRes {
	return cli.queueRequest(types.ToRequestInitChain(req))
}

func (cli *socketClient) BeginBlockAsync(req types.RequestBeginBlock) *ReqRes {
	return cli.queueRequest(types.ToRequestBeginBlock(req))
}

func (cli *socketClient) EndBlockAsync(req types.RequestEndBlock) *ReqRes {
	return cli.queueRequest(types.ToRequestEndBlock(req))
}

//----------------------------------------

func (cli *socketClient) FlushSync() error {
	reqRes := cli.queueRequest(types.ToRequestFlush())
	if err := cli.Error(); err != nil {
		return err
	}
	reqRes.Wait() // NOTE: if we don't flush the queue, its possible to get stuck here
	return cli.Error()
}

func (cli *socketClient) EchoSync(msg string) (*types.ResponseEcho, error) {
	reqres := cli.queueRequest(types.ToRequestEcho(msg))
	cli.FlushSync()
	return reqres.Response.GetEcho(), cli.Error()
}

func (cli *socketClient) InfoSync(req types.RequestInfo) (*types.ResponseInfo, error) {
	reqres := cli.queueRequest(types.ToRequestInfo(req))
	cli.FlushSync()
	return reqres.Response.GetInfo(), cli.Error()
}

func (cli *socketClient) SetOptionSync(req types.RequestSetOption) (*types.ResponseSetOption, error) {
	reqres := cli.queueRequest(types.ToRequestSetOption(req))
	cli.FlushSync()
	return reqres.Response.GetSetOption(), cli.Error()
}

func (cli *socketClient) DeliverTxSync(tx []byte) (*types.ResponseDeliverTx, error) {
	reqres := cli.queueRequest(types.ToRequestDeliverTx(tx))
	cli.FlushSync()
	return reqres.Response.GetDeliverTx(), cli.Error()
}

func (cli *socketClient) CheckTxSync(tx []byte) (*types.ResponseCheckTx, error) {
	reqres := cli.queueRequest(types.ToRequestCheckTx(tx))
	cli.FlushSync()
	return reqres.Response.GetCheckTx(), cli.Error()
}

func (cli *socketClient) QuerySync(req types.RequestQuery) (*types.ResponseQuery, error) {
	reqres := cli.queueRequest(types.ToRequestQuery(req))
	cli.FlushSync()
	return reqres.Response.GetQuery(), cli.Error()
}

func (cli *socketClient) CommitSync() (*types.ResponseCommit, error) {
	reqres := cli.queueRequest(types.ToRequestCommit())
	cli.FlushSync()
	return reqres.Response.GetCommit(), cli.Error()
}

func (cli *socketClient) InitChainSync(req types.RequestInitChain) (*types.ResponseInitChain, error) {
	reqres := cli.queueRequest(types.ToRequestInitChain(req))
	cli.FlushSync()
	return reqres.Response.GetInitChain(), cli.Error()
}

func (cli *socketClient) BeginBlockSync(req types.RequestBeginBlock) (*types.ResponseBeginBlock, error) {
	reqres := cli.queueRequest(types.ToRequestBeginBlock(req))
	cli.FlushSync()
	return reqres.Response.GetBeginBlock(), cli.Error()
}

func (cli *socketClient) EndBlockSync(req types.RequestEndBlock) (*types.ResponseEndBlock, error) {
	reqres := cli.queueRequest(types.ToRequestEndBlock(req))
	cli.FlushSync()
	return reqres.Response.GetEndBlock(), cli.Error()
}

//----------------------------------------

func (cli *socketClient) queueRequest(req *types.Request) *ReqRes {
	reqres := NewReqRes(req)

	// TODO: set cli.err if reqQueue times out
	cli.reqQueue <- reqres

	// Maybe auto-flush, or unset auto-flush
	switch req.Value.(type) {
	case *types.Request_Flush:
		cli.flushTimer.Unset()
	default:
		cli.flushTimer.Set()
	}

	return reqres
}

func (cli *socketClient) flushQueue() {
	// mark all in-flight messages as resolved (they will get cli.Error())
	for req := cli.reqSent.Front(); req != nil; req = req.Next() {
		reqres := req.Value.(*ReqRes)
		reqres.Done()
	}

	// mark all queued messages as resolved
LOOP:
	for {
		select {
		case reqres := <-cli.reqQueue:
			reqres.Done()
		default:
			break LOOP
		}
	}
}

//----------------------------------------

func resMatchesReq(req *types.Request, res *types.Response) (ok bool) {
	switch req.Value.(type) {
	case *types.Request_Echo:
		_, ok = res.Value.(*types.Response_Echo)
	case *types.Request_Flush:
		_, ok = res.Value.(*types.Response_Flush)
	case *types.Request_Info:
		_, ok = res.Value.(*types.Response_Info)
	case *types.Request_SetOption:
		_, ok = res.Value.(*types.Response_SetOption)
	case *types.Request_DeliverTx:
		_, ok = res.Value.(*types.Response_DeliverTx)
	case *types.Request_CheckTx:
		_, ok = res.Value.(*types.Response_CheckTx)
	case *types.Request_Commit:
		_, ok = res.Value.(*types.Response_Commit)
	case *types.Request_Query:
		_, ok = res.Value.(*types.Response_Query)
	case *types.Request_InitChain:
		_, ok = res.Value.(*types.Response_InitChain)
	case *types.Request_BeginBlock:
		_, ok = res.Value.(*types.Response_BeginBlock)
	case *types.Request_EndBlock:
		_, ok = res.Value.(*types.Response_EndBlock)
	}
	return ok
}
