// Copyright 2016 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
