package abcicli

import (
	"context"

	types "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/service"
)

type unsyncLocalClient struct {
	service.BaseService

	types.Application
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
