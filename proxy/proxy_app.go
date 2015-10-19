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
// In other words, the mempool and consensus modules need to
// exclude each other w/ an external mutex.
type ProxyApp struct {
	QuitService
	sync.Mutex

	reqQueue chan QueuedRequest

	mtx            sync.Mutex
	conn           net.Conn
	bufWriter      *bufio.Writer
	err            error
	reqSent        *list.List
	reqPending     *list.Element // Next element in reqSent waiting for response
	resReceived    *list.List
	eventsReceived *list.List
}

func NewProxyApp(conn net.Conn, bufferSize int) *ProxyApp {
	p := &ProxyApp{
		reqQueue:       make(chan QueuedRequest, bufferSize),
		conn:           conn,
		bufWriter:      bufio.NewWriter(conn),
		reqSent:        list.New(),
		reqPending:     nil,
		resReceived:    list.New(),
		eventsReceived: list.New(),
	}
	p.QuitService = *NewQuitService(nil, "ProxyApp", p)
	return p
}

func (p *ProxyApp) OnStart() error {
	p.QuitService.OnStart()
	go p.sendRequestsRoutine()
	go p.recvResponseRoutine()
	return nil
}

func (p *ProxyApp) OnStop() {
	p.QuitService.OnStop()
	p.conn.Close()
}

func (p *ProxyApp) StopForError(err error) {
	p.mtx.Lock()
	fmt.Println("Stopping ProxyApp for error:", err)
	if p.err == nil {
		p.err = err
	}
	p.mtx.Unlock()
	p.Stop()
}

func (p *ProxyApp) Error() error {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return p.err
}

//----------------------------------------

func (p *ProxyApp) sendRequestsRoutine() {
	for {
		var n int
		var err error
		select {
		case <-p.QuitService.Quit:
			return
		case qreq := <-p.reqQueue:
			wire.WriteBinary(qreq.Request, p.bufWriter, &n, &err)
			if err != nil {
				p.StopForError(err)
				return
			}
			if _, ok := qreq.Request.(tmsp.RequestFlush); ok {
				err = p.bufWriter.Flush()
				if err != nil {
					p.StopForError(err)
					return
				}
			}
			p.didSendReq(qreq)
		}
	}
}

func (p *ProxyApp) recvResponseRoutine() {
	r := bufio.NewReader(p.conn) // Buffer reads
	for {
		var res tmsp.Response
		var n int
		var err error
		wire.ReadBinaryPtr(&res, r, maxResponseSize, &n, &err)
		if err != nil {
			p.StopForError(err)
			return
		}
		switch res := res.(type) {
		case tmsp.ResponseException:
			p.StopForError(errors.New(res.Error))
		case tmsp.ResponseEvent:
			p.didRecvEvent(res.Event)
		default:
			err := p.didRecvResponse(res)
			if err != nil {
				p.StopForError(err)
			}
		}
	}
}

func (p *ProxyApp) didSendReq(qreq QueuedRequest) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	p.reqSent.PushBack(qreq)
	if p.reqPending == nil {
		p.reqPending = p.reqSent.Front()
	}
}

func (p *ProxyApp) didRecvResponse(res tmsp.Response) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if p.reqPending == nil {
		return fmt.Errorf("Unexpected result type %v when nothing expected",
			reflect.TypeOf(res))
	} else {
		qreq := p.reqPending.Value.(QueuedRequest)
		if !resMatchesReq(qreq.Request, res) {
			return fmt.Errorf("Unexpected result type %v when response to %v expected",
				reflect.TypeOf(res), reflect.TypeOf(qreq.Request))
		}
		if qreq.Sync {
			qreq.Done()
		}
		p.reqPending = p.reqPending.Next()
	}
	p.resReceived.PushBack(res)
	return nil
}

func (p *ProxyApp) didRecvEvent(event tmsp.Event) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	p.eventsReceived.PushBack(event)
}

//----------------------------------------

func (p *ProxyApp) EchoAsync(key string) {
	p.queueRequestAsync(tmsp.RequestEcho{key})
}

func (p *ProxyApp) FlushAsync() {
	p.queueRequestAsync(tmsp.RequestFlush{})
}

func (p *ProxyApp) AppendTxAsync(tx []byte) {
	p.queueRequestAsync(tmsp.RequestAppendTx{tx})
}

func (p *ProxyApp) GetHashAsync() {
	p.queueRequestAsync(tmsp.RequestGetHash{})
}

/*
func (p *ProxyApp) CommitAsync() {
	p.queueRequestAsync(tmsp.RequestCommit{})
}

func (p *ProxyApp) RollbackAsync() {
	p.queueRequestAsync(tmsp.RequestRollback{})
}
*/

func (p *ProxyApp) SetEventsModeAsync(mode tmsp.EventsMode) {
	p.queueRequestAsync(tmsp.RequestSetEventsMode{mode})
}

