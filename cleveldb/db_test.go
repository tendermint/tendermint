package cleveldb

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/tendermint/tm-db/internal/dbtest"
)

func BenchmarkRandomReadsWrites2(b *testing.B) {
	b.StopTimer()

	numItems := int64(1000000)
	internal := map[int64]int64{}
	for i := 0; i < int(numItems); i++ {
		internal[int64(i)] = int64(0)
	}
	db, err := NewDB(fmt.Sprintf("test_%x", dbtest.RandStr(12)), "")
	if err != nil {
		b.Fatal(err.Error())
		return
	}

	fmt.Println("ok, starting")
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		// Write something
		{
			idx := (int64(rand.Int()) % numItems)
			internal[idx]++
			val := internal[idx]
			idxBytes := dbtest.Int642Bytes(idx)
			valBytes := dbtest.Int642Bytes(val)
			// fmt.Printf("Set %X -> %X\n", idxBytes, valBytes)
			if err = db.Set(
				idxBytes,
				valBytes,
			); err != nil {
				b.Error(err)
			}
		}
		// Read something
		{
			idx := (int64(rand.Int()) % numItems)
			val := internal[idx]
			idxBytes := dbtest.Int642Bytes(idx)
			valBytes, err := db.Get(idxBytes)
			if err != nil {
				b.Error(err)
			}
			// fmt.Printf("Get %X -> %X\n", idxBytes, valBytes)
			if val == 0 {
				if !bytes.Equal(valBytes, nil) {
					b.Errorf("Expected %v for %v, got %X",
						nil, idx, valBytes)
					break
				}
			} else {
				if len(valBytes) != 8 {
					b.Errorf("Expected length 8 for %v, got %X",
						idx, valBytes)
					break
				}
				valGot := dbtest.Bytes2Int64(valBytes)
				if val != valGot {
					b.Errorf("Expected %v for %v, got %v",
						val, idx, valGot)
					break
				}
			}
		}
	}

	db.Close()
}
