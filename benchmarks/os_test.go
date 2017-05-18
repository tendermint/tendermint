package benchmarks

import (
	"os"
	"testing"

	. "github.com/tendermint/tmlibs/common"
)

func BenchmarkFileWrite(b *testing.B) {
	b.StopTimer()
	file, err := os.OpenFile("benchmark_file_write.out",
		os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		b.Error(err)
	}
	testString := RandStr(200) + "\n"
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		file.Write([]byte(testString))
	}

	file.Close()
	err = os.Remove("benchmark_file_write.out")
	if err != nil {
		b.Error(err)
	}
}