func (p *ProxyApp) AddListenerAsync(key string) {
	p.queueRequestAsync(tmsp.RequestAddListener{key})
}

func (p *ProxyApp) RemListenerAsync(key string) {
	p.queueRequestAsync(tmsp.RequestRemListener{key})
}

//----------------------------------------

// Get valid txs, root hash, events; or error
// Clears internal buffers
func (p *ProxyApp) ReapSync(commit bool) (txs [][]byte, hash []byte, events []tmsp.Event, err error) {
	if commit {
		// Send asynchronous commit
		p.queueRequestAsync(tmsp.RequestCommit{})
		// NOTE: we're assuming that there won't be a race condition.
	}
	// Get hash.
	p.queueRequestAsync(tmsp.RequestGetHash{})
	// Flush everything.
	p.queueRequestSync(tmsp.RequestFlush{})
	// Maybe there was an error in response matching
	if p.err != nil {
		return nil, nil, nil, p.err
	}
	// Process the resReceived/reqSent/reqPending.
	if p.resReceived.Len() != p.reqSent.Len() {
		PanicSanity("Unmatched requests & responses")
	}
	var commitCounter = 0
	txs = make([][]byte, 0, p.reqSent.Len())
	events = make([]tmsp.Event, 0, p.eventsReceived.Len())
	reqE, resE := p.reqSent.Front(), p.resReceived.Front()
	for ; reqE != nil; reqE, resE = reqE.Next(), resE.Next() {
		req, res := reqE.Value.(tmsp.Request), resE.Value.(tmsp.Response)
		switch req := req.(type) {
		case tmsp.RequestAppendTx:
			txs = append(txs, req.TxBytes)
		case tmsp.RequestGetHash:
			hash = res.(tmsp.ResponseGetHash).Hash
		case tmsp.RequestCommit:
			if commitCounter > 0 {
				PanicSanity("Unexpected Commit response")
			}
			commitCounter++
		case tmsp.RequestRollback:
			PanicSanity("Unexpected Rollback response")
		default:
			// ignore other messages
		}
	}
	for eE := p.eventsReceived.Front(); eE != nil; eE = eE.Next() {
		events = append(events, eE.Value.(tmsp.Event))
	}

	return txs, hash, events, nil
}

// Rollback or error
// Clears internal buffers
func (p *ProxyApp) RollbackSync() (err error) {
	// Get hash.
	p.queueRequestAsync(tmsp.RequestRollback{})
	// Flush everything.
	p.queueRequestSync(tmsp.RequestFlush{})
	// Maybe there was an error in response matching
	if p.err != nil {
		return p.err
	}
	p.reqSent = list.New()
	p.reqPending = nil
	p.resReceived = list.New()
	p.eventsReceived = list.New()
	return nil
}

func (p *ProxyApp) InfoSync() []string {
	p.queueRequestAsync(tmsp.RequestInfo{})
	p.queueRequestSync(tmsp.RequestFlush{})
	return p.resReceived.Back().Prev().Value.(tmsp.ResponseInfo).Data
}

func (p *ProxyApp) FlushSync() {
	p.queueRequestSync(tmsp.RequestFlush{})
}

//----------------------------------------

func (p *ProxyApp) queueRequestAsync(req tmsp.Request) {
	qreq := QueuedRequest{Request: req}
	p.reqQueue <- qreq
}

func (p *ProxyApp) queueRequestSync(req tmsp.Request) {
	qreq := QueuedRequest{
		req,
		true,
		waitGroup1(),
	}
	p.reqQueue <- qreq
	qreq.Wait()
}

//----------------------------------------

func waitGroup1() (wg *sync.WaitGroup) {
	wg = &sync.WaitGroup{}
	wg.Add(1)
	return
}

func resMatchesReq(req tmsp.Request, res tmsp.Response) (ok bool) {
	switch req.(type) {
	case tmsp.RequestEcho:
		_, ok = res.(tmsp.ResponseEcho)
	case tmsp.RequestFlush:
		_, ok = res.(tmsp.ResponseFlush)
	case tmsp.RequestInfo:
		_, ok = res.(tmsp.ResponseInfo)
	case tmsp.RequestAppendTx:
		_, ok = res.(tmsp.ResponseAppendTx)
	case tmsp.RequestGetHash:
		_, ok = res.(tmsp.ResponseGetHash)
	case tmsp.RequestCommit:
		_, ok = res.(tmsp.ResponseCommit)
	case tmsp.RequestRollback:
		_, ok = res.(tmsp.ResponseRollback)
	case tmsp.RequestSetEventsMode:
		_, ok = res.(tmsp.ResponseSetEventsMode)
	case tmsp.RequestAddListener:
		_, ok = res.(tmsp.ResponseAddListener)
	case tmsp.RequestRemListener:
		_, ok = res.(tmsp.ResponseRemListener)
	default:
		return false
	}
	return
}

type QueuedRequest struct {
	tmsp.Request
	Sync bool
	*sync.WaitGroup
}
