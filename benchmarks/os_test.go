package benchmarks

import (
	"os"
	"testing"

	cmn "github.com/tendermint/tendermint/libs/common"
)

func BenchmarkFileWrite(b *testing.B) {
	b.StopTimer()
	file, err := os.OpenFile("benchmark_file_write.out",
		os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		b.Error(err)
	}
	testString := cmn.RandStr(200) + "\n"
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		_, err := file.Write([]byte(testString))
		if err != nil {
			b.Error(err)
		}
	}

	if err := file.Close(); err != nil {
		b.Error(err)
	}
	if err := os.Remove("benchmark_file_write.out"); err != nil {
		b.Error(err)
	}
}
