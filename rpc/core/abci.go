package core

import (
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

//-----------------------------------------------------------------------------

func ABCIQuery(query []byte) (*ctypes.ResultABCIQuery, error) {
	res := proxyAppQuery.QuerySync(query)
	return &ctypes.ResultABCIQuery{res}, nil
}

func ABCIProof(key []byte, height uint64) (*ctypes.ResultABCIProof, error) {
	res := proxyAppQuery.ProofSync(key, height)
	return &ctypes.ResultABCIProof{res}, nil
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
