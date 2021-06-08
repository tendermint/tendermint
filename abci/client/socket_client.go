package abciclient

import (
	"bufio"
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
)

const (
	// reqQueueSize is the max number of queued async requests.
	// (memory: 256MB max assuming 1MB transactions)
	reqQueueSize = 256
)

type reqResWithContext struct {
	R *ReqRes
	C context.Context // if context.Err is not nil, reqRes will be thrown away (ignored)
}

// This is goroutine-safe, but users should beware that the application in
// general is not meant to be interfaced with concurrent callers.
type socketClient struct {
	service.BaseService
	logger log.Logger

	addr        string
	mustConnect bool
	conn        net.Conn

	reqQueue chan *reqResWithContext

	mtx     sync.Mutex
	err     error
	reqSent *list.List                            // list of requests sent, waiting for response
	resCb   func(*types.Request, *types.Response) // called on all requests, if set.
}

var _ Client = (*socketClient)(nil)

// NewSocketClient creates a new socket client, which connects to a given
// address. If mustConnect is true, the client will return an error upon start
// if it fails to connect.
func NewSocketClient(logger log.Logger, addr string, mustConnect bool) Client {
	cli := &socketClient{
		logger:      logger,
		reqQueue:    make(chan *reqResWithContext, reqQueueSize),
		mustConnect: mustConnect,
		addr:        addr,
		reqSent:     list.New(),
		resCb:       nil,
	}
	cli.BaseService = *service.NewBaseService(logger, "socketClient", cli)
	return cli
}

// OnStart implements Service by connecting to the server and spawning reading
// and writing goroutines.
func (cli *socketClient) OnStart(ctx context.Context) error {
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
			cli.logger.Error(fmt.Sprintf("abci.socketClient failed to connect to %v.  Retrying after %vs...",
				cli.addr, dialRetryIntervalSeconds), "err", err)
			time.Sleep(time.Second * dialRetryIntervalSeconds)
			continue
		}
		cli.conn = conn

		go cli.sendRequestsRoutine(ctx, conn)
		go cli.recvResponseRoutine(ctx, conn)

		return nil
	}
}

// OnStop implements Service by closing connection and flushing all queues.
func (cli *socketClient) OnStop() {
	if cli.conn != nil {
		cli.conn.Close()
	}

	cli.drainQueue()
}

