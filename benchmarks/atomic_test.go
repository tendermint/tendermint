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
