package tmspcli

import (
	"bufio"
	"container/list"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	tmsp "github.com/tendermint/tmsp/types"
)

const maxResponseSize = 1048576 // 1MB TODO make configurable
const flushThrottleMS = 20      // Don't wait longer than...

type Callback func(tmsp.Request, tmsp.Response)

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
	resCb     func(tmsp.Request, tmsp.Response)
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
		var n int
		var err error
		select {
		case <-cli.flushTimer.Ch:
			select {
			case cli.reqQueue <- newReqRes(tmsp.RequestFlush{}):
			default:
				// Probably will fill the buffer, or retry later.
			}
		case <-cli.QuitService.Quit:
			return
		case reqres := <-cli.reqQueue:
			cli.willSendReq(reqres)
			wire.WriteBinaryLengthPrefixed(struct{ tmsp.Request }{reqres.Request}, cli.bufWriter, &n, &err) // Length prefix
			if err != nil {
				cli.StopForError(err)
				return
			}
			// log.Debug("Sent request", "requestType", reflect.TypeOf(reqres.Request), "request", reqres.Request)
			if _, ok := reqres.Request.(tmsp.RequestFlush); ok {
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
		var res tmsp.Response
		var n int
		var err error
		wire.ReadBinaryPtrLengthPrefixed(&res, r, maxResponseSize, &n, &err)
		if err != nil {
			cli.StopForError(err)
			return
		}
		switch res := res.(type) {
		case tmsp.ResponseException:
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

func (cli *TMSPClient) didRecvResponse(res tmsp.Response) error {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()

	// Get the first reqRes
	next := cli.reqSent.Front()
	if next == nil {
		return fmt.Errorf("Unexpected result type %v when nothing expected", reflect.TypeOf(res))
	}
	reqres := next.Value.(*reqRes)
	if !resMatchesReq(reqres.Request, res) {
		return fmt.Errorf("Unexpected result type %v when response to %v expected",
			reflect.TypeOf(res), reflect.TypeOf(reqres.Request))
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
	cli.queueRequest(tmsp.RequestEcho{msg})
}

func (cli *TMSPClient) FlushAsync() {
	cli.queueRequest(tmsp.RequestFlush{})
}

func (cli *TMSPClient) SetOptionAsync(key string, value string) {
	cli.queueRequest(tmsp.RequestSetOption{key, value})
}

func (cli *TMSPClient) AppendTxAsync(tx []byte) {
	cli.queueRequest(tmsp.RequestAppendTx{tx})
}

func (cli *TMSPClient) CheckTxAsync(tx []byte) {
	cli.queueRequest(tmsp.RequestCheckTx{tx})
}

func (cli *TMSPClient) GetHashAsync() {
	cli.queueRequest(tmsp.RequestGetHash{})
}

func (cli *TMSPClient) QueryAsync(query []byte) {
	cli.queueRequest(tmsp.RequestQuery{query})
}

//----------------------------------------

func (cli *TMSPClient) InfoSync() (info string, err error) {
	reqres := cli.queueRequest(tmsp.RequestInfo{})
	cli.FlushSync()
	if cli.err != nil {
		return "", cli.err
	}
	return reqres.Response.(tmsp.ResponseInfo).Info, nil
}

func (cli *TMSPClient) FlushSync() error {
	cli.queueRequest(tmsp.RequestFlush{}).Wait()
	return cli.err
}

func (cli *TMSPClient) AppendTxSync(tx []byte) (code tmsp.RetCode, result []byte, log string, err error) {
	reqres := cli.queueRequest(tmsp.RequestAppendTx{tx})
	cli.FlushSync()
	if cli.err != nil {
		return tmsp.RetCodeInternalError, nil, "", cli.err
	}
	res := reqres.Response.(tmsp.ResponseAppendTx)
	return res.Code, res.Result, res.Log, nil
}

func (cli *TMSPClient) CheckTxSync(tx []byte) (code tmsp.RetCode, result []byte, log string, err error) {
	reqres := cli.queueRequest(tmsp.RequestCheckTx{tx})
	cli.FlushSync()
	if cli.err != nil {
		return tmsp.RetCodeInternalError, nil, "", cli.err
	}
	res := reqres.Response.(tmsp.ResponseCheckTx)
	return res.Code, res.Result, res.Log, nil
}

func (cli *TMSPClient) GetHashSync() (hash []byte, log string, err error) {
	reqres := cli.queueRequest(tmsp.RequestGetHash{})
	cli.FlushSync()
	if cli.err != nil {
		return nil, "", cli.err
	}
	res := reqres.Response.(tmsp.ResponseGetHash)
	return res.Hash, res.Log, nil
}

func (cli *TMSPClient) QuerySync(query []byte) (result []byte, log string, err error) {
	reqres := cli.queueRequest(tmsp.RequestQuery{query})
	cli.FlushSync()
	if cli.err != nil {
		return nil, "", cli.err
	}
	res := reqres.Response.(tmsp.ResponseQuery)
	return res.Result, res.Log, nil
}

//----------------------------------------

func (cli *TMSPClient) queueRequest(req tmsp.Request) *reqRes {
	reqres := newReqRes(req)
	// TODO: set cli.err if reqQueue times out
	cli.reqQueue <- reqres

	// Maybe auto-flush, or unset auto-flush
	switch req.(type) {
	case tmsp.RequestFlush:
		cli.flushTimer.Unset()
	default:
		cli.flushTimer.Set()
	}

	return reqres
}

//----------------------------------------

func resMatchesReq(req tmsp.Request, res tmsp.Response) (ok bool) {
	switch req.(type) {
	case tmsp.RequestEcho:
		_, ok = res.(tmsp.ResponseEcho)
	case tmsp.RequestFlush:
		_, ok = res.(tmsp.ResponseFlush)
	case tmsp.RequestInfo:
		_, ok = res.(tmsp.ResponseInfo)
	case tmsp.RequestSetOption:
		_, ok = res.(tmsp.ResponseSetOption)
	case tmsp.RequestAppendTx:
		_, ok = res.(tmsp.ResponseAppendTx)
	case tmsp.RequestCheckTx:
		_, ok = res.(tmsp.ResponseCheckTx)
	case tmsp.RequestGetHash:
		_, ok = res.(tmsp.ResponseGetHash)
	case tmsp.RequestQuery:
		_, ok = res.(tmsp.ResponseQuery)
	default:
		return false
	}
	return
}

type reqRes struct {
	tmsp.Request
	*sync.WaitGroup
	tmsp.Response // Not set atomically, so be sure to use WaitGroup.
}

func newReqRes(req tmsp.Request) *reqRes {
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
