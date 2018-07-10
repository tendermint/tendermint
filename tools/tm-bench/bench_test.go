package main

import (
	"testing"
	"time"
)

func BenchmarkTimingPerTx(b *testing.B) {
	startTime := time.Now()
	endTime := startTime.Add(time.Second)
	for i := 0; i < b.N; i++ {
		if i%20 == 0 {
			if time.Now().After(endTime) {
				continue
			}
		}
	}
}
