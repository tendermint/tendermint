package mmm

import (
	"bufio"
	"os"
	"syscall"
	"testing"
	"unsafe"

	. "github.com/tendermint/go-common"
)

func BenchmarkMmapCopyMmap(b *testing.B) {
	b.StopTimer()
	size := 1 << (10 * 3)
	bytes1, err := makeMmap(b, size)
	if err != nil {
		b.Error(err)
		return
	}
	bytes2, err := makeMmap(b, size)
	if err != nil {
		b.Error(err)
		return
	}
	b.StartTimer()

	for j := 0; j < b.N; j++ {
		for i := 0; i < size; i++ {
			bytes2[i] = bytes1[i]
		}
	}
}

func BenchmarkMmapCopyFile128Buf(b *testing.B) {
	b.StopTimer()
	size := 1 << (10 * 3)
	bytes1, err := makeMmap(b, size)
	if err != nil {
		b.Error(err)
		return
	}
	file, err := os.OpenFile(Fmt("temp-%v", RandStr(4)), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		b.Error(err)
		return
	}
	buf := bufio.NewWriter(file)
	b.StartTimer()

	for j := 0; j < b.N; j++ {
		for i := 0; i < size; i += 128 {
			buf.Write(bytes1[i : i+128])
		}
		buf.Flush()
	}
}

func BenchmarkMmapCopyFile1024Buf(b *testing.B) {
	b.StopTimer()
	size := 1 << (10 * 3)
	bytes1, err := makeMmap(b, size)
	if err != nil {
		b.Error(err)
		return
	}
	file, err := os.OpenFile(Fmt("temp-%v", RandStr(4)), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		b.Error(err)
		return
	}
	buf := bufio.NewWriter(file)
	b.StartTimer()

	for j := 0; j < b.N; j++ {
		for i := 0; i < size; i += 1024 {
			buf.Write(bytes1[i : i+1024])
		}
		buf.Flush()
	}
}

func BenchmarkMmapCopyFile4096(b *testing.B) {
	b.StopTimer()
	size := 1 << (10 * 3)
	bytes1, err := makeMmap(b, size)
	if err != nil {
		b.Error(err)
		return
	}
	file, err := os.OpenFile(Fmt("temp-%v", RandStr(4)), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		b.Error(err)
		return
	}
	b.StartTimer()

	for j := 0; j < b.N; j++ {
		for i := 0; i < size; i += 4096 {
			file.Write(bytes1[i : i+4096])
		}
	}
}

// Commented out, this is slower, and unsafe.
func _BenchmarkMmapCopyMmap64(b *testing.B) {
	b.StopTimer()
	size := 1 << (10 * 3)
	bytes1, err := makeMmap(b, size)
	if err != nil {
		b.Error(err)
		return
	}
	bytes2, err := makeMmap(b, size)
	if err != nil {
		b.Error(err)
		return
	}
	bytes1_64 := *(*[]uint64)(unsafe.Pointer(&bytes1))
	bytes2_64 := *(*[]uint64)(unsafe.Pointer(&bytes2))
	b.StartTimer()

	for j := 0; j < b.N; j++ {
		for i := 0; i < size/8; i++ {
			bytes2_64[i] = bytes1_64[i]
		}
	}
}

func makeMmap(b *testing.B, size int) ([]byte, error) {
	b.Log("Setting up v mmap region, size:", size)
	bytes1, err := syscall.Mmap(
		0, 0, size,
		syscall.PROT_READ|syscall.PROT_WRITE,
		mmapFlags,
	)
	b.Log("Initializing region")
	for i := 0; i < size; i++ {
		bytes1[i] = byte((i + 13) % 256)
	}
	b.Log("Done")
	return bytes1, err
}
