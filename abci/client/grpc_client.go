package abciclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
)

// A gRPC client.
type grpcClient struct {
	service.BaseService
	logger log.Logger

	mustConnect bool

	client   types.ABCIApplicationClient
	conn     *grpc.ClientConn
	chReqRes chan *ReqRes // dispatches "async" responses to callbacks *in order*, needed by mempool

	mtx   sync.Mutex
	addr  string
	err   error
	resCb func(*types.Request, *types.Response) // listens to all callbacks
}

var _ Client = (*grpcClient)(nil)

// NewGRPCClient creates a gRPC client, which will connect to addr upon the
// start. Note Client#Start returns an error if connection is unsuccessful and
// mustConnect is true.
//
// GRPC calls are synchronous, but some callbacks expect to be called
// asynchronously (eg. the mempool expects to be able to lock to remove bad txs
// from cache). To accommodate, we finish each call in its own go-routine,
// which is expensive, but easy - if you want something better, use the socket
// protocol! maybe one day, if people really want it, we use grpc streams, but
// hopefully not :D
func NewGRPCClient(logger log.Logger, addr string, mustConnect bool) Client {
	cli := &grpcClient{
		logger:      logger,
		addr:        addr,
		mustConnect: mustConnect,
		// Buffering the channel is needed to make calls appear asynchronous,
		// which is required when the caller makes multiple async calls before
		// processing callbacks (e.g. due to holding locks). 64 means that a
		// caller can make up to 64 async calls before a callback must be
		// processed (otherwise it deadlocks). It also means that we can make 64
		// gRPC calls while processing a slow callback at the channel head.
		chReqRes: make(chan *ReqRes, 64),
	}
	cli.BaseService = *service.NewBaseService(logger, "grpcClient", cli)
	return cli
}

func dialerFunc(ctx context.Context, addr string) (net.Conn, error) {
	return tmnet.Connect(addr)
}

func (cli *grpcClient) OnStart(ctx context.Context) error {
	// This processes asynchronous request/response messages and dispatches
	// them to callbacks.
	go func() {
		// Use a separate function to use defer for mutex unlocks (this handles panics)
		callCb := func(reqres *ReqRes) {
			cli.mtx.Lock()
			defer cli.mtx.Unlock()

			reqres.SetDone()
			reqres.Done()

			// Notify client listener if set
			if cli.resCb != nil {
				cli.resCb(reqres.Request, reqres.Response)
			}

			// Notify reqRes listener if set
			if cb := reqres.GetCallback(); cb != nil {
				cb(reqres.Response)
			}
		}

		for {
			select {
			case reqres := <-cli.chReqRes:
				if reqres != nil {
					callCb(reqres)
				} else {
					cli.logger.Error("Received nil reqres")
				}
			case <-ctx.Done():
				return
			}

		}
	}()

RETRY_LOOP:
	for {
		conn, err := grpc.Dial(cli.addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(dialerFunc),
		)
		if err != nil {
			if cli.mustConnect {
				return err
			}
			cli.logger.Error(fmt.Sprintf("abci.grpcClient failed to connect to %v.  Retrying...\n", cli.addr), "err", err)
			time.Sleep(time.Second * dialRetryIntervalSeconds)
			continue RETRY_LOOP
		}

		cli.logger.Info("Dialed server. Waiting for echo.", "addr", cli.addr)
		client := types.NewABCIApplicationClient(conn)
		cli.conn = conn

	ENSURE_CONNECTED:
		for {
			_, err := client.Echo(ctx, &types.RequestEcho{Message: "hello"}, grpc.WaitForReady(true))
			if err == nil {
				break ENSURE_CONNECTED
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}

			cli.logger.Error("Echo failed", "err", err)
			time.Sleep(time.Second * echoRetryIntervalSeconds)
		}

		cli.client = client
		return nil
	}
}

func (cli *grpcClient) OnStop() {
	if cli.conn != nil {
		cli.conn.Close()
	}
	close(cli.chReqRes)
}

func (cli *grpcClient) StopForError(err error) {
	if !cli.IsRunning() {
		return
	}

	cli.mtx.Lock()
	if cli.err == nil {
		cli.err = err
	}
	cli.mtx.Unlock()

	cli.logger.Error("Stopping abci.grpcClient for error", "err", err)
	if err := cli.Stop(); err != nil {
		cli.logger.Error("error stopping abci.grpcClient", "err", err)
	}
}

