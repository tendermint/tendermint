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

	mtx  sync.Mutex
	addr string
	err  error
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

			// Notify reqRes listener if set
			reqres.InvokeCallback()
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
	cli.Stop()
}

func (cli *grpcClient) Error() error {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	return cli.err
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

//----------------------------------------

func (cli *grpcClient) Flush(ctx context.Context) error { return nil }

func (cli *grpcClient) Echo(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	return cli.client.Echo(ctx, types.ToRequestEcho(msg).GetEcho(), grpc.WaitForReady(true))
}

func (cli *grpcClient) Info(ctx context.Context, params types.RequestInfo) (*types.ResponseInfo, error) {
	return cli.client.Info(ctx, types.ToRequestInfo(params).GetInfo(), grpc.WaitForReady(true))
}

func (cli *grpcClient) CheckTx(ctx context.Context, params types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	return cli.client.CheckTx(ctx, types.ToRequestCheckTx(params).GetCheckTx(), grpc.WaitForReady(true))
}

func (cli *grpcClient) Query(ctx context.Context, params types.RequestQuery) (*types.ResponseQuery, error) {
	return cli.client.Query(ctx, types.ToRequestQuery(params).GetQuery(), grpc.WaitForReady(true))
}

func (cli *grpcClient) Commit(ctx context.Context) (*types.ResponseCommit, error) {
	return cli.client.Commit(ctx, types.ToRequestCommit().GetCommit(), grpc.WaitForReady(true))
}

func (cli *grpcClient) InitChain(ctx context.Context, params types.RequestInitChain) (*types.ResponseInitChain, error) {
	return cli.client.InitChain(ctx, types.ToRequestInitChain(params).GetInitChain(), grpc.WaitForReady(true))
}

func (cli *grpcClient) ListSnapshots(ctx context.Context, params types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
	return cli.client.ListSnapshots(ctx, types.ToRequestListSnapshots(params).GetListSnapshots(), grpc.WaitForReady(true))
}

func (cli *grpcClient) OfferSnapshot(ctx context.Context, params types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	return cli.client.OfferSnapshot(ctx, types.ToRequestOfferSnapshot(params).GetOfferSnapshot(), grpc.WaitForReady(true))
}

func (cli *grpcClient) LoadSnapshotChunk(ctx context.Context, params types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {
	return cli.client.LoadSnapshotChunk(ctx, types.ToRequestLoadSnapshotChunk(params).GetLoadSnapshotChunk(), grpc.WaitForReady(true))
}

func (cli *grpcClient) ApplySnapshotChunk(ctx context.Context, params types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
	return cli.client.ApplySnapshotChunk(ctx, types.ToRequestApplySnapshotChunk(params).GetApplySnapshotChunk(), grpc.WaitForReady(true))
}

func (cli *grpcClient) PrepareProposal(ctx context.Context, params types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	return cli.client.PrepareProposal(ctx, types.ToRequestPrepareProposal(params).GetPrepareProposal(), grpc.WaitForReady(true))
}

func (cli *grpcClient) ProcessProposal(ctx context.Context, params types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
	return cli.client.ProcessProposal(ctx, types.ToRequestProcessProposal(params).GetProcessProposal(), grpc.WaitForReady(true))
}

func (cli *grpcClient) ExtendVote(ctx context.Context, params types.RequestExtendVote) (*types.ResponseExtendVote, error) {
	return cli.client.ExtendVote(ctx, types.ToRequestExtendVote(params).GetExtendVote(), grpc.WaitForReady(true))
}

func (cli *grpcClient) VerifyVoteExtension(ctx context.Context, params types.RequestVerifyVoteExtension) (*types.ResponseVerifyVoteExtension, error) {
	return cli.client.VerifyVoteExtension(ctx, types.ToRequestVerifyVoteExtension(params).GetVerifyVoteExtension(), grpc.WaitForReady(true))
}

func (cli *grpcClient) FinalizeBlock(ctx context.Context, params types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	return cli.client.FinalizeBlock(ctx, types.ToRequestFinalizeBlock(params).GetFinalizeBlock(), grpc.WaitForReady(true))
}
