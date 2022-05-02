package core

import (
	"context"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/proxy"
	"github.com/tendermint/tendermint/rpc/coretypes"
)

// ABCIQuery queries the application for some information.
// More: https://docs.tendermint.com/master/rpc/#/ABCI/abci_query
func (env *Environment) ABCIQuery(ctx context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
	resQuery, err := env.ProxyApp.Query(ctx, &abci.RequestQuery{
		Path:   req.Path,
		Data:   req.Data,
		Height: int64(req.Height),
		Prove:  req.Prove,
	})
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultABCIQuery{Response: *resQuery}, nil
}

// ABCIInfo gets some info about the application.
// More: https://docs.tendermint.com/master/rpc/#/ABCI/abci_info
func (env *Environment) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	resInfo, err := env.ProxyApp.Info(ctx, &proxy.RequestInfo)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultABCIInfo{Response: *resInfo}, nil
}