func (cli *grpcClient) Error() error {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	return cli.err
}

// Set listener for all responses
// NOTE: callback may get internally generated flush responses.
func (cli *grpcClient) SetResponseCallback(resCb Callback) {
	cli.mtx.Lock()
	cli.resCb = resCb
	cli.mtx.Unlock()
}

//----------------------------------------

// NOTE: call is synchronous, use ctx to break early if needed
func (cli *grpcClient) FlushAsync(ctx context.Context) (*ReqRes, error) {
	req := types.ToRequestFlush()
	res, err := cli.client.Flush(ctx, req.GetFlush(), grpc.WaitForReady(true))
	if err != nil {
		return nil, err
	}
	return cli.finishAsyncCall(ctx, req, &types.Response{Value: &types.Response_Flush{Flush: res}})
}

// NOTE: call is synchronous, use ctx to break early if needed
func (cli *grpcClient) DeliverTxAsync(ctx context.Context, params types.RequestDeliverTx) (*ReqRes, error) {
	req := types.ToRequestDeliverTx(params)
	res, err := cli.client.DeliverTx(ctx, req.GetDeliverTx(), grpc.WaitForReady(true))
	if err != nil {
		return nil, err
	}
	return cli.finishAsyncCall(ctx, req, &types.Response{Value: &types.Response_DeliverTx{DeliverTx: res}})
}

// NOTE: call is synchronous, use ctx to break early if needed
func (cli *grpcClient) CheckTxAsync(ctx context.Context, params types.RequestCheckTx) (*ReqRes, error) {
	req := types.ToRequestCheckTx(params)
	res, err := cli.client.CheckTx(ctx, req.GetCheckTx(), grpc.WaitForReady(true))
	if err != nil {
		return nil, err
	}
	return cli.finishAsyncCall(ctx, req, &types.Response{Value: &types.Response_CheckTx{CheckTx: res}})
}

