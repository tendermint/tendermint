package abciclient

import (
	"context"

	types "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

// NOTE: use defer to unlock mutex because Application might panic (e.g., in
// case of malicious tx or query). It only makes sense for publicly exposed
// methods like CheckTx (/broadcast_tx_* RPC endpoint) or Query (/abci_query
// RPC endpoint), but defers are used everywhere for the sake of consistency.
type localClient struct {
	service.BaseService
	types.Application
}

var _ Client = (*localClient)(nil)

// NewLocalClient creates a local client, which will be directly calling the
// methods of the given app.
//
// The client methods ignore their context argument.
func NewLocalClient(logger log.Logger, app types.Application) Client {
	cli := &localClient{
		Application: app,
	}
	cli.BaseService = *service.NewBaseService(logger, "localClient", cli)
	return cli
}

func (*localClient) OnStart(context.Context) error { return nil }
func (*localClient) OnStop()                       {}
func (*localClient) Error() error                  { return nil }
func (*localClient) Flush(context.Context) error   { return nil }
func (*localClient) Echo(_ context.Context, msg string) (*types.ResponseEcho, error) {
	return &types.ResponseEcho{Message: msg}, nil
}
