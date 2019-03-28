package core

import (
	"os"
	"runtime/pprof"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
)

// UnsafeFlushMempool removes all transactions from the mempool.
func UnsafeFlushMempool(ctx *rpctypes.Context) (*ctypes.ResultUnsafeFlushMempool, error) {
	mempool.Flush()
	return &ctypes.ResultUnsafeFlushMempool{}, nil
}

var profFile *os.File

// UnsafeStartCPUProfiler starts a pprof profiler using the given filename.
func UnsafeStartCPUProfiler(ctx *rpctypes.Context, filename string) (*ctypes.ResultUnsafeProfile, error) {
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

// UnsafeStopCPUProfiler stops the running pprof profiler.
func UnsafeStopCPUProfiler(ctx *rpctypes.Context) (*ctypes.ResultUnsafeProfile, error) {
	pprof.StopCPUProfile()
	if err := profFile.Close(); err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsafeProfile{}, nil
}

// UnsafeWriteHeapProfile dumps a heap profile to the given filename.
func UnsafeWriteHeapProfile(ctx *rpctypes.Context, filename string) (*ctypes.ResultUnsafeProfile, error) {
	memProfFile, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	if err := pprof.WriteHeapProfile(memProfFile); err != nil {
		return nil, err
	}
	if err := memProfFile.Close(); err != nil {
		return nil, err
	}

	return &ctypes.ResultUnsafeProfile{}, nil
}
