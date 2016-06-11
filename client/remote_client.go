package tmspcli

import (
	"bufio"
	"container/list"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/types"
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
type remoteClient struct {
	QuitService
	sync.Mutex // [EB]: is this even used?

	reqQueue    chan *ReqRes
	flushTimer  *ThrottleTimer
	mustConnect bool

	mtx     sync.Mutex
	addr    string
	conn    net.Conn
	err     error
	reqSent *list.List
	resCb   func(*types.Request, *types.Response) // listens to all callbacks
}

func NewSocketClient(addr string, mustConnect bool) (*remoteClient, error) {
	cli := &remoteClient{
		reqQueue:    make(chan *ReqRes, reqQueueSize),
		flushTimer:  NewThrottleTimer("remoteClient", flushThrottleMS),
		mustConnect: mustConnect,

		addr:    addr,
		reqSent: list.New(),
		resCb:   nil,
	}
	cli.QuitService = *NewQuitService(nil, "remoteClient", cli)
	_, err := cli.Start() // Just start it, it's confusing for callers to remember to start.
	return cli, err
}

func (cli *remoteClient) OnStart() error {
	cli.QuitService.OnStart()
RETRY_LOOP:
	for {
		conn, err := Connect(cli.addr)
		if err != nil {
			if cli.mustConnect {
				return err
			} else {
				fmt.Printf("tmsp.remoteClient failed to connect to %v.  Retrying...\n", cli.addr)
				time.Sleep(time.Second * 3)
				continue RETRY_LOOP
			}
		}
		go cli.sendRequestsRoutine(conn)
		go cli.recvResponseRoutine(conn)
		return err
	}
	return nil // never happens
}

func (cli *remoteClient) OnStop() {
	cli.QuitService.OnStop()
	if cli.conn != nil {
		cli.conn.Close()
	}
}

// Set listener for all responses
// NOTE: callback may get internally generated flush responses.
func (cli *remoteClient) SetResponseCallback(resCb Callback) {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	cli.resCb = resCb
}

func (cli *remoteClient) StopForError(err error) {
	cli.mtx.Lock()
	fmt.Printf("Stopping tmsp.remoteClient for error: %v\n", err.Error())
	if cli.err == nil {
		cli.err = err
	}
	cli.mtx.Unlock()
	cli.Stop()
}

func (cli *remoteClient) Error() error {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	return cli.err
}

//----------------------------------------

func (cli *remoteClient) sendRequestsRoutine(conn net.Conn) {
	w := bufio.NewWriter(conn)
	for {
		select {
		case <-cli.flushTimer.Ch:
			select {
			case cli.reqQueue <- NewReqRes(types.ToRequestFlush()): // cant this block ?
			default:
				// Probably will fill the buffer, or retry later.
			}
		case <-cli.QuitService.Quit:
			return
		case reqres := <-cli.reqQueue:
			cli.willSendReq(reqres)
			err := types.WriteMessage(reqres.Request, w)
			if err != nil {
				cli.StopForError(err)
				return
			}
			// log.Debug("Sent request", "requestType", reflect.TypeOf(reqres.Request), "request", reqres.Request)
			if _, ok := reqres.Request.Value.(*types.Request_Flush); ok {
				err = w.Flush()
				if err != nil {
					cli.StopForError(err)
					return
				}
			}
		}
	}
}

func (cli *remoteClient) recvResponseRoutine(conn net.Conn) {
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
		default:
			// log.Debug("Received response", "responseType", reflect.TypeOf(res), "response", res)
			err := cli.didRecvResponse(res)
			if err != nil {
				cli.StopForError(err)
			}
		}
	}
}

func (cli *remoteClient) willSendReq(reqres *ReqRes) {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	cli.reqSent.PushBack(reqres)
}

