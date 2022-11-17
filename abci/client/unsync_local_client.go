package abcicli

import (
	"context"
	"sync"

	types "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/service"
)

type unsyncLocalClient struct {
	service.BaseService

	types.Application

	// This mutex is exclusively used to protect the callback.
	mtx sync.RWMutex
	Callback
}

var _ Client = (*unsyncLocalClient)(nil)

// NewUnsyncLocalClient creates an unsynchronized local client, which will be
// directly calling the methods of the given app.
//
// Unlike NewLocalClient, it does not hold a mutex around the application, so
// it is up to the application to manage its synchronization properly.
func NewUnsyncLocalClient(app types.Application) Client {
	cli := &unsyncLocalClient{
		Application: app,
	}
	cli.BaseService = *service.NewBaseService(nil, "unsyncLocalClient", cli)
	return cli
}

// TODO: change types.Application to include Error()?
func (app *unsyncLocalClient) Error() error {
	return nil
}

func (app *unsyncLocalClient) Flush(_ context.Context) error {
	return nil
}

func (app *unsyncLocalClient) Echo(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	return &types.ResponseEcho{Message: msg}, nil
}

//-------------------------------------------------------

func (app *unsyncLocalClient) SetResponseCallback(cb Callback) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	app.Callback = cb
}

func (app *unsyncLocalClient) CheckTxAsync(ctx context.Context, req *types.RequestCheckTx) (*ReqRes, error) {
	res, err := app.Application.CheckTx(ctx, req)
	if err != nil {
		return nil, err
	}
	return app.callback(
		types.ToRequestCheckTx(req),
		types.ToResponseCheckTx(res),
	), nil
}

func (app *unsyncLocalClient) callback(req *types.Request, res *types.Response) *ReqRes {
	app.mtx.RLock()
	defer app.mtx.RUnlock()
	app.Callback(req, res)
	rr := newLocalReqRes(req, res)
	rr.callbackInvoked = true
	return rr
}
