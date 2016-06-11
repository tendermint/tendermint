package tmspcli

import (
	"fmt"
	"net"
	"sync"
	"time"

	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/types"
)

// A stripped copy of the remoteClient that makes
// synchronous calls using grpc
type grpcClient struct {
	QuitService
	mustConnect bool

	client types.TMSPApplicationClient

	mtx   sync.Mutex
	addr  string
	err   error
	resCb func(*types.Request, *types.Response) // listens to all callbacks
}

func NewGRPCClient(addr string, mustConnect bool) (*grpcClient, error) {
	cli := &grpcClient{
		addr:        addr,
		mustConnect: mustConnect,
	}
	cli.QuitService = *NewQuitService(nil, "grpcClient", cli)
	_, err := cli.Start() // Just start it, it's confusing for callers to remember to start.
	return cli, err
}

func dialerFunc(addr string, timeout time.Duration) (net.Conn, error) {
	return Connect(addr)
}

func (cli *grpcClient) OnStart() error {
	cli.QuitService.OnStart()
RETRY_LOOP:
	for {
		conn, err := grpc.Dial(cli.addr, grpc.WithInsecure(), grpc.WithDialer(dialerFunc))
		if err != nil {
			if cli.mustConnect {
				return err
			} else {
				fmt.Printf("tmsp.grpcClient failed to connect to %v.  Retrying...\n", cli.addr)
				time.Sleep(time.Second * 3)
				continue RETRY_LOOP
			}
		}
		cli.client = types.NewTMSPApplicationClient(conn)
		return nil
	}
}

func (cli *grpcClient) OnStop() {
	cli.QuitService.OnStop()
	// TODO: how to close when TMSPApplicationClient interface doesn't expose Close ?
}

// Set listener for all responses
// NOTE: callback may get internally generated flush responses.
func (cli *grpcClient) SetResponseCallback(resCb Callback) {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	cli.resCb = resCb
}

func (cli *grpcClient) StopForError(err error) {
	cli.mtx.Lock()
	fmt.Printf("Stopping tmsp.grpcClient for error: %v\n", err.Error())
	if cli.err == nil {
		cli.err = err
	}
	cli.mtx.Unlock()
	cli.Stop()
}

func (cli *grpcClient) Error() error {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()
	return cli.err
}

//----------------------------------------
// async calls are really sync.
// maybe one day, if people really want it, we use grpc streams,
// but hopefully not :D

func (cli *grpcClient) EchoAsync(msg string) *ReqRes {
	req := types.ToRequestEcho(msg)
	res, err := cli.client.Echo(context.Background(), req.GetEcho())
	if err != nil {
		cli.err = err
	}
	return cli.finishAsyncCall(req, &types.Response{&types.Response_Echo{res}})
}

func (cli *grpcClient) FlushAsync() *ReqRes {
	req := types.ToRequestFlush()
	res, err := cli.client.Flush(context.Background(), req.GetFlush())
	if err != nil {
		cli.err = err
	}
	return cli.finishAsyncCall(req, &types.Response{&types.Response_Flush{res}})
}

func (cli *grpcClient) InfoAsync() *ReqRes {
	req := types.ToRequestInfo()
	res, err := cli.client.Info(context.Background(), req.GetInfo())
	if err != nil {
		cli.err = err
	}
	return cli.finishAsyncCall(req, &types.Response{&types.Response_Info{res}})
}

func (cli *grpcClient) SetOptionAsync(key string, value string) *ReqRes {
	req := types.ToRequestSetOption(key, value)
	res, err := cli.client.SetOption(context.Background(), req.GetSetOption())
	if err != nil {
		cli.err = err
	}
	return cli.finishAsyncCall(req, &types.Response{&types.Response_SetOption{res}})
}

func (cli *grpcClient) AppendTxAsync(tx []byte) *ReqRes {
	req := types.ToRequestAppendTx(tx)
	res, err := cli.client.AppendTx(context.Background(), req.GetAppendTx())
	if err != nil {
		cli.err = err
	}
	return cli.finishAsyncCall(req, &types.Response{&types.Response_AppendTx{res}})
}

func (cli *grpcClient) CheckTxAsync(tx []byte) *ReqRes {
	req := types.ToRequestCheckTx(tx)
	res, err := cli.client.CheckTx(context.Background(), req.GetCheckTx())
	if err != nil {
		cli.err = err
	}
	return cli.finishAsyncCall(req, &types.Response{&types.Response_CheckTx{res}})
}

