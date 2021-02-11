package memdb

import (
	"testing"

	"github.com/tendermint/tm-db/internal/dbtest"
)

func BenchmarkMemDBRangeScans1M(b *testing.B) {
	db := NewDB()
	defer db.Close()

	dbtest.BenchmarkRangeScans(b, db, int64(1e6))
}

func BenchmarkMemDBRangeScans10M(b *testing.B) {
	db := NewDB()
	defer db.Close()

	dbtest.BenchmarkRangeScans(b, db, int64(10e6))
}

func BenchmarkMemDBRandomReadsWrites(b *testing.B) {
	db := NewDB()
	defer db.Close()

	dbtest.BenchmarkRandomReadsWrites(b, db)
}
