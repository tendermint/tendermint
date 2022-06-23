package abciclient

import (
	"bufio"
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
)

// This is goroutine-safe, but users should beware that the application in
// general is not meant to be interfaced with concurrent callers.
type socketClient struct {
	service.BaseService
	logger log.Logger

	addr        string
	mustConnect bool
	conn        net.Conn

	reqQueue chan *requestAndResponse

	mtx     sync.Mutex
	err     error
	reqSent *list.List // list of requests sent, waiting for response
}

var _ Client = (*socketClient)(nil)

// NewSocketClient creates a new socket client, which connects to a given
// address. If mustConnect is true, the client will return an error upon start
// if it fails to connect.
func NewSocketClient(logger log.Logger, addr string, mustConnect bool) Client {
	cli := &socketClient{
		logger:      logger,
		reqQueue:    make(chan *requestAndResponse),
		mustConnect: mustConnect,
		addr:        addr,
		reqSent:     list.New(),
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
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		conn, err = tmnet.Connect(cli.addr)
		if err != nil {
			if cli.mustConnect {
				return err
			}

			cli.logger.Error("abci.socketClient failed to connect, retrying after",
				"retry_after", dialRetryIntervalSeconds,
				"target", cli.addr,
				"err", err)

			timer.Reset(time.Second * dialRetryIntervalSeconds)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				continue
			}
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

//----------------------------------------

func (cli *socketClient) sendRequestsRoutine(ctx context.Context, conn io.Writer) {
	bw := bufio.NewWriter(conn)
	for {
		select {
		case <-ctx.Done():
			return
		case reqres := <-cli.reqQueue:
			// N.B. We must enqueue before sending out the request, otherwise the
			// server may reply before we do it, and the receiver will fail for an
			// unsolicited reply.
			cli.trackRequest(reqres)

			if err := types.WriteMessage(reqres.Request, bw); err != nil {
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
		res := &types.Response{}

		if err := types.ReadMessage(r, res); err != nil {
			cli.stopForError(fmt.Errorf("read message: %w", err))
			return
		}

		switch r := res.Value.(type) {
		case *types.Response_Exception: // app responded with error
			// XXX After setting cli.err, release waiters (e.g. reqres.Done())
			cli.stopForError(errors.New(r.Exception.Error))
			return
		default:
			if err := cli.didRecvResponse(res); err != nil {
				cli.stopForError(err)
				return
			}
		}
	}
}

func (cli *socketClient) trackRequest(reqres *requestAndResponse) {
	// N.B. We must NOT hold the client state lock while checking this, or we
	// may deadlock with shutdown.
	if !cli.IsRunning() {
		return
	}

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
		return fmt.Errorf("unexpected %T when nothing expected", res.Value)
	}

	reqres := next.Value.(*requestAndResponse)
	if !resMatchesReq(reqres.Request, res) {
		return fmt.Errorf("unexpected %T when response to %T expected", res.Value, reqres.Request.Value)
	}

	reqres.Response = res
	reqres.markDone()        // release waiters
	cli.reqSent.Remove(next) // pop first item from linked list

	return nil
}

//----------------------------------------

func (cli *socketClient) doRequest(ctx context.Context, req *types.Request) (*types.Response, error) {
	if !cli.IsRunning() {
		return nil, errors.New("client has stopped")
	}

	reqres := makeReqRes(req)

	select {
	case cli.reqQueue <- reqres:
	case <-ctx.Done():
		return nil, fmt.Errorf("can't queue req: %w", ctx.Err())
	}

	select {
	case <-reqres.signal:
		if err := cli.Error(); err != nil {
			return nil, err
		}

		return reqres.Response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// drainQueue marks as complete and discards all remaining pending requests
// from the queue.
func (cli *socketClient) drainQueue() {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()

	// mark all in-flight messages as resolved (they will get cli.Error())
	for req := cli.reqSent.Front(); req != nil; req = req.Next() {
		reqres := req.Value.(*requestAndResponse)
		reqres.markDone()
	}
}

//----------------------------------------

func (cli *socketClient) Flush(ctx context.Context) error {
	_, err := cli.doRequest(ctx, types.ToRequestFlush())
	if err != nil {
		return err
	}
	return nil
}

func (cli *socketClient) Echo(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	res, err := cli.doRequest(ctx, types.ToRequestEcho(msg))
	if err != nil {
		return nil, err
	}
	return res.GetEcho(), nil
}

func (cli *socketClient) Info(ctx context.Context, req *types.RequestInfo) (*types.ResponseInfo, error) {
	res, err := cli.doRequest(ctx, types.ToRequestInfo(req))
	if err != nil {
		return nil, err
	}
	return res.GetInfo(), nil
}

func (cli *socketClient) CheckTx(ctx context.Context, req *types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	res, err := cli.doRequest(ctx, types.ToRequestCheckTx(req))
	if err != nil {
		return nil, err
	}
	return res.GetCheckTx(), nil
}

func (cli *socketClient) Query(ctx context.Context, req *types.RequestQuery) (*types.ResponseQuery, error) {
	res, err := cli.doRequest(ctx, types.ToRequestQuery(req))
	if err != nil {
		return nil, err
	}
	return res.GetQuery(), nil
}

func (cli *socketClient) Commit(ctx context.Context) (*types.ResponseCommit, error) {
	res, err := cli.doRequest(ctx, types.ToRequestCommit())
	if err != nil {
		return nil, err
	}
	return res.GetCommit(), nil
}

func (cli *socketClient) InitChain(ctx context.Context, req *types.RequestInitChain) (*types.ResponseInitChain, error) {
	res, err := cli.doRequest(ctx, types.ToRequestInitChain(req))
	if err != nil {
		return nil, err
	}
	return res.GetInitChain(), nil
}

func (cli *socketClient) ListSnapshots(ctx context.Context, req *types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
	res, err := cli.doRequest(ctx, types.ToRequestListSnapshots(req))
	if err != nil {
		return nil, err
	}
	return res.GetListSnapshots(), nil
}

func (cli *socketClient) OfferSnapshot(ctx context.Context, req *types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	res, err := cli.doRequest(ctx, types.ToRequestOfferSnapshot(req))
	if err != nil {
		return nil, err
	}
	return res.GetOfferSnapshot(), nil
}

func (cli *socketClient) LoadSnapshotChunk(ctx context.Context, req *types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {
	res, err := cli.doRequest(ctx, types.ToRequestLoadSnapshotChunk(req))
	if err != nil {
		return nil, err
	}
	return res.GetLoadSnapshotChunk(), nil
}

func (cli *socketClient) ApplySnapshotChunk(ctx context.Context, req *types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
	res, err := cli.doRequest(ctx, types.ToRequestApplySnapshotChunk(req))
	if err != nil {
		return nil, err
	}
	return res.GetApplySnapshotChunk(), nil
}

func (cli *socketClient) PrepareProposal(ctx context.Context, req *types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	res, err := cli.doRequest(ctx, types.ToRequestPrepareProposal(req))
	if err != nil {
		return nil, err
	}
	return res.GetPrepareProposal(), nil
}

func (cli *socketClient) ProcessProposal(ctx context.Context, req *types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
	res, err := cli.doRequest(ctx, types.ToRequestProcessProposal(req))
	if err != nil {
		return nil, err
	}
	return res.GetProcessProposal(), nil
}

func (cli *socketClient) ExtendVote(ctx context.Context, req *types.RequestExtendVote) (*types.ResponseExtendVote, error) {
	res, err := cli.doRequest(ctx, types.ToRequestExtendVote(req))
	if err != nil {
		return nil, err
	}
	return res.GetExtendVote(), nil
}

func (cli *socketClient) VerifyVoteExtension(ctx context.Context, req *types.RequestVerifyVoteExtension) (*types.ResponseVerifyVoteExtension, error) {
	res, err := cli.doRequest(ctx, types.ToRequestVerifyVoteExtension(req))
	if err != nil {
		return nil, err
	}
	return res.GetVerifyVoteExtension(), nil
}

func (cli *socketClient) FinalizeBlock(ctx context.Context, req *types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	res, err := cli.doRequest(ctx, types.ToRequestFinalizeBlock(req))
	if err != nil {
		return nil, err
	}
	return res.GetFinalizeBlock(), nil
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
	case *types.Request_ProcessProposal:
		_, ok = res.Value.(*types.Response_ProcessProposal)
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
	cli.Stop()
}
