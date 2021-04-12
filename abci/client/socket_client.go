package abcicli

import (
	"bufio"
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"time"

	"github.com/tendermint/tendermint/abci/types"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/libs/timer"
)

const (
	// reqQueueSize is the max number of queued async requests.
	// (memory: 256MB max assuming 1MB transactions)
	reqQueueSize = 256
	// Don't wait longer than...
	flushThrottleMS = 20
)

type reqResWithContext struct {
	R *ReqRes
	C context.Context // if context.Err is not nil, reqRes will be thrown away (ignored)
}

// This is goroutine-safe, but users should beware that the application in
// general is not meant to be interfaced with concurrent callers.
type socketClient struct {
	service.BaseService

	addr        string
	mustConnect bool
	conn        net.Conn

	reqQueue   chan *reqResWithContext
	flushTimer *timer.ThrottleTimer

	mtx     tmsync.RWMutex
	err     error
	reqSent *list.List                            // list of requests sent, waiting for response
	resCb   func(*types.Request, *types.Response) // called on all requests, if set.
}

var _ Client = (*socketClient)(nil)

// NewSocketClient creates a new socket client, which connects to a given
// address. If mustConnect is true, the client will return an error upon start
// if it fails to connect.
func NewSocketClient(addr string, mustConnect bool) Client {
	cli := &socketClient{
		reqQueue:    make(chan *reqResWithContext, reqQueueSize),
		flushTimer:  timer.NewThrottleTimer("socketClient", flushThrottleMS),
		mustConnect: mustConnect,

		addr:    addr,
		reqSent: list.New(),
		resCb:   nil,
	}
	cli.BaseService = *service.NewBaseService(nil, "socketClient", cli)
	return cli
}

// OnStart implements Service by connecting to the server and spawning reading
// and writing goroutines.
func (cli *socketClient) OnStart() error {
	var (
		err  error
		conn net.Conn
	)

	for {
		conn, err = tmnet.Connect(cli.addr)
		if err != nil {
			if cli.mustConnect {
				return err
			}
			cli.Logger.Error(fmt.Sprintf("abci.socketClient failed to connect to %v.  Retrying after %vs...",
				cli.addr, dialRetryIntervalSeconds), "err", err)
			time.Sleep(time.Second * dialRetryIntervalSeconds)
			continue
		}
		cli.conn = conn

		go cli.sendRequestsRoutine(conn)
		go cli.recvResponseRoutine(conn)

		return nil
	}
}

// OnStop implements Service by closing connection and flushing all queues.
func (cli *socketClient) OnStop() {
	if cli.conn != nil {
		cli.conn.Close()
	}

	cli.flushQueue()
	cli.flushTimer.Stop()
}

// Error returns an error if the client was stopped abruptly.
func (cli *socketClient) Error() error {
	cli.mtx.RLock()
	defer cli.mtx.RUnlock()
	return cli.err
}

// SetResponseCallback sets a callback, which will be executed for each
// non-error & non-empty response from the server.
//
// NOTE: callback may get internally generated flush responses.
func (cli *socketClient) SetResponseCallback(resCb Callback) {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	cli.resCb = resCb
}

//----------------------------------------

func (cli *socketClient) sendRequestsRoutine(conn io.Writer) {
	w := bufio.NewWriter(conn)
	for {
		select {
		case reqres := <-cli.reqQueue:
			// cli.Logger.Debug("Sent request", "requestType", reflect.TypeOf(reqres.Request), "request", reqres.Request)

			if reqres.C.Err() != nil {
				cli.Logger.Debug("Request's context is done", "req", reqres.R, "err", reqres.C.Err())
				continue
			}

			cli.willSendReq(reqres.R)
			err := types.WriteMessage(reqres.R.Request, w)
			if err != nil {
				cli.stopForError(fmt.Errorf("write to buffer: %w", err))
				return
			}

			// If it's a flush request, flush the current buffer.
			if _, ok := reqres.R.Request.Value.(*types.Request_Flush); ok {
				err = w.Flush()
				if err != nil {
					cli.stopForError(fmt.Errorf("flush buffer: %w", err))
					return
				}
			}
		case <-cli.flushTimer.Ch: // flush queue
			select {
			case cli.reqQueue <- &reqResWithContext{R: NewReqRes(types.ToRequestFlush()), C: context.Background()}:
			default:
				// Probably will fill the buffer, or retry later.
			}
		case <-cli.Quit():
			return
		}
	}
}

