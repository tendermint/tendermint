package core

import (
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

//-----------------------------------------------------------------------------

func ABCIQuery(query []byte) (*ctypes.ResultABCIQuery, error) {
	res := proxyAppQuery.QuerySync(query)
	return &ctypes.ResultABCIQuery{res}, nil
}

func ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	res, err := proxyAppQuery.InfoSync()
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultABCIInfo{
		Data:             res.Data,
		Version:          res.Version,
		LastBlockHeight:  res.LastBlockHeight,
		LastBlockAppHash: res.LastBlockAppHash,
	}, nil
}
