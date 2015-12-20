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

// This is goroutine-safe, but users should beware that
// the application in general is not meant to be interfaced
// with concurrent callers.
type remoteAppContext struct {
	QuitService
	sync.Mutex // [EB]: is this even used?

	reqQueue chan *reqRes

	mtx       sync.Mutex
	conn      net.Conn
	bufWriter *bufio.Writer
	err       error
	reqSent   *list.List
	resCb     func(tmsp.Request, tmsp.Response)
}

func NewRemoteAppContext(conn net.Conn, bufferSize int) *remoteAppContext {
	app := &remoteAppContext{
		reqQueue:  make(chan *reqRes, bufferSize),
		conn:      conn,
		bufWriter: bufio.NewWriter(conn),
		reqSent:   list.New(),
		resCb:     nil,
	}
	app.QuitService = *NewQuitService(nil, "remoteAppContext", app)
	return app
}

func (app *remoteAppContext) OnStart() error {
	app.QuitService.OnStart()
	go app.sendRequestsRoutine()
	go app.recvResponseRoutine()
	return nil
}

func (app *remoteAppContext) OnStop() {
	app.QuitService.OnStop()
	app.conn.Close()
}

func (app *remoteAppContext) SetResponseCallback(resCb Callback) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	app.resCb = resCb
}

func (app *remoteAppContext) StopForError(err error) {
	app.mtx.Lock()
	log.Error("Stopping remoteAppContext for error.", "error", err)
	if app.err == nil {
		app.err = err
	}
	app.mtx.Unlock()
	app.Stop()
}

func (app *remoteAppContext) Error() error {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.err
}

//----------------------------------------

func (app *remoteAppContext) sendRequestsRoutine() {
	for {
		var n int
		var err error
		select {
		case <-app.QuitService.Quit:
			return
		case reqres := <-app.reqQueue:

			app.willSendReq(reqres)

			wire.WriteBinaryLengthPrefixed(reqres.Request, app.bufWriter, &n, &err) // Length prefix
			if err != nil {
				app.StopForError(err)
				return
			}
			log.Debug("Sent request", "requestType", reflect.TypeOf(reqres.Request), "request", reqres.Request)
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

func (app *remoteAppContext) recvResponseRoutine() {
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
			log.Debug("Received response", "responseType", reflect.TypeOf(res), "response", res)
			err := app.didRecvResponse(res)
			if err != nil {
				app.StopForError(err)
			}
		}
	}
}

func (app *remoteAppContext) willSendReq(reqres *reqRes) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	app.reqSent.PushBack(reqres)
}

func (app *remoteAppContext) didRecvResponse(res tmsp.Response) error {
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

func (app *remoteAppContext) EchoAsync(msg string) {
	app.queueRequest(tmsp.RequestEcho{msg})
}

func (app *remoteAppContext) FlushAsync() {
	app.queueRequest(tmsp.RequestFlush{})
}

func (app *remoteAppContext) SetOptionAsync(key string, value string) {
	app.queueRequest(tmsp.RequestSetOption{key, value})
}

func (app *remoteAppContext) AppendTxAsync(tx []byte) {
	app.queueRequest(tmsp.RequestAppendTx{tx})
}

func (app *remoteAppContext) GetHashAsync() {
	app.queueRequest(tmsp.RequestGetHash{})
}

func (app *remoteAppContext) CommitAsync() {
	app.queueRequest(tmsp.RequestCommit{})
}

func (app *remoteAppContext) RollbackAsync() {
	app.queueRequest(tmsp.RequestRollback{})
}

func (app *remoteAppContext) AddListenerAsync(key string) {
	app.queueRequest(tmsp.RequestAddListener{key})
}

func (app *remoteAppContext) RemListenerAsync(key string) {
	app.queueRequest(tmsp.RequestRemListener{key})
}

//----------------------------------------

func (app *remoteAppContext) InfoSync() (info []string, err error) {
	reqres := app.queueRequest(tmsp.RequestInfo{})
	app.FlushSync()
	if app.err != nil {
		return nil, app.err
	}
	return reqres.Response.(tmsp.ResponseInfo).Data, nil
}

func (app *remoteAppContext) FlushSync() error {
	app.queueRequest(tmsp.RequestFlush{}).Wait()
	return app.err
}

func (app *remoteAppContext) GetHashSync() (hash []byte, err error) {
	reqres := app.queueRequest(tmsp.RequestGetHash{})
	app.FlushSync()
	if app.err != nil {
		return nil, app.err
	}
	return reqres.Response.(tmsp.ResponseGetHash).Hash, nil
}

// Commits or error
func (app *remoteAppContext) CommitSync() (err error) {
	app.queueRequest(tmsp.RequestCommit{})
	app.FlushSync()
	return app.err
}

// Rollback or error
// Clears internal buffers
func (app *remoteAppContext) RollbackSync() (err error) {
	app.queueRequest(tmsp.RequestRollback{})
	app.FlushSync()
	return app.err
}

//----------------------------------------

func (app *remoteAppContext) queueRequest(req tmsp.Request) *reqRes {
	reqres := NewreqRes(req)
	// TODO: set app.err if reqQueue times out
	app.reqQueue <- reqres
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
	case tmsp.RequestGetHash:
		_, ok = res.(tmsp.ResponseGetHash)
	case tmsp.RequestCommit:
		_, ok = res.(tmsp.ResponseCommit)
	case tmsp.RequestRollback:
		_, ok = res.(tmsp.ResponseRollback)
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

func NewreqRes(req tmsp.Request) *reqRes {
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
