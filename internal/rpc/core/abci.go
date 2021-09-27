package core

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/proxy"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// ABCIQuery queries the application for some information.
// More: https://docs.tendermint.com/master/rpc/#/ABCI/abci_query
func (env *Environment) ABCIQuery(
	ctx *rpctypes.Context,
	path string,
	data bytes.HexBytes,
	height int64,
	prove bool,
) (*coretypes.ResultABCIQuery, error) {
	resQuery, err := env.ProxyAppQuery.QuerySync(ctx.Context(), abci.RequestQuery{
		Path:   path,
		Data:   data,
		Height: height,
		Prove:  prove,
	})
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultABCIQuery{Response: *resQuery}, nil
}

// ABCIInfo gets some info about the application.
// More: https://docs.tendermint.com/master/rpc/#/ABCI/abci_info
func (env *Environment) ABCIInfo(ctx *rpctypes.Context) (*coretypes.ResultABCIInfo, error) {
	resInfo, err := env.ProxyAppQuery.InfoSync(ctx.Context(), proxy.RequestInfo)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultABCIInfo{Response: *resInfo}, nil
}
