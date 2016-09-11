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
	res := proxyAppQuery.InfoSync()
	return &ctypes.ResultTMSPInfo{res}, nil
}
