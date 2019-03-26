package abcicli

import (
	"fmt"
	"net"
	"sync"
	"time"

	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"

	"github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
)

var _ Client = (*grpcClient)(nil)

// A stripped copy of the remoteClient that makes
// synchronous calls using grpc
type grpcClient struct {
	cmn.BaseService
	mustConnect bool

	client types.ABCIApplicationClient
	conn   *grpc.ClientConn

	mtx   sync.Mutex
	addr  string
	err   error
	resCb func(*types.Request, *types.Response) // listens to all callbacks
}

func NewGRPCClient(addr string, mustConnect bool) *grpcClient {
	cli := &grpcClient{
		addr:        addr,
		mustConnect: mustConnect,
	}
	cli.BaseService = *cmn.NewBaseService(nil, "grpcClient", cli)
	return cli
}

func dialerFunc(addr string, timeout time.Duration) (net.Conn, error) {
	return cmn.Connect(addr)
}

func (cli *grpcClient) OnStart() error {
	if err := cli.BaseService.OnStart(); err != nil {
		return err
	}
RETRY_LOOP:
	for {
		conn, err := grpc.Dial(cli.addr, grpc.WithInsecure(), grpc.WithDialer(dialerFunc))
		if err != nil {
			if cli.mustConnect {
				return err
			}
			cli.Logger.Error(fmt.Sprintf("abci.grpcClient failed to connect to %v.  Retrying...\n", cli.addr), "err", err)
			time.Sleep(time.Second * dialRetryIntervalSeconds)
			continue RETRY_LOOP
		}

		cli.Logger.Info("Dialed server. Waiting for echo.", "addr", cli.addr)
		client := types.NewABCIApplicationClient(conn)
		cli.conn = conn

	ENSURE_CONNECTED:
		for {
			_, err := client.Echo(context.Background(), &types.RequestEcho{Message: "hello"}, grpc.FailFast(true))
			if err == nil {
				break ENSURE_CONNECTED
			}
			cli.Logger.Error("Echo failed", "err", err)
			time.Sleep(time.Second * echoRetryIntervalSeconds)
		}

		cli.client = client
		return nil
	}
}

func (cli *grpcClient) OnStop() {
	cli.BaseService.OnStop()

	if cli.conn != nil {
		cli.conn.Close()
	}
}

func (cli *grpcClient) StopForError(err error) {
	cli.mtx.Lock()
	if !cli.IsRunning() {
		return
	}

	if cli.err == nil {
		cli.err = err
	}
	cli.mtx.Unlock()

	cli.Logger.Error(fmt.Sprintf("Stopping abci.grpcClient for error: %v", err.Error()))
	cli.Stop()
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
// GRPC calls are synchronous, but some callbacks expect to be called asynchronously
// (eg. the mempool expects to be able to lock to remove bad txs from cache).
// To accommodate, we finish each call in its own go-routine,
// which is expensive, but easy - if you want something better, use the socket protocol!
// maybe one day, if people really want it, we use grpc streams,
// but hopefully not :D

func (cli *grpcClient) EchoAsync(msg string) *ReqRes {
	req := types.ToRequestEcho(msg)
	res, err := cli.client.Echo(context.Background(), req.GetEcho(), grpc.FailFast(true))
	if err != nil {
		cli.StopForError(err)
	}
	return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_Echo{Echo: res}})
}

func (cli *grpcClient) FlushAsync() *ReqRes {
	req := types.ToRequestFlush()
	res, err := cli.client.Flush(context.Background(), req.GetFlush(), grpc.FailFast(true))
	if err != nil {
		cli.StopForError(err)
	}
	return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_Flush{Flush: res}})
}

func (cli *grpcClient) InfoAsync(params types.RequestInfo) *ReqRes {
	req := types.ToRequestInfo(params)
	res, err := cli.client.Info(context.Background(), req.GetInfo(), grpc.FailFast(true))
	if err != nil {
		cli.StopForError(err)
	}
	return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_Info{Info: res}})
}

func (cli *grpcClient) SetOptionAsync(params types.RequestSetOption) *ReqRes {
	req := types.ToRequestSetOption(params)
	res, err := cli.client.SetOption(context.Background(), req.GetSetOption(), grpc.FailFast(true))
	if err != nil {
		cli.StopForError(err)
	}
	return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_SetOption{SetOption: res}})
}

func (cli *grpcClient) DeliverTxAsync(tx []byte) *ReqRes {
	req := types.ToRequestDeliverTx(tx)
	res, err := cli.client.DeliverTx(context.Background(), req.GetDeliverTx(), grpc.FailFast(true))
	if err != nil {
		cli.StopForError(err)
	}
	return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_DeliverTx{DeliverTx: res}})
}

func (cli *grpcClient) CheckTxAsync(tx []byte) *ReqRes {
	req := types.ToRequestCheckTx(tx)
	res, err := cli.client.CheckTx(context.Background(), req.GetCheckTx(), grpc.FailFast(true))
	if err != nil {
		cli.StopForError(err)
	}
	return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_CheckTx{CheckTx: res}})
}

