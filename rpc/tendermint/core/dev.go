package core

import (
	"fmt"
	"os"
	"runtime/pprof"
	"strconv"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func UnsafeFlushMempool() (*ctypes.ResultUnsafeFlushMempool, error) {
	mempool.Flush()
	return &ctypes.ResultUnsafeFlushMempool{}, nil
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

var profFile *os.File

func UnsafeStartCPUProfiler(filename string) (*ctypes.ResultUnsafeProfile, error) {
	var err error
	profFile, err = os.Create(filename)
	if err != nil {
		return nil, err
	}
	err = pprof.StartCPUProfile(profFile)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsafeProfile{}, nil
}

func UnsafeStopCPUProfiler() (*ctypes.ResultUnsafeProfile, error) {
	pprof.StopCPUProfile()
	profFile.Close()
	return &ctypes.ResultUnsafeProfile{}, nil
}

func UnsafeWriteHeapProfile(filename string) (*ctypes.ResultUnsafeProfile, error) {
	memProfFile, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	pprof.WriteHeapProfile(memProfFile)
	memProfFile.Close()

	return &ctypes.ResultUnsafeProfile{}, nil
}
