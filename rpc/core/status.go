package core

import (
	"fmt"
	"strconv"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

func Status() (*ctypes.ResultStatus, error) {
	latestHeight := blockStore.Height()
	var (
		latestBlockMeta *types.BlockMeta
		latestBlockHash []byte
		latestAppHash   []byte
		latestBlockTime int64
	)
	if latestHeight != 0 {
		latestBlockMeta = blockStore.LoadBlockMeta(latestHeight)
		latestBlockHash = latestBlockMeta.Hash
		latestAppHash = latestBlockMeta.Header.AppHash
		latestBlockTime = latestBlockMeta.Header.Time.UnixNano()
	}

	return &ctypes.ResultStatus{
		NodeInfo:          p2pSwitch.NodeInfo(),
		PubKey:            privValidator.PubKey,
		LatestBlockHash:   latestBlockHash,
		LatestAppHash:     latestAppHash,
		LatestBlockHeight: latestHeight,
		LatestBlockTime:   latestBlockTime}, nil
}

func UnsafeSetConfig(typ, key, value string) (*ctypes.ResultUnsafeSetConfig, error) {
	switch typ {
	case "string":
		config.Set(key, value)
	case "int":
		val, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("non-integer value found. key:%s; value:%s; err:%v", key, value, err)
		}
		config.Set(key, val)
	case "bool":
		switch value {
		case "true":
			config.Set(key, true)
		case "false":
			config.Set(key, false)
		default:
			return nil, fmt.Errorf("bool value must be true or false. got %s", value)
		}
	default:
		return nil, fmt.Errorf("Unknown type %s", typ)
	}
	return &ctypes.ResultUnsafeSetConfig{}, nil
}