func (cli *grpcClient) QueryAsync(query []byte) *ReqRes {
	req := types.ToRequestQuery(query)
	res, err := cli.client.Query(context.Background(), req.GetQuery())
	if err != nil {
		cli.err = err
	}
	return cli.finishAsyncCall(req, &types.Response{&types.Response_Query{res}})
}

func (cli *grpcClient) CommitAsync() *ReqRes {
	req := types.ToRequestCommit()
	res, err := cli.client.Commit(context.Background(), req.GetCommit())
	if err != nil {
		cli.err = err
	}
	return cli.finishAsyncCall(req, &types.Response{&types.Response_Commit{res}})
}

func (cli *grpcClient) InitChainAsync(validators []*types.Validator) *ReqRes {
	req := types.ToRequestInitChain(validators)
	res, err := cli.client.InitChain(context.Background(), req.GetInitChain())
	if err != nil {
		cli.err = err
	}
	return cli.finishAsyncCall(req, &types.Response{&types.Response_InitChain{res}})
}

func (cli *grpcClient) BeginBlockAsync(height uint64) *ReqRes {
	req := types.ToRequestBeginBlock(height)
	res, err := cli.client.BeginBlock(context.Background(), req.GetBeginBlock())
	if err != nil {
		cli.err = err
	}
	return cli.finishAsyncCall(req, &types.Response{&types.Response_BeginBlock{res}})
}

func (cli *grpcClient) EndBlockAsync(height uint64) *ReqRes {
	req := types.ToRequestEndBlock(height)
	res, err := cli.client.EndBlock(context.Background(), req.GetEndBlock())
	if err != nil {
		cli.err = err
	}
	return cli.finishAsyncCall(req, &types.Response{&types.Response_EndBlock{res}})
}

func (cli *grpcClient) finishAsyncCall(req *types.Request, res *types.Response) *ReqRes {
	reqres := NewReqRes(req)
	reqres.Response = res // Set response
	reqres.Done()         // Release waiters

	// Notify reqRes listener if set
	if cb := reqres.GetCallback(); cb != nil {
		cb(res)
	}

	// Notify client listener if set
	if cli.resCb != nil {
		cli.resCb(reqres.Request, res)
	}
	return reqres
}

//----------------------------------------

func (cli *grpcClient) EchoSync(msg string) (res types.Result) {
	r := cli.EchoAsync(msg).Response.GetEcho()
	return types.NewResultOK([]byte(r.Message), LOG)
}

func (cli *grpcClient) FlushSync() error {
	return nil
}

func (cli *grpcClient) InfoSync() (res types.Result) {
	r := cli.InfoAsync().Response.GetInfo()
	return types.NewResultOK([]byte(r.Info), LOG)
}

func (cli *grpcClient) SetOptionSync(key string, value string) (res types.Result) {
	reqres := cli.SetOptionAsync(key, value)
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response.GetSetOption()
	return types.Result{Code: OK, Data: nil, Log: resp.Log}
}

func (cli *grpcClient) AppendTxSync(tx []byte) (res types.Result) {
	reqres := cli.AppendTxAsync(tx)
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response.GetAppendTx()
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *grpcClient) CheckTxSync(tx []byte) (res types.Result) {
	reqres := cli.CheckTxAsync(tx)
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response.GetCheckTx()
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *grpcClient) QuerySync(query []byte) (res types.Result) {
	reqres := cli.QueryAsync(query)
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response.GetQuery()
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *grpcClient) CommitSync() (res types.Result) {
	reqres := cli.CommitAsync()
	if cli.err != nil {
		return types.ErrInternalError.SetLog(cli.err.Error())
	}
	resp := reqres.Response.GetCommit()
	return types.Result{Code: resp.Code, Data: resp.Data, Log: resp.Log}
}

func (cli *grpcClient) InitChainSync(validators []*types.Validator) (err error) {
	cli.InitChainAsync(validators)
	if cli.err != nil {
		return cli.err
	}
	return nil
}

func (cli *grpcClient) BeginBlockSync(height uint64) (err error) {
	cli.BeginBlockAsync(height)
	if cli.err != nil {
		return cli.err
	}
	return nil
}

func (cli *grpcClient) EndBlockSync(height uint64) (validators []*types.Validator, err error) {
	reqres := cli.EndBlockAsync(height)
	if cli.err != nil {
		return nil, cli.err
	}
	return reqres.Response.GetEndBlock().Diffs, nil
}
