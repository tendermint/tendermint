package core

import (
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

//-----------------------------------------------------------------------------

func TMSPQuery(query []byte) (*ctypes.ResultTMSPQuery, error) {
	res := proxyAppQuery.QuerySync(query)
	return &ctypes.ResultTMSPQuery{res}, nil
}

func TMSPProof(key []byte, blockHeight int64) (*ctypes.ResultTMSPProof, error) {
	res := proxyAppQuery.ProofSync(key, blockHeight)
	return &ctypes.ResultTMSPProof{res}, nil
}

func TMSPInfo() (*ctypes.ResultTMSPInfo, error) {
	res, tmspInfo, lastBlockInfo, configInfo := proxyAppQuery.InfoSync()
	return &ctypes.ResultTMSPInfo{res, tmspInfo, lastBlockInfo, configInfo}, nil
}
