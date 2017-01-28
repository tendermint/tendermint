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

	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/go-common"
)

const (
	OK  = types.CodeType_OK
	LOG = ""
)

const reqQueueSize = 256        // TODO make configurable
const maxResponseSize = 1048576 // 1MB TODO make configurable
const flushThrottleMS = 20      // Don't wait longer than...

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

func NewSocketClient(addr string, mustConnect bool) (*socketClient, error) {
	cli := &socketClient{
		reqQueue:    make(chan *ReqRes, reqQueueSize),
		flushTimer:  cmn.NewThrottleTimer("socketClient", flushThrottleMS),
		mustConnect: mustConnect,

		addr:    addr,
		reqSent: list.New(),
		resCb:   nil,
	}
	cli.BaseService = *cmn.NewBaseService(nil, "socketClient", cli)

	_, err := cli.Start() // Just start it, it's confusing for callers to remember to start.
	return cli, err
}

func (cli *socketClient) OnStart() error {
	cli.BaseService.OnStart()

	var err error
	var conn net.Conn
RETRY_LOOP:
	for {
		conn, err = cmn.Connect(cli.addr)
		if err != nil {
			if cli.mustConnect {
				return err
			}
			log.Warn(fmt.Sprintf("abci.socketClient failed to connect to %v.  Retrying...", cli.addr))
			time.Sleep(time.Second * 3)
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
	cli.mtx.Lock()
	if !cli.IsRunning() {
		return
	}

	if cli.err == nil {
		cli.err = err
	}
	cli.mtx.Unlock()

	log.Warn(fmt.Sprintf("Stopping abci.socketClient for error: %v", err.Error()))
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
	defer cli.mtx.Unlock()
	cli.resCb = resCb
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
		case <-cli.BaseService.Quit:
			return
		case reqres := <-cli.reqQueue:
			cli.willSendReq(reqres)
			err := types.WriteMessage(reqres.Request, w)
			if err != nil {
				cli.StopForError(fmt.Errorf("Error writing msg: %v", err))
				return
			}
			// log.Debug("Sent request", "requestType", reflect.TypeOf(reqres.Request), "request", reqres.Request)
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
			// log.Debug("Received response", "responseType", reflect.TypeOf(res), "response", res)
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

func (cli *socketClient) InfoAsync() *ReqRes {
	return cli.queueRequest(types.ToRequestInfo())
}

func (cli *socketClient) SetOptionAsync(key string, value string) *ReqRes {
	return cli.queueRequest(types.ToRequestSetOption(key, value))
}

func (cli *socketClient) DeliverTxAsync(tx []byte) *ReqRes {
	return cli.queueRequest(types.ToRequestDeliverTx(tx))
}

func (cli *socketClient) CheckTxAsync(tx []byte) *ReqRes {
	return cli.queueRequest(types.ToRequestCheckTx(tx))
}

func (cli *socketClient) QueryAsync(reqQuery types.RequestQuery) *ReqRes {
	return cli.queueRequest(types.ToRequestQuery(reqQuery))
}

func (cli *socketClient) CommitAsync() *ReqRes {
	return cli.queueRequest(types.ToRequestCommit())
}

func (cli *socketClient) InitChainAsync(validators []*types.Validator) *ReqRes {
	return cli.queueRequest(types.ToRequestInitChain(validators))
}

func (cli *socketClient) BeginBlockAsync(hash []byte, header *types.Header) *ReqRes {
	return cli.queueRequest(types.ToRequestBeginBlock(hash, header))
}

func (cli *socketClient) EndBlockAsync(height uint64) *ReqRes {
	return cli.queueRequest(types.ToRequestEndBlock(height))
}

//----------------------------------------

func (cli *socketClient) EchoSync(msg string) (res types.Result) {
	reqres := cli.queueRequest(types.ToRequestEcho(msg))
	cli.FlushSync()
	if err := cli.Error(); err != nil {
		return types.ErrInternalError.SetLog(err.Error())
	}
	resp := reqres.Response.GetEcho()
	return types.Result{Code: OK, Data: []byte(resp.Message)}
}

func (cli *socketClient) FlushSync() error {
	reqRes := cli.queueRequest(types.ToRequestFlush())
	if err := cli.Error(); err != nil {
		return types.ErrInternalError.SetLog(err.Error())
	}
	reqRes.Wait() // NOTE: if we don't flush the queue, its possible to get stuck here
	return cli.Error()
}

func (cli *socketClient) InfoSync() (resInfo types.ResponseInfo, err error) {
	reqres := cli.queueRequest(types.ToRequestInfo())
	cli.FlushSync()
	if err := cli.Error(); err != nil {
		return resInfo, err
	}
	if resInfo_ := reqres.Response.GetInfo(); resInfo_ != nil {
		return *resInfo_, nil
	}
	return resInfo, nil
}

func (cli *socketClient) SetOptionSync(key string, value string) (res types.Result) {
	reqres := cli.queueRequest(types.ToRequestSetOption(key, value))
	cli.FlushSync()
	if err := cli.Error(); err != nil {
		return types.ErrInternalError.SetLog(err.Error())
	}
	resp := reqres.Response.GetSetOption()
	return types.Result{Code: OK, Data: nil, Log: resp.Log}
}

func (cli *socketClient) DeliverTxSync(tx []byte) (res types.Result) {
	reqres := cli.queueRequest(types.ToRequestDeliverTx(tx))
	cli.FlushSync()
	if err := cli.Error(); err != nil {
		return types.ErrInternalError.SetLog(err.Error())
	}
	resp := reqres.Response.GetDeliverTx()
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *socketClient) CheckTxSync(tx []byte) (res types.Result) {
	reqres := cli.queueRequest(types.ToRequestCheckTx(tx))
	cli.FlushSync()
	if err := cli.Error(); err != nil {
		return types.ErrInternalError.SetLog(err.Error())
	}
	resp := reqres.Response.GetCheckTx()
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *socketClient) QuerySync(reqQuery types.RequestQuery) (resQuery types.ResponseQuery, err error) {
	reqres := cli.queueRequest(types.ToRequestQuery(reqQuery))
	cli.FlushSync()
	if err := cli.Error(); err != nil {
		return resQuery, err
	}
	if resQuery_ := reqres.Response.GetQuery(); resQuery_ != nil {
		return *resQuery_, nil
	}
	return resQuery, nil
}

func (cli *socketClient) CommitSync() (res types.Result) {
	reqres := cli.queueRequest(types.ToRequestCommit())
	cli.FlushSync()
	if err := cli.Error(); err != nil {
		return types.ErrInternalError.SetLog(err.Error())
	}
	resp := reqres.Response.GetCommit()
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *socketClient) InitChainSync(validators []*types.Validator) (err error) {
	cli.queueRequest(types.ToRequestInitChain(validators))
	cli.FlushSync()
	if err := cli.Error(); err != nil {
		return err
	}
	return nil
}

func (cli *socketClient) BeginBlockSync(hash []byte, header *types.Header) (err error) {
	cli.queueRequest(types.ToRequestBeginBlock(hash, header))
	cli.FlushSync()
	if err := cli.Error(); err != nil {
		return err
	}
	return nil
}

func (cli *socketClient) EndBlockSync(height uint64) (resEndBlock types.ResponseEndBlock, err error) {
	reqres := cli.queueRequest(types.ToRequestEndBlock(height))
	cli.FlushSync()
	if err := cli.Error(); err != nil {
		return resEndBlock, err
	}
	if blk := reqres.Response.GetEndBlock(); blk != nil {
		return *blk, nil
	}
	return resEndBlock, nil
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
