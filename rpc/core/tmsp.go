package core

import (
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

//-----------------------------------------------------------------------------

func TMSPQuery(query []byte) (*ctypes.ResultTMSPQuery, error) {
	res := proxyAppQuery.QuerySync(query)
	return &ctypes.ResultTMSPQuery{res}, nil
}

func TMSPInfo() (*ctypes.ResultTMSPInfo, error) {
	res, err := proxyAppQuery.InfoSync()
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultTMSPInfo{
		Data:             res.Data,
		Version:          res.Version,
		LastBlockHeight:  res.LastBlockHeight,
		LastBlockAppHash: res.LastBlockAppHash,
	}, nil
}