func (cli *socketClient) recvResponseRoutine(conn io.Reader) {
	r := bufio.NewReader(conn)
	for {
		var res = &types.Response{}
		err := types.ReadMessage(r, res)
		if err != nil {
			cli.stopForError(fmt.Errorf("read message: %w", err))
			return
		}

		// cli.Logger.Debug("Received response", "responseType", reflect.TypeOf(res), "response", res)

		switch r := res.Value.(type) {
		case *types.Response_Exception: // app responded with error
			// XXX After setting cli.err, release waiters (e.g. reqres.Done())
			cli.stopForError(errors.New(r.Exception.Error))
			return
		default:
			err := cli.didRecvResponse(res)
			if err != nil {
				cli.stopForError(err)
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

	// Get the first ReqRes.
	next := cli.reqSent.Front()
	if next == nil {
		return fmt.Errorf("unexpected %v when nothing expected", reflect.TypeOf(res.Value))
	}

	reqres := next.Value.(*ReqRes)
	if !resMatchesReq(reqres.Request, res) {
		return fmt.Errorf("unexpected %v when response to %v expected",
			reflect.TypeOf(res.Value), reflect.TypeOf(reqres.Request.Value))
	}

	reqres.Response = res
	reqres.Done()            // release waiters
	cli.reqSent.Remove(next) // pop first item from linked list

	// Notify client listener if set (global callback).
	if cli.resCb != nil {
		cli.resCb(reqres.Request, res)
	}

	// Notify reqRes listener if set (request specific callback).
	//
	// NOTE: It is possible this callback isn't set on the reqres object. At this
	// point, in which case it will be called after, when it is set.
	reqres.InvokeCallback()

	return nil
}

//----------------------------------------

func (cli *socketClient) EchoAsync(ctx context.Context, msg string) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestEcho(msg))
}

func (cli *socketClient) FlushAsync(ctx context.Context) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestFlush())
}

func (cli *socketClient) InfoAsync(ctx context.Context, req types.RequestInfo) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestInfo(req))
}

func (cli *socketClient) DeliverTxAsync(ctx context.Context, req types.RequestDeliverTx) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestDeliverTx(req))
}

func (cli *socketClient) CheckTxAsync(ctx context.Context, req types.RequestCheckTx) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestCheckTx(req))
}

func (cli *socketClient) QueryAsync(ctx context.Context, req types.RequestQuery) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestQuery(req))
}

func (cli *socketClient) CommitAsync(ctx context.Context) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestCommit())
}

func (cli *socketClient) InitChainAsync(ctx context.Context, req types.RequestInitChain) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestInitChain(req))
}

func (cli *socketClient) BeginBlockAsync(ctx context.Context, req types.RequestBeginBlock) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestBeginBlock(req))
}

func (cli *socketClient) EndBlockAsync(ctx context.Context, req types.RequestEndBlock) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestEndBlock(req))
}

func (cli *socketClient) ListSnapshotsAsync(ctx context.Context, req types.RequestListSnapshots) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestListSnapshots(req))
}

func (cli *socketClient) OfferSnapshotAsync(ctx context.Context, req types.RequestOfferSnapshot) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestOfferSnapshot(req))
}

func (cli *socketClient) LoadSnapshotChunkAsync(
	ctx context.Context,
	req types.RequestLoadSnapshotChunk,
) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestLoadSnapshotChunk(req))
}

func (cli *socketClient) ApplySnapshotChunkAsync(
	ctx context.Context,
	req types.RequestApplySnapshotChunk,
) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestApplySnapshotChunk(req))
}

//----------------------------------------

