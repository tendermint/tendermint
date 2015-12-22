package proxy

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

const maxResponseSize = 1048576 // 1MB
const flushThrottleMS = 20      // Don't wait longer than...

// This is goroutine-safe, but users should beware that
// the application in general is not meant to be interfaced
// with concurrent callers.
type remoteAppConn struct {
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

func NewRemoteAppConn(conn net.Conn, bufferSize int) *remoteAppConn {
	app := &remoteAppConn{
		reqQueue:   make(chan *reqRes, bufferSize),
		flushTimer: NewThrottleTimer("remoteAppConn", flushThrottleMS),

		conn:      conn,
		bufWriter: bufio.NewWriter(conn),
		reqSent:   list.New(),
		resCb:     nil,
	}
	app.QuitService = *NewQuitService(nil, "remoteAppConn", app)
	return app
}

func (app *remoteAppConn) OnStart() error {
	app.QuitService.OnStart()
	go app.sendRequestsRoutine()
	go app.recvResponseRoutine()
	return nil
}

func (app *remoteAppConn) OnStop() {
	app.QuitService.OnStop()
	app.conn.Close()
}

// NOTE: callback may get internally generated flush responses.
func (app *remoteAppConn) SetResponseCallback(resCb Callback) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	app.resCb = resCb
}

func (app *remoteAppConn) StopForError(err error) {
	app.mtx.Lock()
	log.Error("Stopping remoteAppConn for error.", "error", err)
	if app.err == nil {
		app.err = err
	}
	app.mtx.Unlock()
	app.Stop()
}

func (app *remoteAppConn) Error() error {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.err
}

//----------------------------------------

func (app *remoteAppConn) sendRequestsRoutine() {
	for {
		var n int
		var err error
		select {
		case <-app.flushTimer.Ch:
			select {
			case app.reqQueue <- newReqRes(tmsp.RequestFlush{}):
			default:
				// Probably will fill the buffer, or retry later.
			}
		case <-app.QuitService.Quit:
			return
		case reqres := <-app.reqQueue:
			app.willSendReq(reqres)
			wire.WriteBinaryLengthPrefixed(struct{ tmsp.Request }{reqres.Request}, app.bufWriter, &n, &err) // Length prefix
			if err != nil {
				app.StopForError(err)
				return
			}
			if _, ok := reqres.Request.(tmsp.RequestFlush); ok {
				err = app.bufWriter.Flush()
				if err != nil {
					app.StopForError(err)
					return
				}
			}
		}
	}
}

func (app *remoteAppConn) recvResponseRoutine() {
	r := bufio.NewReader(app.conn) // Buffer reads
	for {
		var res tmsp.Response
		var n int
		var err error
		wire.ReadBinaryPtrLengthPrefixed(&res, r, maxResponseSize, &n, &err)
		if err != nil {
			app.StopForError(err)
			return
		}
		switch res := res.(type) {
		case tmsp.ResponseException:
			app.StopForError(errors.New(res.Error))
		default:
			err := app.didRecvResponse(res)
			if err != nil {
				app.StopForError(err)
			}
		}
	}
}

func (app *remoteAppConn) willSendReq(reqres *reqRes) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	app.reqSent.PushBack(reqres)
}

func (app *remoteAppConn) didRecvResponse(res tmsp.Response) error {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	// Special logic for events which have no corresponding requests.
	if _, ok := res.(tmsp.ResponseEvent); ok && app.resCb != nil {
		app.resCb(nil, res)
		return nil
	}

	// Get the first reqRes
	next := app.reqSent.Front()
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
	app.reqSent.Remove(next) // Pop first item from linked list

	// Callback if there is a listener
	if app.resCb != nil {
		app.resCb(reqres.Request, res)
	}

	return nil
}

//----------------------------------------

func (app *remoteAppConn) EchoAsync(msg string) {
	app.queueRequest(tmsp.RequestEcho{msg})
}

func (app *remoteAppConn) FlushAsync() {
	app.queueRequest(tmsp.RequestFlush{})
}

func (app *remoteAppConn) SetOptionAsync(key string, value string) {
	app.queueRequest(tmsp.RequestSetOption{key, value})
}

func (app *remoteAppConn) AppendTxAsync(tx []byte) {
	app.queueRequest(tmsp.RequestAppendTx{tx})
}

func (app *remoteAppConn) CheckTxAsync(tx []byte) {
	app.queueRequest(tmsp.RequestCheckTx{tx})
}

func (app *remoteAppConn) GetHashAsync() {
	app.queueRequest(tmsp.RequestGetHash{})
}

func (app *remoteAppConn) AddListenerAsync(key string) {
	app.queueRequest(tmsp.RequestAddListener{key})
}

func (app *remoteAppConn) RemListenerAsync(key string) {
	app.queueRequest(tmsp.RequestRemListener{key})
}

//----------------------------------------

func (app *remoteAppConn) InfoSync() (info []string, err error) {
	reqres := app.queueRequest(tmsp.RequestInfo{})
	app.FlushSync()
	if app.err != nil {
		return nil, app.err
	}
	return reqres.Response.(tmsp.ResponseInfo).Data, nil
}

func (app *remoteAppConn) FlushSync() error {
	app.queueRequest(tmsp.RequestFlush{}).Wait()
	return app.err
}

func (app *remoteAppConn) GetHashSync() (hash []byte, err error) {
	reqres := app.queueRequest(tmsp.RequestGetHash{})
	app.FlushSync()
	if app.err != nil {
		return nil, app.err
	}
	return reqres.Response.(tmsp.ResponseGetHash).Hash, nil
}

//----------------------------------------

func (app *remoteAppConn) queueRequest(req tmsp.Request) *reqRes {
	reqres := newReqRes(req)
	// TODO: set app.err if reqQueue times out
	app.reqQueue <- reqres

	// Maybe auto-flush, or unset auto-flush
	switch req.(type) {
	case tmsp.RequestFlush:
		app.flushTimer.Unset()
	default:
		app.flushTimer.Set()
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
	case tmsp.RequestAddListener:
		_, ok = res.(tmsp.ResponseAddListener)
	case tmsp.RequestRemListener:
		_, ok = res.(tmsp.ResponseRemListener)
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
