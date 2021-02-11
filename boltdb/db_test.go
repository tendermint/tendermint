package boltdb

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tm-db/internal/dbtest"
)

func TestBoltDBNewBoltDB(t *testing.T) {
	name := fmt.Sprintf("test_%x", dbtest.RandStr(12))
	dir := os.TempDir()
	defer dbtest.CleanupDBDir(dir, name)

	db, err := NewDB(name, dir)
	require.NoError(t, err)
	db.Close()
}

func BenchmarkBoltDBRandomReadsWrites(b *testing.B) {
	name := fmt.Sprintf("test_%x", dbtest.RandStr(12))
	db, err := NewDB(name, "")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		db.Close()
		dbtest.CleanupDBDir("", name)
	}()

	dbtest.BenchmarkRandomReadsWrites(b, db)
}
