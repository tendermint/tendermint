package tmspcli

import (
	"bufio"
	"container/list"
	"errors"
	"fmt"
	"net"
	"sync"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/types"
)

const maxResponseSize = 1048576 // 1MB TODO make configurable
const flushThrottleMS = 20      // Don't wait longer than...

type Callback func(*types.Request, *types.Response)

// This is goroutine-safe, but users should beware that
// the application in general is not meant to be interfaced
// with concurrent callers.
type TMSPClient struct {
	QuitService
	sync.Mutex // [EB]: is this even used?

	reqQueue   chan *reqRes
	flushTimer *ThrottleTimer

	mtx       sync.Mutex
	conn      net.Conn
	bufWriter *bufio.Writer
	err       error
	reqSent   *list.List
	resCb     func(*types.Request, *types.Response)
}

func NewTMSPClient(conn net.Conn, bufferSize int) *TMSPClient {
	cli := &TMSPClient{
		reqQueue:   make(chan *reqRes, bufferSize),
		flushTimer: NewThrottleTimer("TMSPClient", flushThrottleMS),

		conn:      conn,
		bufWriter: bufio.NewWriter(conn),
		reqSent:   list.New(),
		resCb:     nil,
	}
	cli.QuitService = *NewQuitService(nil, "TMSPClient", cli)
	cli.Start() // Just start it, it's confusing for callers to remember to start.
	return cli
}

func (cli *TMSPClient) OnStart() error {
	cli.QuitService.OnStart()
	go cli.sendRequestsRoutine()
	go cli.recvResponseRoutine()
	return nil
}

func (cli *TMSPClient) OnStop() {
	cli.QuitService.OnStop()
	cli.conn.Close()
}

// NOTE: callback may get internally generated flush responses.
func (cli *TMSPClient) SetResponseCallback(resCb Callback) {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	cli.resCb = resCb
}

func (cli *TMSPClient) StopForError(err error) {
	cli.mtx.Lock()
	// log.Error("Stopping TMSPClient for error.", "error", err)
	if cli.err == nil {
		cli.err = err
	}
	cli.mtx.Unlock()
	cli.Stop()
}

func (cli *TMSPClient) Error() error {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	return cli.err
}

//----------------------------------------

func (cli *TMSPClient) sendRequestsRoutine() {
	for {
		select {
		case <-cli.flushTimer.Ch:
			select {
			case cli.reqQueue <- newReqRes(types.RequestFlush()):
			default:
				// Probably will fill the buffer, or retry later.
			}
		case <-cli.QuitService.Quit:
			return
		case reqres := <-cli.reqQueue:
			cli.willSendReq(reqres)
			err := types.WriteMessage(reqres.Request, cli.bufWriter)
			if err != nil {
				cli.StopForError(err)
				return
			}
			// log.Debug("Sent request", "requestType", reflect.TypeOf(reqres.Request), "request", reqres.Request)
			if reqres.Request.Type == types.MessageType_Flush {
				err = cli.bufWriter.Flush()
				if err != nil {
					cli.StopForError(err)
					return
				}
			}
		}
	}
}

func (cli *TMSPClient) recvResponseRoutine() {
	r := bufio.NewReader(cli.conn) // Buffer reads
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

func (cli *TMSPClient) willSendReq(reqres *reqRes) {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	cli.reqSent.PushBack(reqres)
}

func (cli *TMSPClient) didRecvResponse(res *types.Response) error {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()

	// Get the first reqRes
	next := cli.reqSent.Front()
	if next == nil {
		return fmt.Errorf("Unexpected result type %v when nothing expected", res.Type)
	}
	reqres := next.Value.(*reqRes)
	if !resMatchesReq(reqres.Request, res) {
		return fmt.Errorf("Unexpected result type %v when response to %v expected",
			res.Type, reqres.Request.Type)
	}

	reqres.Response = res    // Set response
	reqres.Done()            // Release waiters
	cli.reqSent.Remove(next) // Pop first item from linked list

	// Callback if there is a listener
	if cli.resCb != nil {
		cli.resCb(reqres.Request, res)
	}

	return nil
}

//----------------------------------------

func (cli *TMSPClient) EchoAsync(msg string) {
	cli.queueRequest(types.RequestEcho(msg))
}

func (cli *TMSPClient) FlushAsync() {
	cli.queueRequest(types.RequestFlush())
}

func (cli *TMSPClient) SetOptionAsync(key string, value string) {
	cli.queueRequest(types.RequestSetOption(key, value))
}

func (cli *TMSPClient) AppendTxAsync(tx []byte) {
	cli.queueRequest(types.RequestAppendTx(tx))
}

func (cli *TMSPClient) CheckTxAsync(tx []byte) {
	cli.queueRequest(types.RequestCheckTx(tx))
}

func (cli *TMSPClient) GetHashAsync() {
	cli.queueRequest(types.RequestGetHash())
}

func (cli *TMSPClient) QueryAsync(query []byte) {
	cli.queueRequest(types.RequestQuery(query))
}

//----------------------------------------

func (cli *TMSPClient) InfoSync() (info string, err error) {
	reqres := cli.queueRequest(types.RequestInfo())
	cli.FlushSync()
	if cli.err != nil {
		return "", cli.err
	}
	return string(reqres.Response.Data), nil
}

func (cli *TMSPClient) FlushSync() error {
	cli.queueRequest(types.RequestFlush()).Wait()
	return cli.err
}

func (cli *TMSPClient) AppendTxSync(tx []byte) (code types.CodeType, result []byte, log string, err error) {
	reqres := cli.queueRequest(types.RequestAppendTx(tx))
	cli.FlushSync()
	if cli.err != nil {
		return types.CodeType_InternalError, nil, "", cli.err
	}
	res := reqres.Response
	return res.Code, res.Data, res.Log, nil
}

func (cli *TMSPClient) CheckTxSync(tx []byte) (code types.CodeType, result []byte, log string, err error) {
	reqres := cli.queueRequest(types.RequestCheckTx(tx))
	cli.FlushSync()
	if cli.err != nil {
		return types.CodeType_InternalError, nil, "", cli.err
	}
	res := reqres.Response
	return res.Code, res.Data, res.Log, nil
}

func (cli *TMSPClient) GetHashSync() (hash []byte, log string, err error) {
	reqres := cli.queueRequest(types.RequestGetHash())
	cli.FlushSync()
	if cli.err != nil {
		return nil, "", cli.err
	}
	res := reqres.Response
	return res.Data, res.Log, nil
}

func (cli *TMSPClient) QuerySync(query []byte) (code types.CodeType, result []byte, log string, err error) {
	reqres := cli.queueRequest(types.RequestQuery(query))
	cli.FlushSync()
	if cli.err != nil {
		return types.CodeType_InternalError, nil, "", cli.err
	}
	res := reqres.Response
	return res.Code, res.Data, res.Log, nil
}

//----------------------------------------

func (cli *TMSPClient) queueRequest(req *types.Request) *reqRes {
	reqres := newReqRes(req)
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

type reqRes struct {
	*types.Request
	*sync.WaitGroup
	*types.Response // Not set atomically, so be sure to use WaitGroup.
}

func newReqRes(req *types.Request) *reqRes {
	return &reqRes{
		Request:   req,
		WaitGroup: waitGroup1(),
		Response:  nil,
	}
}

func waitGroup1() (wg *sync.WaitGroup) {
	wg = &sync.WaitGroup{}
	wg.Add(1)
	return
}
