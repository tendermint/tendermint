// Copyright 2015 Tendermint. All Rights Reserved.
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

package benchmarks

import (
	"sync/atomic"
	"testing"
	"unsafe"
)

func BenchmarkAtomicUintPtr(b *testing.B) {
	b.StopTimer()
	pointers := make([]uintptr, 1000)
	b.Log(unsafe.Sizeof(pointers[0]))
	b.StartTimer()

	for j := 0; j < b.N; j++ {
		atomic.StoreUintptr(&pointers[j%1000], uintptr(j))
	}
}

func BenchmarkAtomicPointer(b *testing.B) {
	b.StopTimer()
	pointers := make([]unsafe.Pointer, 1000)
	b.Log(unsafe.Sizeof(pointers[0]))
	b.StartTimer()

	for j := 0; j < b.N; j++ {
		atomic.StorePointer(&pointers[j%1000], unsafe.Pointer(uintptr(j)))
	}
}