// finishAsyncCall creates a ReqRes for an async call, and immediately populates it
// with the response. We don't complete it until it's been ordered via the channel.
func (cli *grpcClient) finishAsyncCall(ctx context.Context, req *types.Request, res *types.Response) (*ReqRes, error) {
	reqres := NewReqRes(req)
	reqres.Response = res
	select {
	case cli.chReqRes <- reqres: // use channel for async responses, since they must be ordered
		return reqres, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// finishSyncCall waits for an async call to complete. It is necessary to call all
// sync calls asynchronously as well, to maintain call and response ordering via
// the channel, and this method will wait until the async call completes.
func (cli *grpcClient) finishSyncCall(reqres *ReqRes) *types.Response {
	// It's possible that the callback is called twice, since the callback can
	// be called immediately on SetCallback() in addition to after it has been
	// set. This is because completing the ReqRes happens in a separate critical
	// section from the one where the callback is called: there is a race where
	// SetCallback() is called between completing the ReqRes and dispatching the
	// callback.
	//
	// We also buffer the channel with 1 response, since SetCallback() will be
	// called synchronously if the reqres is already completed, in which case
	// it will block on sending to the channel since it hasn't gotten around to
	// receiving from it yet.
	//
	// ReqRes should really handle callback dispatch internally, to guarantee
	// that it's only called once and avoid the above race conditions.
	var once sync.Once
	ch := make(chan *types.Response, 1)
	reqres.SetCallback(func(res *types.Response) {
		once.Do(func() {
			ch <- res
		})
	})
	return <-ch
}

//----------------------------------------

func (cli *grpcClient) Flush(ctx context.Context) error { return nil }

func (cli *grpcClient) Echo(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	req := types.ToRequestEcho(msg)
	return cli.client.Echo(ctx, req.GetEcho(), grpc.WaitForReady(true))
}

func (cli *grpcClient) Info(
	ctx context.Context,
	params types.RequestInfo,
) (*types.ResponseInfo, error) {
	req := types.ToRequestInfo(params)
	return cli.client.Info(ctx, req.GetInfo(), grpc.WaitForReady(true))
}

func (cli *grpcClient) DeliverTx(
	ctx context.Context,
	params types.RequestDeliverTx,
) (*types.ResponseDeliverTx, error) {

	reqres, err := cli.DeliverTxAsync(ctx, params)
	if err != nil {
		return nil, err
	}
	return cli.finishSyncCall(reqres).GetDeliverTx(), cli.Error()
}

func (cli *grpcClient) CheckTx(
	ctx context.Context,
	params types.RequestCheckTx,
) (*types.ResponseCheckTx, error) {

	reqres, err := cli.CheckTxAsync(ctx, params)
	if err != nil {
		return nil, err
	}
	return cli.finishSyncCall(reqres).GetCheckTx(), cli.Error()
}

func (cli *grpcClient) Query(
	ctx context.Context,
	params types.RequestQuery,
) (*types.ResponseQuery, error) {
	req := types.ToRequestQuery(params)
	return cli.client.Query(ctx, req.GetQuery(), grpc.WaitForReady(true))
}

func (cli *grpcClient) Commit(ctx context.Context) (*types.ResponseCommit, error) {
	req := types.ToRequestCommit()
	return cli.client.Commit(ctx, req.GetCommit(), grpc.WaitForReady(true))
}

func (cli *grpcClient) InitChain(
	ctx context.Context,
	params types.RequestInitChain,
) (*types.ResponseInitChain, error) {

	req := types.ToRequestInitChain(params)
	return cli.client.InitChain(ctx, req.GetInitChain(), grpc.WaitForReady(true))
}

func (cli *grpcClient) BeginBlock(
	ctx context.Context,
	params types.RequestBeginBlock,
) (*types.ResponseBeginBlock, error) {

	req := types.ToRequestBeginBlock(params)
	return cli.client.BeginBlock(ctx, req.GetBeginBlock(), grpc.WaitForReady(true))
}

func (cli *grpcClient) EndBlock(
	ctx context.Context,
	params types.RequestEndBlock,
) (*types.ResponseEndBlock, error) {

	req := types.ToRequestEndBlock(params)
	return cli.client.EndBlock(ctx, req.GetEndBlock(), grpc.WaitForReady(true))
}

func (cli *grpcClient) ListSnapshots(
	ctx context.Context,
	params types.RequestListSnapshots,
) (*types.ResponseListSnapshots, error) {

	req := types.ToRequestListSnapshots(params)
	return cli.client.ListSnapshots(ctx, req.GetListSnapshots(), grpc.WaitForReady(true))
}

func (cli *grpcClient) OfferSnapshot(
	ctx context.Context,
	params types.RequestOfferSnapshot,
) (*types.ResponseOfferSnapshot, error) {

	req := types.ToRequestOfferSnapshot(params)
	return cli.client.OfferSnapshot(ctx, req.GetOfferSnapshot(), grpc.WaitForReady(true))
}

func (cli *grpcClient) LoadSnapshotChunk(
	ctx context.Context,
	params types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {

	req := types.ToRequestLoadSnapshotChunk(params)
	return cli.client.LoadSnapshotChunk(ctx, req.GetLoadSnapshotChunk(), grpc.WaitForReady(true))
}

func (cli *grpcClient) ApplySnapshotChunk(
	ctx context.Context,
	params types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {

	req := types.ToRequestApplySnapshotChunk(params)
	return cli.client.ApplySnapshotChunk(ctx, req.GetApplySnapshotChunk(), grpc.WaitForReady(true))
}

func (cli *grpcClient) PrepareProposal(
	ctx context.Context,
	params types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {

	req := types.ToRequestPrepareProposal(params)
	return cli.client.PrepareProposal(ctx, req.GetPrepareProposal(), grpc.WaitForReady(true))
}

func (cli *grpcClient) ProcessProposal(
	ctx context.Context,
	params types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {

	req := types.ToRequestProcessProposal(params)
	return cli.client.ProcessProposal(ctx, req.GetProcessProposal(), grpc.WaitForReady(true))
}

func (cli *grpcClient) ExtendVote(
	ctx context.Context,
	params types.RequestExtendVote) (*types.ResponseExtendVote, error) {

	req := types.ToRequestExtendVote(params)
	return cli.client.ExtendVote(ctx, req.GetExtendVote(), grpc.WaitForReady(true))
}

func (cli *grpcClient) VerifyVoteExtension(
	ctx context.Context,
	params types.RequestVerifyVoteExtension) (*types.ResponseVerifyVoteExtension, error) {

	req := types.ToRequestVerifyVoteExtension(params)
	return cli.client.VerifyVoteExtension(ctx, req.GetVerifyVoteExtension(), grpc.WaitForReady(true))
}
