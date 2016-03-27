package tmspcli

import (
	"bufio"
	"container/list"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/types"
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

func NewClient(addr string, mustConnect bool) (*remoteClient, error) {
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
	if mustConnect {
		return nil, err
	} else {
		return cli, nil
	}
}

func (cli *remoteClient) OnStart() (err error) {
	cli.QuitService.OnStart()
	doneCh := make(chan struct{})
	go func() {
	RETRY_LOOP:
		for {
			conn, err_ := Connect(cli.addr)
			if err_ != nil {
				if cli.mustConnect {
					err = err_ // OnStart() will return this.
					close(doneCh)
					return
				} else {
					fmt.Printf("tmsp.remoteClient failed to connect to %v.  Retrying...\n", cli.addr)
					time.Sleep(time.Second * 3)
					continue RETRY_LOOP
				}
			}
			go cli.sendRequestsRoutine(conn)
			go cli.recvResponseRoutine(conn)
			close(doneCh) // OnStart() will return no error.
			return
		}
	}()
	<-doneCh
	return // err
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
			case cli.reqQueue <- NewReqRes(types.RequestFlush()):
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
			if reqres.Request.Type == types.MessageType_Flush {
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
		switch res.Type {
		case types.MessageType_Exception:
			// XXX After setting cli.err, release waiters (e.g. reqres.Done())
			cli.StopForError(errors.New(res.Error))
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
		return fmt.Errorf("Unexpected result type %v when nothing expected", res.Type)
	}
	reqres := next.Value.(*ReqRes)
	if !resMatchesReq(reqres.Request, res) {
		return fmt.Errorf("Unexpected result type %v when response to %v expected",
			res.Type, reqres.Request.Type)
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
	return cli.queueRequest(types.RequestEcho(msg))
}

func (cli *remoteClient) FlushAsync() *ReqRes {
	return cli.queueRequest(types.RequestFlush())
}

func (cli *remoteClient) InfoAsync() *ReqRes {
	return cli.queueRequest(types.RequestInfo())
}

func (cli *remoteClient) SetOptionAsync(key string, value string) *ReqRes {
	return cli.queueRequest(types.RequestSetOption(key, value))
}

func (cli *remoteClient) AppendTxAsync(tx []byte) *ReqRes {
	return cli.queueRequest(types.RequestAppendTx(tx))
}

func (cli *remoteClient) CheckTxAsync(tx []byte) *ReqRes {
	return cli.queueRequest(types.RequestCheckTx(tx))
}

func (cli *remoteClient) QueryAsync(query []byte) *ReqRes {
	return cli.queueRequest(types.RequestQuery(query))
}

func (cli *remoteClient) CommitAsync() *ReqRes {
	return cli.queueRequest(types.RequestCommit())
}

func (cli *remoteClient) InitChainAsync(validators []*types.Validator) *ReqRes {
	return cli.queueRequest(types.RequestInitChain(validators))
}

func (cli *remoteClient) BeginBlockAsync(height uint64) *ReqRes {
	return cli.queueRequest(types.RequestBeginBlock(height))
}

func (cli *remoteClient) EndBlockAsync(height uint64) *ReqRes {
	return cli.queueRequest(types.RequestEndBlock(height))
}

//----------------------------------------

func (cli *remoteClient) EchoSync(msg string) (res types.Result) {
	reqres := cli.queueRequest(types.RequestEcho(msg))
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *remoteClient) FlushSync() error {
	cli.queueRequest(types.RequestFlush()).Wait()
	return cli.err
}

func (cli *remoteClient) InfoSync() (res types.Result) {
	reqres := cli.queueRequest(types.RequestInfo())
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *remoteClient) SetOptionSync(key string, value string) (res types.Result) {
	reqres := cli.queueRequest(types.RequestSetOption(key, value))
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *remoteClient) AppendTxSync(tx []byte) (res types.Result) {
	reqres := cli.queueRequest(types.RequestAppendTx(tx))
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *remoteClient) CheckTxSync(tx []byte) (res types.Result) {
	reqres := cli.queueRequest(types.RequestCheckTx(tx))
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *remoteClient) QuerySync(query []byte) (res types.Result) {
	reqres := cli.queueRequest(types.RequestQuery(query))
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *remoteClient) CommitSync() (res types.Result) {
	reqres := cli.queueRequest(types.RequestCommit())
	cli.FlushSync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *remoteClient) InitChainSync(validators []*types.Validator) (err error) {
	cli.queueRequest(types.RequestInitChain(validators))
	cli.FlushSync()
	if cli.err != nil {
		return cli.err
	}
	return nil
}

func (cli *remoteClient) BeginBlockSync(height uint64) (err error) {
	cli.queueRequest(types.RequestBeginBlock(height))
	cli.FlushSync()
	if cli.err != nil {
		return cli.err
	}
	return nil
}

func (cli *remoteClient) EndBlockSync(height uint64) (validators []*types.Validator, err error) {
	reqres := cli.queueRequest(types.RequestEndBlock(height))
	cli.FlushSync()
	if cli.err != nil {
		return nil, cli.err
	}
	return reqres.Response.Validators, nil
}

//----------------------------------------

func (cli *remoteClient) queueRequest(req *types.Request) *ReqRes {
	reqres := NewReqRes(req)
	// TODO: set cli.err if reqQueue times out
	cli.reqQueue <- reqres

	// Maybe auto-flush, or unset auto-flush
	switch req.Type {
	case types.MessageType_Flush:
		cli.flushTimer.Unset()
	default:
		cli.flushTimer.Set()
	}

	return reqres
}

//----------------------------------------

func resMatchesReq(req *types.Request, res *types.Response) (ok bool) {
	return req.Type == res.Type
}