func (cli *grpcClient) QueryAsync(params types.RequestQuery) *ReqRes {
	req := types.ToRequestQuery(params)
	res, err := cli.client.Query(context.Background(), req.GetQuery(), grpc.FailFast(true))
	if err != nil {
		cli.StopForError(err)
	}
	return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_Query{Query: res}})
}

func (cli *grpcClient) CommitAsync() *ReqRes {
	req := types.ToRequestCommit()
	res, err := cli.client.Commit(context.Background(), req.GetCommit(), grpc.FailFast(true))
	if err != nil {
		cli.StopForError(err)
	}
	return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_Commit{Commit: res}})
}

func (cli *grpcClient) InitChainAsync(params types.RequestInitChain) *ReqRes {
	req := types.ToRequestInitChain(params)
	res, err := cli.client.InitChain(context.Background(), req.GetInitChain(), grpc.FailFast(true))
	if err != nil {
		cli.StopForError(err)
	}
	return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_InitChain{InitChain: res}})
}

func (cli *grpcClient) BeginBlockAsync(params types.RequestBeginBlock) *ReqRes {
	req := types.ToRequestBeginBlock(params)
	res, err := cli.client.BeginBlock(context.Background(), req.GetBeginBlock(), grpc.FailFast(true))
	if err != nil {
		cli.StopForError(err)
	}
	return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_BeginBlock{BeginBlock: res}})
}

func (cli *grpcClient) EndBlockAsync(params types.RequestEndBlock) *ReqRes {
	req := types.ToRequestEndBlock(params)
	res, err := cli.client.EndBlock(context.Background(), req.GetEndBlock(), grpc.FailFast(true))
	if err != nil {
		cli.StopForError(err)
	}
	return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_EndBlock{EndBlock: res}})
}

func (cli *grpcClient) finishAsyncCall(req *types.Request, res *types.Response) *ReqRes {
	reqres := NewReqRes(req)
	reqres.Response = res // Set response
	reqres.Done()         // Release waiters
	reqres.SetDone()      // so reqRes.SetCallback will run the callback

	// go routine for callbacks
	go func() {
		// Notify reqRes listener if set
		if cb := reqres.GetCallback(); cb != nil {
			cb(res)
		}

		// Notify client listener if set
		if cli.resCb != nil {
			cli.resCb(reqres.Request, res)
		}
	}()
	return reqres
}

//----------------------------------------

func (cli *grpcClient) FlushSync() error {
	return nil
}

func (cli *grpcClient) EchoSync(msg string) (*types.ResponseEcho, error) {
	reqres := cli.EchoAsync(msg)
	// StopForError should already have been called if error is set
	return reqres.Response.GetEcho(), cli.Error()
}

func (cli *grpcClient) InfoSync(req types.RequestInfo) (*types.ResponseInfo, error) {
	reqres := cli.InfoAsync(req)
	return reqres.Response.GetInfo(), cli.Error()
}

func (cli *grpcClient) SetOptionSync(req types.RequestSetOption) (*types.ResponseSetOption, error) {
	reqres := cli.SetOptionAsync(req)
	return reqres.Response.GetSetOption(), cli.Error()
}

func (cli *grpcClient) DeliverTxSync(tx []byte) (*types.ResponseDeliverTx, error) {
	reqres := cli.DeliverTxAsync(tx)
	return reqres.Response.GetDeliverTx(), cli.Error()
}

func (cli *grpcClient) CheckTxSync(tx []byte) (*types.ResponseCheckTx, error) {
	reqres := cli.CheckTxAsync(tx)
	return reqres.Response.GetCheckTx(), cli.Error()
}

func (cli *grpcClient) QuerySync(req types.RequestQuery) (*types.ResponseQuery, error) {
	reqres := cli.QueryAsync(req)
	return reqres.Response.GetQuery(), cli.Error()
}

func (cli *grpcClient) CommitSync() (*types.ResponseCommit, error) {
	reqres := cli.CommitAsync()
	return reqres.Response.GetCommit(), cli.Error()
}

func (cli *grpcClient) InitChainSync(params types.RequestInitChain) (*types.ResponseInitChain, error) {
	reqres := cli.InitChainAsync(params)
	return reqres.Response.GetInitChain(), cli.Error()
}

func (cli *grpcClient) BeginBlockSync(params types.RequestBeginBlock) (*types.ResponseBeginBlock, error) {
	reqres := cli.BeginBlockAsync(params)
	return reqres.Response.GetBeginBlock(), cli.Error()
}

func (cli *grpcClient) EndBlockSync(params types.RequestEndBlock) (*types.ResponseEndBlock, error) {
	reqres := cli.EndBlockAsync(params)
	return reqres.Response.GetEndBlock(), cli.Error()
}
