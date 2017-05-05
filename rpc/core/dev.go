package core

import (
	"os"
	"runtime/pprof"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func UnsafeFlushMempool() (*ctypes.ResultUnsafeFlushMempool, error) {
	mempool.Flush()
	return &ctypes.ResultUnsafeFlushMempool{}, nil
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