// Error returns an error if the client was stopped abruptly.
func (cli *socketClient) Error() error {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
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

func (cli *socketClient) sendRequestsRoutine(ctx context.Context, conn io.Writer) {
	bw := bufio.NewWriter(conn)
	for {
		select {
		case <-ctx.Done():
			return
		case reqres := <-cli.reqQueue:
			if ctx.Err() != nil {
				return
			}

			if reqres.C.Err() != nil {
				cli.logger.Debug("Request's context is done", "req", reqres.R, "err", reqres.C.Err())
				continue
			}
			cli.willSendReq(reqres.R)

			if err := types.WriteMessage(reqres.R.Request, bw); err != nil {
				cli.stopForError(fmt.Errorf("write to buffer: %w", err))
				return
			}
			if err := bw.Flush(); err != nil {
				cli.stopForError(fmt.Errorf("flush buffer: %w", err))
				return
			}
		}
	}
}

func (cli *socketClient) recvResponseRoutine(ctx context.Context, conn io.Reader) {
	r := bufio.NewReader(conn)
	for {
		if ctx.Err() != nil {
			return
		}
		var res = &types.Response{}
		err := types.ReadMessage(r, res)
		if err != nil {
			cli.stopForError(fmt.Errorf("read message: %w", err))
			return
		}

		// cli.logger.Debug("Received response", "responseType", reflect.TypeOf(res), "response", res)

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

func (cli *socketClient) FlushAsync(ctx context.Context) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestFlush())
}

func (cli *socketClient) CheckTxAsync(ctx context.Context, req types.RequestCheckTx) (*ReqRes, error) {
	return cli.queueRequestAsync(ctx, types.ToRequestCheckTx(req))
}

//----------------------------------------

func (cli *socketClient) Flush(ctx context.Context) error {
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

func (cli *socketClient) Echo(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestEcho(msg))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetEcho(), nil
}

func (cli *socketClient) Info(
	ctx context.Context,
	req types.RequestInfo,
) (*types.ResponseInfo, error) {
	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestInfo(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetInfo(), nil
}

func (cli *socketClient) CheckTx(
	ctx context.Context,
	req types.RequestCheckTx,
) (*types.ResponseCheckTx, error) {
	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestCheckTx(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetCheckTx(), nil
}

func (cli *socketClient) Query(
	ctx context.Context,
	req types.RequestQuery,
) (*types.ResponseQuery, error) {
	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestQuery(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetQuery(), nil
}

func (cli *socketClient) Commit(ctx context.Context) (*types.ResponseCommit, error) {
	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestCommit())
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetCommit(), nil
}

func (cli *socketClient) InitChain(
	ctx context.Context,
	req types.RequestInitChain,
) (*types.ResponseInitChain, error) {

	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestInitChain(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetInitChain(), nil
}

func (cli *socketClient) ListSnapshots(
	ctx context.Context,
	req types.RequestListSnapshots,
) (*types.ResponseListSnapshots, error) {

	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestListSnapshots(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetListSnapshots(), nil
}

func (cli *socketClient) OfferSnapshot(
	ctx context.Context,
	req types.RequestOfferSnapshot,
) (*types.ResponseOfferSnapshot, error) {

	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestOfferSnapshot(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetOfferSnapshot(), nil
}

func (cli *socketClient) LoadSnapshotChunk(
	ctx context.Context,
	req types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {

	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestLoadSnapshotChunk(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetLoadSnapshotChunk(), nil
}

func (cli *socketClient) ApplySnapshotChunk(
	ctx context.Context,
	req types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {

	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestApplySnapshotChunk(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetApplySnapshotChunk(), nil
}

func (cli *socketClient) PrepareProposal(
	ctx context.Context,
	req types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {

	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestPrepareProposal(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetPrepareProposal(), nil
}

func (cli *socketClient) ProcessProposal(
	ctx context.Context,
	req types.RequestProcessProposal,
) (*types.ResponseProcessProposal, error) {

	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestProcessProposal(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetProcessProposal(), nil
}

func (cli *socketClient) ExtendVote(
	ctx context.Context,
	req types.RequestExtendVote) (*types.ResponseExtendVote, error) {

	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestExtendVote(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetExtendVote(), nil
}

func (cli *socketClient) VerifyVoteExtension(
	ctx context.Context,
	req types.RequestVerifyVoteExtension) (*types.ResponseVerifyVoteExtension, error) {

	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestVerifyVoteExtension(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetVerifyVoteExtension(), nil
}

func (cli *socketClient) FinalizeBlock(
	ctx context.Context,
	req types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {

	reqres, err := cli.queueRequestAndFlush(ctx, types.ToRequestFinalizeBlock(req))
	if err != nil {
		return nil, err
	}
	return reqres.Response.GetFinalizeBlock(), nil
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

func (cli *socketClient) queueRequestAndFlush(
	ctx context.Context,
	req *types.Request,
) (*ReqRes, error) {

	reqres, err := cli.queueRequest(ctx, req, true)
	if err != nil {
		return nil, queueErr(err)
	}

	if err := cli.Flush(ctx); err != nil {
		return nil, err
	}

	return reqres, cli.Error()
}

func queueErr(e error) error {
	return fmt.Errorf("can't queue req: %w", e)
}

// drainQueue marks as complete and discards all remaining pending requests
// from the queue.
func (cli *socketClient) drainQueue() {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()

	// mark all in-flight messages as resolved (they will get cli.Error())
	for req := cli.reqSent.Front(); req != nil; req = req.Next() {
		reqres := req.Value.(*ReqRes)
		reqres.Done()
	}

	// Mark all queued messages as resolved.
	//
	// TODO(creachadair): We can't simply range the channel, because it is never
	// closed, and the writer continues to add work.
	// See https://github.com/tendermint/tendermint/issues/6996.
	for {
		select {
		case reqres := <-cli.reqQueue:
			reqres.R.Done()
		default:
			return
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
	case *types.Request_CheckTx:
		_, ok = res.Value.(*types.Response_CheckTx)
	case *types.Request_Commit:
		_, ok = res.Value.(*types.Response_Commit)
	case *types.Request_Query:
		_, ok = res.Value.(*types.Response_Query)
	case *types.Request_InitChain:
		_, ok = res.Value.(*types.Response_InitChain)
	case *types.Request_PrepareProposal:
		_, ok = res.Value.(*types.Response_PrepareProposal)
	case *types.Request_ExtendVote:
		_, ok = res.Value.(*types.Response_ExtendVote)
	case *types.Request_VerifyVoteExtension:
		_, ok = res.Value.(*types.Response_VerifyVoteExtension)
	case *types.Request_ApplySnapshotChunk:
		_, ok = res.Value.(*types.Response_ApplySnapshotChunk)
	case *types.Request_LoadSnapshotChunk:
		_, ok = res.Value.(*types.Response_LoadSnapshotChunk)
	case *types.Request_ListSnapshots:
		_, ok = res.Value.(*types.Response_ListSnapshots)
	case *types.Request_OfferSnapshot:
		_, ok = res.Value.(*types.Response_OfferSnapshot)
	case *types.Request_FinalizeBlock:
		_, ok = res.Value.(*types.Response_FinalizeBlock)
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

	cli.logger.Info("Stopping abci.socketClient", "reason", err)
	if err := cli.Stop(); err != nil {
		cli.logger.Error("error stopping abci.socketClient", "err", err)
	}
}