func (cli *socketClient) FlushSync(ctx context.Context) error {
	reqRes, err := cli.queueRequest(ctx, types.ToRequestFlush(), true)
	if err != nil {
		return queueErr(err)
	}

	if err := cli.Error(); err != nil {
		return err
	}

	gotResp := make(chan struct{})
	go func() {
		// NOTE: if we don't flush the queue, its possible to get stuck here
		reqRes.Wait()
		close(gotResp)
	}()

	select {
	case <-gotResp:
		return cli.Error()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (cli *socketClient) EchoSync(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestEcho(msg))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetEcho(), nil
}

func (cli *socketClient) InfoSync(
	ctx context.Context,
	req types.RequestInfo,
) (*types.ResponseInfo, error) {
	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestInfo(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetInfo(), nil
}

func (cli *socketClient) DeliverTxSync(
	ctx context.Context,
	req types.RequestDeliverTx,
) (*types.ResponseDeliverTx, error) {

	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestDeliverTx(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetDeliverTx(), nil
}

func (cli *socketClient) CheckTxSync(
	ctx context.Context,
	req types.RequestCheckTx,
) (*types.ResponseCheckTx, error) {
	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestCheckTx(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetCheckTx(), nil
}

func (cli *socketClient) QuerySync(
	ctx context.Context,
	req types.RequestQuery,
) (*types.ResponseQuery, error) {
	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestQuery(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetQuery(), nil
}

func (cli *socketClient) CommitSync(ctx context.Context) (*types.ResponseCommit, error) {
	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestCommit())
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetCommit(), nil
}

func (cli *socketClient) InitChainSync(
	ctx context.Context,
	req types.RequestInitChain,
) (*types.ResponseInitChain, error) {

	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestInitChain(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetInitChain(), nil
}

func (cli *socketClient) BeginBlockSync(
	ctx context.Context,
	req types.RequestBeginBlock,
) (*types.ResponseBeginBlock, error) {

	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestBeginBlock(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetBeginBlock(), nil
}

func (cli *socketClient) EndBlockSync(
	ctx context.Context,
	req types.RequestEndBlock,
) (*types.ResponseEndBlock, error) {

	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestEndBlock(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetEndBlock(), nil
}

func (cli *socketClient) ListSnapshotsSync(
	ctx context.Context,
	req types.RequestListSnapshots,
) (*types.ResponseListSnapshots, error) {

	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestListSnapshots(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetListSnapshots(), nil
}

func (cli *socketClient) OfferSnapshotSync(
	ctx context.Context,
	req types.RequestOfferSnapshot,
) (*types.ResponseOfferSnapshot, error) {

	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestOfferSnapshot(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetOfferSnapshot(), nil
}

func (cli *socketClient) LoadSnapshotChunkSync(
	ctx context.Context,
	req types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {

	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestLoadSnapshotChunk(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetLoadSnapshotChunk(), nil
}

func (cli *socketClient) ApplySnapshotChunkSync(
	ctx context.Context,
	req types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {

	reqres, err := cli.queueRequestAndFlushSync(ctx, types.ToRequestApplySnapshotChunk(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetApplySnapshotChunk(), nil
}

//----------------------------------------

// queueRequest enqueues req onto the queue. If the queue is full, it ether
// returns an error (sync=false) or blocks (sync=true).
//
// When sync=true, ctx can be used to break early. When sync=false, ctx will be
// used later to determine if request should be dropped (if ctx.Err is
// non-nil).
//
// The caller is responsible for checking cli.Error.
func (cli *socketClient) queueRequest(ctx context.Context, req *types.Request, sync bool) (*ReqRes, error) {
	reqres := NewReqRes(req)

	if sync {
		select {
		case cli.reqQueue <- &reqResWithContext{R: reqres, C: context.Background()}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	} else {
		select {
		case cli.reqQueue <- &reqResWithContext{R: reqres, C: ctx}:
		default:
			return nil, errors.New("buffer is full")
		}
	}

	// Maybe auto-flush, or unset auto-flush
	switch req.Value.(type) {
	case *types.Request_Flush:
		cli.flushTimer.Unset()
	default:
		cli.flushTimer.Set()
	}

	return reqres, nil
}

func (cli *socketClient) queueRequestAsync(
	ctx context.Context,
	req *types.Request,
) (*ReqRes, error) {

	reqres, err := cli.queueRequest(ctx, req, false)
	if err != nil {
		return nil, queueErr(err)
	}

	return reqres, cli.Error()
}

func (cli *socketClient) queueRequestAndFlushSync(
	ctx context.Context,
	req *types.Request,
) (*ReqRes, error) {

	reqres, err := cli.queueRequest(ctx, req, true)
	if err != nil {
		return nil, queueErr(err)
	}

	if err := cli.FlushSync(ctx); err != nil {
		return nil, err
	}

	return reqres, cli.Error()
}

func queueErr(e error) error {
	return fmt.Errorf("can't queue req: %w", e)
}

func (cli *socketClient) flushQueue() {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()

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
			reqres.R.Done()
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
	case *types.Request_ApplySnapshotChunk:
		_, ok = res.Value.(*types.Response_ApplySnapshotChunk)
	case *types.Request_LoadSnapshotChunk:
		_, ok = res.Value.(*types.Response_LoadSnapshotChunk)
	case *types.Request_ListSnapshots:
		_, ok = res.Value.(*types.Response_ListSnapshots)
	case *types.Request_OfferSnapshot:
		_, ok = res.Value.(*types.Response_OfferSnapshot)
	}
	return ok
}

func (cli *socketClient) stopForError(err error) {
	if !cli.IsRunning() {
		return
	}

	cli.mtx.Lock()
	cli.err = err
	cli.mtx.Unlock()

	cli.Logger.Info("Stopping abci.socketClient", "reason", err)
	if err := cli.Stop(); err != nil {
		cli.Logger.Error("Error stopping abci.socketClient", "err", err)
	}
}