func (cli *remoteClient) didRecvResponse(res *types.Response) error {
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

func (cli *remoteClient) EchoAsync(msg string) *ReqRes {
	return cli.queueRequest(types.ToRequestEcho(msg))
}

func (cli *remoteClient) FlushAsync() *ReqRes {
	return cli.queueRequest(types.ToRequestFlush())
}

func (cli *remoteClient) InfoAsync() *ReqRes {
	return cli.queueRequest(types.ToRequestInfo())
}

func (cli *remoteClient) SetOptionAsync(key string, value string) *ReqRes {
	return cli.queueRequest(types.ToRequestSetOption(key, value))
}

func (cli *remoteClient) AppendTxAsync(tx []byte) *ReqRes {
	return cli.queueRequest(types.ToRequestAppendTx(tx))
}

func (cli *remoteClient) CheckTxAsync(tx []byte) *ReqRes {
	return cli.queueRequest(types.ToRequestCheckTx(tx))
}

func (cli *remoteClient) QueryAsync(query []byte) *ReqRes {
	return cli.queueRequest(types.ToRequestQuery(query))
}

func (cli *remoteClient) CommitAsync() *ReqRes {
	return cli.queueRequest(types.ToRequestCommit())
}

func (cli *remoteClient) InitChainAsync(validators []*types.Validator) *ReqRes {
	return cli.queueRequest(types.ToRequestInitChain(validators))
}

func (cli *remoteClient) BeginBlockAsync(height uint64) *ReqRes {
	return cli.queueRequest(types.ToRequestBeginBlock(height))
}

func (cli *remoteClient) EndBlockAsync(height uint64) *ReqRes {
	return cli.queueRequest(types.ToRequestEndBlock(height))
}

//----------------------------------------

func (cli *remoteClient) EchoSync(msg string) (res types.Result) {
	reqres := cli.queueRequest(types.ToRequestEcho(msg))
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response.GetEcho()
	return types.Result{Code: OK, Data: []byte(resp.Message), Log: LOG}
}

func (cli *remoteClient) FlushSync() error {
	cli.queueRequest(types.ToRequestFlush()).Wait()
	return cli.err
}

func (cli *remoteClient) InfoSync() (res types.Result) {
	reqres := cli.queueRequest(types.ToRequestInfo())
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response.GetInfo()
	return types.Result{Code: OK, Data: []byte(resp.Info), Log: LOG}
}

func (cli *remoteClient) SetOptionSync(key string, value string) (res types.Result) {
	reqres := cli.queueRequest(types.ToRequestSetOption(key, value))
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response.GetSetOption()
	return types.Result{Code: OK, Data: nil, Log: resp.Log}
}

func (cli *remoteClient) AppendTxSync(tx []byte) (res types.Result) {
	reqres := cli.queueRequest(types.ToRequestAppendTx(tx))
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response.GetAppendTx()
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *remoteClient) CheckTxSync(tx []byte) (res types.Result) {
	reqres := cli.queueRequest(types.ToRequestCheckTx(tx))
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response.GetCheckTx()
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *remoteClient) QuerySync(query []byte) (res types.Result) {
	reqres := cli.queueRequest(types.ToRequestQuery(query))
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response.GetQuery()
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *remoteClient) CommitSync() (res types.Result) {
	reqres := cli.queueRequest(types.ToRequestCommit())
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response.GetCommit()
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *remoteClient) InitChainSync(validators []*types.Validator) (err error) {
	cli.queueRequest(types.ToRequestInitChain(validators))
	cli.FlushSync()
	if cli.err != nil {
		return cli.err
	}
	return nil
}

func (cli *remoteClient) BeginBlockSync(height uint64) (err error) {
	cli.queueRequest(types.ToRequestBeginBlock(height))
	cli.FlushSync()
	if cli.err != nil {
		return cli.err
	}
	return nil
}

func (cli *remoteClient) EndBlockSync(height uint64) (validators []*types.Validator, err error) {
	reqres := cli.queueRequest(types.ToRequestEndBlock(height))
	cli.FlushSync()
	if cli.err != nil {
		return nil, cli.err
	}
	return reqres.Response.GetEndBlock().Diffs, nil
}

//----------------------------------------

func (cli *remoteClient) queueRequest(req *types.Request) *ReqRes {
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
	case *types.Request_AppendTx:
		_, ok = res.Value.(*types.Response_AppendTx)
	case *types.Request_CheckTx:
		_, ok = res.Value.(*types.Response_CheckTx)
	case *types.Request_Commit:
		_, ok = res.Value.(*types.Response_Commit)
	case *types.Request_Query:
		_, ok = res.Value.(*types.Response_Query)
	case *types.Request_InitChain:
		_, ok = res.Value.(*types.Response_InitChain)
	case *types.Request_EndBlock:
		_, ok = res.Value.(*types.Response_EndBlock)
	}
	return ok
}
