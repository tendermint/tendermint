package core

import (
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func Tx(hash []byte) (*ctypes.ResultTx, error) {
	r, err := txIndexer.Tx(hash)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultTx{*r}, nil
}
