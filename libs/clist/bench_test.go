package clist

import "testing"

func BenchmarkDetaching(b *testing.B) {
	lst := New()
	for i := 0; i < b.N+1; i++ {
		lst.PushBack(i)
	}
	start := lst.Front()
	nxt := start.Next()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start.removed = true
		start.DetachNext()
		start.DetachPrev()
		tmp := nxt
		nxt = nxt.Next()
		start = tmp
	}
}
