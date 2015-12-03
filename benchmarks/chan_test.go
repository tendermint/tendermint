package benchmarks

import (
	"testing"
)

func BenchmarkChanMakeClose(b *testing.B) {
	b.StopTimer()
	b.StartTimer()

	for j := 0; j < b.N; j++ {
		foo := make(chan struct{})
		close(foo)
		something, ok := <-foo
		if ok {
			b.Error(something, ok)
		}
	}
}
