package core

import (
	abci "github.com/tendermint/abci/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

//-----------------------------------------------------------------------------

func ABCIQuery(path string, data []byte, prove bool) (*ctypes.ResultABCIQuery, error) {
	resQuery, err := proxyAppQuery.QuerySync(abci.RequestQuery{
		Path:  path,
		Data:  data,
		Prove: prove,
	})
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultABCIQuery{resQuery}, nil
}

func ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	resInfo, err := proxyAppQuery.InfoSync()
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultABCIInfo{resInfo}, nil
}
